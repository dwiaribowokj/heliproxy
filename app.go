package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type App struct {
	store   *ConfigStore
	manager *KeyManager
}

type APIResponse struct {
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
	Data  any    `json:"data,omitempty"`
}

type ProxyAttemptResult struct {
	KeyID        string `json:"key_id"`
	KeyName      string `json:"key_name"`
	APIKeyMasked string `json:"api_key_masked"`
	Status       int    `json:"status,omitempty"`
	Error        string `json:"error,omitempty"`
	LatencyMS    int64  `json:"latency_ms,omitempty"`
}

func NewApp(store *ConfigStore, cfg *Config) *App {
	return &App{store: store, manager: NewKeyManager(cfg)}
}

func (a *App) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", a.handleHealth)
	mux.HandleFunc("GET /dashboard", a.handleDashboard)
	mux.HandleFunc("GET /api/admin/config", a.handleGetConfig)
	mux.HandleFunc("PUT /api/admin/config", a.handlePutConfig)
	mux.HandleFunc("GET /api/admin/status", a.handleStatus)
	mux.HandleFunc("GET /api/admin/usage", a.handleRefreshUsage)
	mux.HandleFunc("POST /api/admin/usage", a.handleRefreshUsage)
	mux.HandleFunc("POST /rpc", a.handleRPC)
	mux.HandleFunc("/v1/", a.handleREST)
	mux.HandleFunc("/", a.handleRoot)
	return loggingMiddleware(mux)
}

func (a *App) handleREST(w http.ResponseWriter, r *http.Request) {
	if !a.authorized(r, true) {
		writeJSON(w, http.StatusUnauthorized, APIResponse{OK: false, Error: "invalid_api_key"})
		return
	}
	cfg := a.manager.Config()
	maxBody := cfg.Routing.MaxBodyBytes
	if maxBody <= 0 {
		maxBody = defaultMaxBodyBytes
	}
	var body []byte
	if r.Body != nil {
		var err error
		body, err = io.ReadAll(http.MaxBytesReader(w, r.Body, int64(maxBody)))
		if err != nil {
			writeJSON(w, http.StatusRequestEntityTooLarge, APIResponse{OK: false, Error: "request_body_too_large"})
			return
		}
	}

	attempts := a.manager.NextAttempts(time.Now().UTC())
	if len(attempts) == 0 {
		writeJSON(w, http.StatusServiceUnavailable, APIResponse{OK: false, Error: "no_available_helius_keys"})
		return
	}

	results := make([]ProxyAttemptResult, 0, len(attempts))
	for _, attempt := range attempts {
		started := time.Now()
		resp, respBody, err := a.forwardREST(r.Context(), cfg, r, body, attempt.Key)
		latency := time.Since(started)
		result := ProxyAttemptResult{
			KeyID:        attempt.Key.ID,
			KeyName:      attempt.Key.Name,
			APIKeyMasked: maskSecret(attempt.Key.APIKey),
			LatencyMS:    latency.Milliseconds(),
		}
		if err != nil {
			message := compactError(err.Error())
			result.Error = message
			results = append(results, result)
			a.manager.MarkAttempt(attempt.Key.ID, false, 0, message, latency, true)
			continue
		}

		result.Status = resp.StatusCode
		if shouldFailoverStatus(resp.StatusCode) {
			message := upstreamErrorMessage(respBody, resp.Status)
			result.Error = message
			results = append(results, result)
			a.manager.MarkAttempt(attempt.Key.ID, false, resp.StatusCode, message, latency, true)
			continue
		}

		results = append(results, result)
		a.manager.MarkAttempt(attempt.Key.ID, true, resp.StatusCode, "", latency, false)
		copyResponseHeaders(w.Header(), resp.Header)
		w.Header().Set("X-Heliproxy-Key-ID", attempt.Key.ID)
		w.WriteHeader(resp.StatusCode)
		_, _ = w.Write(respBody)
		return
	}

	writeJSON(w, http.StatusServiceUnavailable, map[string]any{
		"error": map[string]any{
			"message":  "all helius keys failed",
			"attempts": results,
		},
	})
}

func (a *App) handleRoot(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		writeJSON(w, http.StatusNotFound, APIResponse{OK: false, Error: "not_found"})
		return
	}
	if r.Method == http.MethodPost {
		a.handleRPC(w, r)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"name":      "heliproxy",
		"rpc":       "POST /?api-key=<heliproxy_client_key>",
		"dashboard": "/dashboard?api-key=<admin_key>",
	})
}

