package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

var websocketUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

func (a *App) WSRoutes() http.Handler {
	return http.HandlerFunc(a.handleWebSocket)
}

func (a *App) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	if !a.authorized(r, true) {
		writeJSON(w, http.StatusUnauthorized, APIResponse{OK: false, Error: "invalid_api_key"})
		return
	}

	cfg := a.manager.Config()
	attempts := a.manager.NextAttempts(time.Now().UTC())
	if len(attempts) == 0 {
		writeJSON(w, http.StatusServiceUnavailable, APIResponse{OK: false, Error: "no_available_helius_keys"})
		return
	}

	var lastErr string
	for _, attempt := range attempts {
		started := time.Now()
		upstream, resp, err := dialUpstreamWebSocket(r.Context(), cfg, r, attempt.Key)
		latency := time.Since(started)
		if err != nil {
			status := 0
			if resp != nil {
				status = resp.StatusCode
			}
			lastErr = compactError(err.Error())
			a.manager.MarkAttempt(attempt.Key.ID, false, status, lastErr, latency, true)
			continue
		}

		responseHeader := http.Header{}
		responseHeader.Set("X-Heliproxy-Key-ID", attempt.Key.ID)
		client, err := websocketUpgrader.Upgrade(w, r, responseHeader)
		if err != nil {
			_ = upstream.Close()
			lastErr = compactError(err.Error())
			a.manager.MarkAttempt(attempt.Key.ID, false, 0, lastErr, latency, false)
			return
		}

		a.manager.MarkAttempt(attempt.Key.ID, true, http.StatusSwitchingProtocols, "", latency, false)
		bridgeWebSockets(r.Context(), client, upstream)
		return
	}

	writeJSON(w, http.StatusServiceUnavailable, map[string]any{
		"error": map[string]any{
			"message": compactError("all helius websocket keys failed: " + lastErr),
		},
	})
}

func dialUpstreamWebSocket(ctx context.Context, cfg *Config, original *http.Request, key HeliusKey) (*websocket.Conn, *http.Response, error) {
	upstreamURL, err := buildWebSocketURL(cfg.Helius.WSBaseURL, original.URL, key.APIKey)
	if err != nil {
		return nil, nil, err
	}
	header := http.Header{}
	copyRequestHeaders(header, original.Header)
	stripWebSocketHandshakeHeaders(header)
	header.Set("User-Agent", userAgent(original.Header.Get("User-Agent")))
	dialer := websocket.Dialer{
		HandshakeTimeout: timeoutDuration(cfg),
	}
	conn, resp, err := dialer.DialContext(ctx, upstreamURL, header)
	if err != nil {
		if resp != nil {
			return nil, resp, fmt.Errorf("upstream websocket %d: %s", resp.StatusCode, resp.Status)
		}
		return nil, resp, err
	}
	return conn, resp, nil
}

func stripWebSocketHandshakeHeaders(header http.Header) {
	for key := range header {
		canonical := http.CanonicalHeaderKey(key)
		if strings.HasPrefix(canonical, "Sec-Websocket-") || canonical == "Sec-Websocket" {
			delete(header, key)
		}
	}
}

func bridgeWebSockets(ctx context.Context, client, upstream *websocket.Conn) {
	done := make(chan error, 2)
	go copyWebSocket(upstream, client, done)
	go copyWebSocket(client, upstream, done)

	select {
	case err := <-done:
		if err != nil && !isExpectedWebSocketClose(err) {
			log.Printf("websocket bridge closed: %v", compactError(err.Error()))
		}
	case <-ctx.Done():
	}
	_ = client.Close()
	_ = upstream.Close()
}

func copyWebSocket(dst, src *websocket.Conn, done chan<- error) {
	for {
		messageType, reader, err := src.NextReader()
		if err != nil {
			done <- err
			return
		}
		writer, err := dst.NextWriter(messageType)
		if err != nil {
			done <- err
			return
		}
		_, copyErr := io.Copy(writer, reader)
		closeErr := writer.Close()
		if copyErr != nil {
			done <- copyErr
			return
		}
		if closeErr != nil {
			done <- closeErr
			return
		}
	}
}

func isExpectedWebSocketClose(err error) bool {
	return websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) ||
		strings.Contains(err.Error(), "use of closed network connection")
}

func buildWebSocketURL(baseURL string, original *url.URL, apiKey string) (string, error) {
	base, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil {
		return "", err
	}
	if base.Scheme == "http" {
		base.Scheme = "ws"
	}
	if base.Scheme == "https" {
		base.Scheme = "wss"
	}
	if base.Scheme != "ws" && base.Scheme != "wss" {
		return "", fmt.Errorf("invalid helius websocket url scheme")
	}
	if base.Host == "" {
		return "", fmt.Errorf("invalid helius websocket url")
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
	for key, values := range base.Query() {
		if isClientAPIKeyParam(key) {
			continue
		}
		for _, value := range values {
			q.Add(key, value)
		}
	}
	q.Set("api-key", apiKey)
	upstream.RawQuery = q.Encode()
	return upstream.String(), nil
}
