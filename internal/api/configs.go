package api

import (
	"context"
	"fmt"
	"net/http"
)

// ---- request / response types -----------------------------------------------

// CloudConfigCreate matches the CloudConfigCreate schema.
type CloudConfigCreate struct {
	Provider       string         `json:"provider"`
	Name           string         `json:"name"`
	CredentialType string         `json:"credential_type,omitempty"`
	Config         map[string]any `json:"config"`
}

// CloudConfigUpdate matches the CloudConfigUpdate schema (all fields optional).
type CloudConfigUpdate struct {
	Name           *string        `json:"name,omitempty"`
	CredentialType *string        `json:"credential_type,omitempty"`
	Config         map[string]any `json:"config,omitempty"`
}

// CloudConfigResponse matches the CloudConfigResponse schema.
type CloudConfigResponse struct {
	ID             string         `json:"id"`
	Provider       string         `json:"provider"`
	Name           string         `json:"name"`
	CredentialType string         `json:"credential_type"`
	Config         map[string]any `json:"config"`
	CreatedAt      string         `json:"created_at"`
	UpdatedAt      string         `json:"updated_at"`
	LastTest       string         `json:"last_test,omitempty"`
	LastTestAt     string         `json:"last_test_at,omitempty"`
	TenantID       string         `json:"tenant_id,omitempty"`
	SubscriptionID string         `json:"subscription_id,omitempty"`
	ProjectID      string         `json:"project_id,omitempty"`
	Err            string         `json:"err,omitempty"`
}

// CLIConfigsListResponse is the paginated wrapper returned by GET /api/v1/configs.
type CLIConfigsListResponse struct {
	Data  []CloudConfigResponse `json:"data"`
	Total int                   `json:"total"`
	Page  int                   `json:"page"`
}

// CLIConfigCreate matches the _ConfigCreate schema used by /api/v1/configs.
type CLIConfigCreate struct {
	Provider        string `json:"provider"`
	CloudConfig     string `json:"cloud_config,omitempty"`
	ServiceCategory string `json:"service_category,omitempty"`
	Region          string `json:"region,omitempty"`
}

// ---- API methods ------------------------------------------------------------

// ListConfigs returns all configs via the CLI-compatible /api/v1/configs endpoint.
// Falls back to /api/v1/config if the wrapper endpoint returns unexpected shape.
func (c *Client) ListConfigs(ctx context.Context) ([]CloudConfigResponse, error) {
	// Try CLI alias first (returns paginated wrapper).
	var wrapper CLIConfigsListResponse
	if err := c.GetJSON(ctx, "/api/v1/configs", &wrapper); err != nil {
		return nil, err
	}
	// Some server versions may return a flat array here; handle both.
	if wrapper.Data != nil {
		return wrapper.Data, nil
	}
	// If Data is nil the server returned a flat array — retry against /api/v1/config.
	var flat []CloudConfigResponse
	if err := c.GetJSON(ctx, "/api/v1/config", &flat); err != nil {
		return nil, err
	}
	return flat, nil
}

// GetConfig fetches a single config via GET /api/v1/configs/{id}.
func (c *Client) GetConfig(ctx context.Context, id string) (*CloudConfigResponse, error) {
	var out CloudConfigResponse
	if err := c.GetJSON(ctx, "/api/v1/configs/"+id, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// CreateConfig posts to /api/v1/configs (CLI-compatible endpoint).
func (c *Client) CreateConfig(ctx context.Context, req CloudConfigCreate) (*CloudConfigResponse, error) {
	// The CLI endpoint takes _ConfigCreate shape; we use the richer /api/v1/config endpoint.
	return postJSON[CloudConfigResponse](ctx, c, "/api/v1/config", req)
}

// UpdateConfig sends a partial update to /api/v1/configs/{id}.
func (c *Client) UpdateConfig(ctx context.Context, id string, req CloudConfigUpdate) (*CloudConfigResponse, error) {
	return putJSON[CloudConfigResponse](ctx, c, "/api/v1/configs/"+id, req)
}

// DeleteConfig deletes via DELETE /api/v1/configs/{id}. Returns nil on 204.
func (c *Client) DeleteConfig(ctx context.Context, id string) error {
	resp, err := c.Do(ctx, http.MethodDelete, "/api/v1/configs/"+id, nil, nil)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	return nil
}

// TestConfig calls POST /api/v1/config/{id}/test (note: /config not /configs).
func (c *Client) TestConfig(ctx context.Context, id string) (map[string]any, error) {
	result, err := postJSON[map[string]any](ctx, c, fmt.Sprintf("/api/v1/config/%s/test", id), nil)
	if err != nil {
		return nil, err
	}
	return *result, nil
}