func (a *App) handleRPC(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, APIResponse{OK: false, Error: "method_not_allowed"})
		return
	}
	if !a.authorized(r, true) {
		writeJSON(w, http.StatusUnauthorized, APIResponse{OK: false, Error: "invalid_api_key"})
		return
	}
	cfg := a.manager.Config()
	maxBody := cfg.Routing.MaxBodyBytes
	if maxBody <= 0 {
		maxBody = defaultMaxBodyBytes
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, int64(maxBody)))
	if err != nil {
		writeJSON(w, http.StatusRequestEntityTooLarge, APIResponse{OK: false, Error: "request_body_too_large"})
		return
	}
	if len(bytes.TrimSpace(body)) == 0 {
		writeJSON(w, http.StatusBadRequest, APIResponse{OK: false, Error: "empty_body"})
		return
	}

	attempts := a.manager.NextAttempts(time.Now().UTC())
	if len(attempts) == 0 {
		writeJSON(w, http.StatusServiceUnavailable, APIResponse{OK: false, Error: "no_available_helius_keys"})
		return
	}

	results := make([]ProxyAttemptResult, 0, len(attempts))
	for _, attempt := range attempts {
		started := time.Now()
		resp, respBody, err := a.forwardRPC(r.Context(), cfg, r, body, attempt.Key)
		latency := time.Since(started)
		result := ProxyAttemptResult{
			KeyID:        attempt.Key.ID,
			KeyName:      attempt.Key.Name,
			APIKeyMasked: maskSecret(attempt.Key.APIKey),
			LatencyMS:    latency.Milliseconds(),
		}
		if err != nil {
			message := compactError(err.Error())
			result.Error = message
			results = append(results, result)
			a.manager.MarkAttempt(attempt.Key.ID, false, 0, message, latency, true)
			continue
		}

		result.Status = resp.StatusCode
		if shouldFailoverStatus(resp.StatusCode) {
			message := upstreamErrorMessage(respBody, resp.Status)
			result.Error = message
			results = append(results, result)
			a.manager.MarkAttempt(attempt.Key.ID, false, resp.StatusCode, message, latency, true)
			continue
		}

		results = append(results, result)
		a.manager.MarkAttempt(attempt.Key.ID, true, resp.StatusCode, "", latency, false)
		copyResponseHeaders(w.Header(), resp.Header)
		w.Header().Set("X-Heliproxy-Key-ID", attempt.Key.ID)
		w.WriteHeader(resp.StatusCode)
		_, _ = w.Write(respBody)
		return
	}

	writeJSON(w, http.StatusServiceUnavailable, map[string]any{
		"error": map[string]any{
			"message":  "all helius keys failed",
			"attempts": results,
		},
	})
}

func (a *App) forwardRPC(ctx context.Context, cfg *Config, original *http.Request, body []byte, key HeliusKey) (*http.Response, []byte, error) {
	upstreamURL, err := buildRPCURL(cfg.Helius.RPCBaseURL, key.APIKey)
	if err != nil {
		return nil, nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, upstreamURL, bytes.NewReader(body))
	if err != nil {
		return nil, nil, err
	}
	copyRequestHeaders(req.Header, original.Header)
	if req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("User-Agent", userAgent(original.Header.Get("User-Agent")))

	client := &http.Client{Timeout: timeoutDuration(cfg)}
	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, err
	}
	return resp, respBody, nil
}

func (a *App) forwardREST(ctx context.Context, cfg *Config, original *http.Request, body []byte, key HeliusKey) (*http.Response, []byte, error) {
	upstreamURL, err := buildRESTURL(cfg.Helius.RestBaseURL, original.URL, key.APIKey)
	if err != nil {
		return nil, nil, err
	}
	req, err := http.NewRequestWithContext(ctx, original.Method, upstreamURL, bytes.NewReader(body))
	if err != nil {
		return nil, nil, err
	}
	copyRequestHeaders(req.Header, original.Header)
	if len(body) > 0 && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("User-Agent", userAgent(original.Header.Get("User-Agent")))

	client := &http.Client{Timeout: timeoutDuration(cfg)}
	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, err
	}
	return resp, respBody, nil
}

