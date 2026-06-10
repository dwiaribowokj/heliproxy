package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type ProjectUsage struct {
	CreditsRemaining        float64                    `json:"creditsRemaining"`
	CreditsUsed             float64                    `json:"creditsUsed"`
	PrepaidCreditsRemaining float64                    `json:"prepaidCreditsRemaining"`
	PrepaidCreditsUsed      float64                    `json:"prepaidCreditsUsed"`
	SubscriptionDetails     AdminSubscriptionDetails   `json:"subscriptionDetails"`
	Usage                   map[string]float64         `json:"usage"`
	Raw                     map[string]json.RawMessage `json:"-"`
}

type AdminSubscriptionDetails struct {
	BillingCycle AdminBillingCycle `json:"billingCycle"`
	CreditsLimit float64           `json:"creditsLimit"`
	Plan         string            `json:"plan"`
}

type AdminBillingCycle struct {
	Start string `json:"start"`
	End   string `json:"end"`
}

type UsageRefreshResult struct {
	ID           string        `json:"id"`
	Name         string        `json:"name"`
	ProjectID    string        `json:"project_id"`
	APIKeyMasked string        `json:"api_key_masked"`
	Usage        *ProjectUsage `json:"usage,omitempty"`
	Error        string        `json:"error,omitempty"`
}

type UsageClient struct {
	HTTPClient *http.Client
	BaseURL    string
}

func NewUsageClient(timeout time.Duration, baseURL string) *UsageClient {
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	if strings.TrimSpace(baseURL) == "" {
		baseURL = defaultAdminBaseURL
	}
	return &UsageClient{
		HTTPClient: &http.Client{Timeout: timeout},
		BaseURL:    strings.TrimRight(baseURL, "/"),
	}
}

func (c *UsageClient) GetProjectUsage(ctx context.Context, apiKey, projectID string) (*ProjectUsage, error) {
	apiKey = strings.TrimSpace(apiKey)
	projectID = strings.TrimSpace(projectID)
	if apiKey == "" {
		return nil, fmt.Errorf("api key is empty")
	}
	if projectID == "" {
		return nil, fmt.Errorf("missing_project_id")
	}
	endpoint := c.BaseURL + "/admin/projects/" + url.PathEscape(projectID) + "/usage"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Api-Key", apiKey)
	req.Header.Set("User-Agent", "heliproxy/0.1")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var body map[string]any
		_ = json.NewDecoder(resp.Body).Decode(&body)
		if msg, ok := body["message"].(string); ok && msg != "" {
			return nil, fmt.Errorf("helius admin api %d: %s", resp.StatusCode, msg)
		}
		if errText, ok := body["error"].(string); ok && errText != "" {
			return nil, fmt.Errorf("helius admin api %d: %s", resp.StatusCode, errText)
		}
		return nil, fmt.Errorf("helius admin api %d", resp.StatusCode)
	}

	var usage ProjectUsage
	if err := json.NewDecoder(resp.Body).Decode(&usage); err != nil {
		return nil, err
	}
	return &usage, nil
}

func (a *App) refreshUsage(ctx context.Context) []UsageRefreshResult {
	cfg := a.manager.Config()
	client := NewUsageClient(timeoutDuration(cfg), cfg.Helius.AdminBaseURL)
	keys := a.manager.KeysForUsage()
	results := make([]UsageRefreshResult, 0, len(keys))
	for _, key := range keys {
		result := UsageRefreshResult{
			ID:           key.ID,
			Name:         key.Name,
			ProjectID:    key.ProjectID,
			APIKeyMasked: maskSecret(key.APIKey),
		}
		usage, err := client.GetProjectUsage(ctx, key.APIKey, key.ProjectID)
		if err != nil {
			result.Error = err.Error()
			a.manager.MarkUsage(key.ID, nil, result.Error)
			results = append(results, result)
			continue
		}
		result.Usage = usage
		a.manager.MarkUsage(key.ID, usage, "")
		results = append(results, result)
	}
	return results
}