func (a *App) handleHealth(w http.ResponseWriter, r *http.Request) {
	status := a.manager.Status(time.Now().UTC())
	code := http.StatusOK
	if status.ConfiguredKeys > 0 && status.AvailableKeys == 0 {
		code = http.StatusServiceUnavailable
	}
	writeJSON(w, code, map[string]any{
		"ok":              code == http.StatusOK,
		"configured_keys": status.ConfiguredKeys,
		"enabled_keys":    status.EnabledKeys,
		"available_keys":  status.AvailableKeys,
	})
}

func (a *App) handleGetConfig(w http.ResponseWriter, r *http.Request) {
	if !a.authorized(r, false) {
		writeJSON(w, http.StatusUnauthorized, APIResponse{OK: false, Error: "invalid_admin_key"})
		return
	}
	writeJSON(w, http.StatusOK, APIResponse{OK: true, Data: publicConfig(a.manager.Config(), a.store.Path)})
}

func (a *App) handlePutConfig(w http.ResponseWriter, r *http.Request) {
	if !a.authorized(r, false) {
		writeJSON(w, http.StatusUnauthorized, APIResponse{OK: false, Error: "invalid_admin_key"})
		return
	}
	var incoming Config
	if err := json.NewDecoder(r.Body).Decode(&incoming); err != nil {
		writeJSON(w, http.StatusBadRequest, APIResponse{OK: false, Error: "invalid_json: " + err.Error()})
		return
	}
	merged, err := mergeConfigUpdate(a.manager.Config(), &incoming)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, APIResponse{OK: false, Error: err.Error()})
		return
	}
	if err := a.store.Save(merged); err != nil {
		writeJSON(w, http.StatusInternalServerError, APIResponse{OK: false, Error: err.Error()})
		return
	}
	a.manager.UpdateConfig(merged)
	writeJSON(w, http.StatusOK, APIResponse{OK: true, Data: publicConfig(merged, a.store.Path)})
}

func (a *App) handleStatus(w http.ResponseWriter, r *http.Request) {
	if !a.authorized(r, false) {
		writeJSON(w, http.StatusUnauthorized, APIResponse{OK: false, Error: "invalid_admin_key"})
		return
	}
	writeJSON(w, http.StatusOK, APIResponse{OK: true, Data: a.manager.Status(time.Now().UTC())})
}

func (a *App) handleRefreshUsage(w http.ResponseWriter, r *http.Request) {
	if !a.authorized(r, false) {
		writeJSON(w, http.StatusUnauthorized, APIResponse{OK: false, Error: "invalid_admin_key"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), timeoutDuration(a.manager.Config())+5*time.Second)
	defer cancel()
	results := a.refreshUsage(ctx)
	writeJSON(w, http.StatusOK, APIResponse{OK: true, Data: results})
}

func (a *App) authorized(r *http.Request, allowClient bool) bool {
	presented := extractAPIKey(r)
	if presented == "" {
		return false
	}
	cfg := a.manager.Config()
	if containsSecret(cfg.Auth.AdminKeys, presented) {
		return true
	}
	return allowClient && containsSecret(cfg.Auth.ClientKeys, presented)
}

func extractAPIKey(r *http.Request) string {
	query := r.URL.Query()
	for _, key := range []string{"api-key", "api_key", "apikey"} {
		if v := strings.TrimSpace(query.Get(key)); v != "" {
			return v
		}
	}
	if v := strings.TrimSpace(r.Header.Get("X-Api-Key")); v != "" {
		return v
	}
	if auth := strings.TrimSpace(r.Header.Get("Authorization")); auth != "" {
		parts := strings.Fields(auth)
		if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
			return strings.TrimSpace(parts[1])
		}
	}
	return ""
}

func containsSecret(values []string, needle string) bool {
	needle = strings.TrimSpace(needle)
	if needle == "" {
		return false
	}
	for _, v := range values {
		if strings.TrimSpace(v) == needle {
			return true
		}
	}
	return false
}

func buildRPCURL(baseURL, apiKey string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil {
		return "", err
	}
	if u.Scheme == "" || u.Host == "" {
		return "", fmt.Errorf("invalid helius rpc url")
	}
	q := u.Query()
	q.Set("api-key", apiKey)
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func buildRESTURL(baseURL string, original *url.URL, apiKey string) (string, error) {
	base, err := url.Parse(strings.TrimRight(strings.TrimSpace(baseURL), "/"))
	if err != nil {
		return "", err
	}
	if base.Scheme == "" || base.Host == "" {
		return "", fmt.Errorf("invalid helius rest url")
	}
	upstream := *base
	basePath := strings.TrimRight(base.EscapedPath(), "/")
	requestPath := "/"
	if original != nil && original.EscapedPath() != "" {
		requestPath = original.EscapedPath()
	}
	upstream.Path = basePath + requestPath
	upstream.RawPath = ""
	q := url.Values{}
	if original != nil {
		for key, values := range original.Query() {
			if isClientAPIKeyParam(key) {
				continue
			}
			for _, value := range values {
				q.Add(key, value)
			}
		}
	}
	q.Set("api-key", apiKey)
	upstream.RawQuery = q.Encode()
	return upstream.String(), nil
}

func isClientAPIKeyParam(key string) bool {
	switch strings.ToLower(strings.TrimSpace(key)) {
	case "api-key", "api_key", "apikey":
		return true
	default:
		return false
	}
}

func copyRequestHeaders(dst http.Header, src http.Header) {
	for key, values := range src {
		canonical := http.CanonicalHeaderKey(key)
		if isHopByHopHeader(canonical) || canonical == "Authorization" || canonical == "X-Api-Key" || canonical == "Host" || canonical == "Content-Length" {
			continue
		}
		for _, value := range values {
			dst.Add(canonical, value)
		}
	}
}

func copyResponseHeaders(dst http.Header, src http.Header) {
	for key, values := range src {
		canonical := http.CanonicalHeaderKey(key)
		if isHopByHopHeader(canonical) || canonical == "Content-Length" {
			continue
		}
		for _, value := range values {
			dst.Add(canonical, value)
		}
	}
}

func isHopByHopHeader(key string) bool {
	switch http.CanonicalHeaderKey(key) {
	case "Connection", "Keep-Alive", "Proxy-Authenticate", "Proxy-Authorization", "Te", "Trailer", "Transfer-Encoding", "Upgrade":
		return true
	default:
		return false
	}
}

func shouldFailoverStatus(status int) bool {
	switch status {
	case http.StatusUnauthorized, http.StatusForbidden, http.StatusRequestTimeout, http.StatusTooManyRequests,
		http.StatusInternalServerError, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return true
	default:
		return false
	}
}

func upstreamErrorMessage(body []byte, fallback string) string {
	var payload any
	if err := json.Unmarshal(body, &payload); err == nil {
		if msg := findErrorMessage(payload); msg != "" {
			return compactError(msg)
		}
	}
	text := strings.TrimSpace(string(body))
	if text == "" {
		text = fallback
	}
	return compactError(text)
}

func findErrorMessage(v any) string {
	switch typed := v.(type) {
	case map[string]any:
		for _, key := range []string{"message", "error", "errorMessage"} {
			if val, ok := typed[key]; ok {
				switch inner := val.(type) {
				case string:
					return inner
				case map[string]any:
					if msg := findErrorMessage(inner); msg != "" {
						return msg
					}
				}
			}
		}
	case []any:
		for _, item := range typed {
			if msg := findErrorMessage(item); msg != "" {
				return msg
			}
		}
	}
	return ""
}

func compactError(message string) string {
	message = strings.TrimSpace(strings.ReplaceAll(message, "\n", " "))
	if len(message) > 240 {
		return message[:240]
	}
	return message
}

func userAgent(original string) string {
	original = strings.TrimSpace(original)
	if original == "" {
		return "heliproxy/0.1"
	}
	return original + " heliproxy/0.1"
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(recorder, r)
		log.Printf("%s %s %d %s", r.Method, safeLogPath(r.URL), recorder.status, time.Since(started).Round(time.Millisecond))
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func safeLogPath(u *url.URL) string {
	if u == nil {
		return ""
	}
	copy := *u
	q := copy.Query()
	for _, key := range []string{"api-key", "api_key", "apikey"} {
		if q.Get(key) != "" {
			q.Set(key, maskedSecretPlaceholder)
		}
	}
	copy.RawQuery = q.Encode()
	return copy.RequestURI()
}

func isNetworkError(err error) bool {
	var netErr net.Error
	return errors.As(err, &netErr)
}
