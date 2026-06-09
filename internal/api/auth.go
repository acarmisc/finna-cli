package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// ---- request / response types -----------------------------------------------

// LoginRequest matches /api/v1/auth/login (TokenRequest schema).
type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// LoginResponse is the shape returned by /api/v1/auth/login.
type LoginResponse struct {
	AccessToken string `json:"access_token"`
	Token       string `json:"token"`       // /api/v1/auth/token uses "token"
	TokenType   string `json:"token_type"`
}

// AccessTokenValue returns whichever token field is populated.
func (r *LoginResponse) AccessTokenValue() string {
	if r.AccessToken != "" {
		return r.AccessToken
	}
	return r.Token
}

// GitHubRedirectResponse is returned by GET /api/v1/auth/github.
type GitHubRedirectResponse struct {
	URL string `json:"url"`
}

// GitHubCallbackRequest matches the GitHubCallbackRequest schema.
type GitHubCallbackRequest struct {
	Code string `json:"code"`
}

// GitHubCallbackResponse is the shape returned by POST /api/v1/auth/github/callback.
type GitHubCallbackResponse struct {
	Token string `json:"token"`
}

// OIDCProviderPublic matches ProviderPublic schema.
type OIDCProviderPublic struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Issuer  string `json:"issuer"`
	Enabled bool   `json:"enabled"`
}

// OIDCLoginRequest matches OIDCLoginRequest schema.
type OIDCLoginRequest struct {
	ProviderID string `json:"provider_id"`
}

// OIDCLoginResponse matches OIDCLoginResponse schema.
type OIDCLoginResponse struct {
	AuthorizationURL string `json:"authorization_url"`
	State            string `json:"state"`
}

// OIDCCallbackRequest matches OIDCCallbackRequest schema.
type OIDCCallbackRequest struct {
	ProviderID string `json:"provider_id"`
	Code       string `json:"code"`
	State      string `json:"state"`
}

// OIDCCallbackResponse matches OIDCCallbackResponse schema.
type OIDCCallbackResponse struct {
	Token    string `json:"token"`
	UserID   int    `json:"user_id"`
	Username string `json:"username"`
}

// GCPRegisterRequest matches GCPRegisterRequest schema.
type GCPRegisterRequest struct {
	ProjectID      string `json:"project_id"`
	KeyFileContent string `json:"key_file_content,omitempty"`
}

// AzureRegisterRequest matches AzureServiceAccountRegisterRequest schema.
type AzureRegisterRequest struct {
	TenantID       string `json:"tenant_id"`
	ClientID       string `json:"client_id"`
	ClientSecret   string `json:"client_secret,omitempty"`
	SubscriptionID string `json:"subscription_id,omitempty"`
}

// AuthProviderConfig is the nested config block for AuthProviderInput.
type AuthProviderConfig struct {
	Issuer       string `json:"issuer,omitempty"`
	ClientID     string `json:"client_id,omitempty"`
	ClientSecret string `json:"client_secret,omitempty"`
	// Additional fields passed through.
	Extra map[string]any `json:"-"`
}

// AuthProviderInput matches AuthProviderInput schema.
type AuthProviderInput struct {
	Name    string              `json:"name"`
	Kind    string              `json:"kind,omitempty"`
	Enabled bool                `json:"enabled"`
	Config  AuthProviderConfig  `json:"config"`
}

// AuthProviderResponse matches AuthProviderResponse schema.
type AuthProviderResponse struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Kind        string         `json:"kind"`
	Enabled     bool           `json:"enabled"`
	Config      map[string]any `json:"config"`
	CreatedAt   string         `json:"created_at,omitempty"`
	UpdatedAt   string         `json:"updated_at,omitempty"`
	CreatedBy   string         `json:"created_by,omitempty"`
	LastTestAt  string         `json:"last_test_at,omitempty"`
	LastTestOK  *bool          `json:"last_test_ok"`
}

// ---- API methods ------------------------------------------------------------

// Login posts credentials to /api/v1/auth/login and returns the JWT.
func (c *Client) Login(ctx context.Context, req LoginRequest) (*LoginResponse, error) {
	return postJSON[LoginResponse](ctx, c, "/api/v1/auth/login", req)
}

// GitHubRedirectURL fetches the GitHub OAuth authorization URL.
func (c *Client) GitHubRedirectURL(ctx context.Context) (*GitHubRedirectResponse, error) {
	resp, err := c.Do(ctx, http.MethodGet, "/api/v1/auth/github", nil, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	var r GitHubRedirectResponse
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return nil, fmt.Errorf("decode github redirect: %w", err)
	}
	return &r, nil
}

// GitHubCallback exchanges an OAuth code for a JWT.
func (c *Client) GitHubCallback(ctx context.Context, code string) (*GitHubCallbackResponse, error) {
	return postJSON[GitHubCallbackResponse](ctx, c, "/api/v1/auth/github/callback", GitHubCallbackRequest{Code: code})
}

// OIDCProviders lists enabled public OIDC providers (unauthenticated).
func (c *Client) OIDCProviders(ctx context.Context) ([]OIDCProviderPublic, error) {
	var out []OIDCProviderPublic
	if err := c.GetJSON(ctx, "/api/v1/auth/oidc/providers", &out); err != nil {
		return nil, err
	}
	return out, nil
}

// OIDCLogin initiates the PKCE flow and returns the authorization URL + state.
func (c *Client) OIDCLogin(ctx context.Context, providerID string) (*OIDCLoginResponse, error) {
	return postJSON[OIDCLoginResponse](ctx, c, "/api/v1/auth/oidc/login", OIDCLoginRequest{ProviderID: providerID})
}

// OIDCCallback completes the PKCE callback and returns a Finna JWT.
func (c *Client) OIDCCallback(ctx context.Context, req OIDCCallbackRequest) (*OIDCCallbackResponse, error) {
	return postJSON[OIDCCallbackResponse](ctx, c, "/api/v1/auth/oidc/callback", req)
}

// RegisterGCP sends GCP service account credentials.
func (c *Client) RegisterGCP(ctx context.Context, req GCPRegisterRequest) (*map[string]any, error) {
	return postJSON[map[string]any](ctx, c, "/api/v1/auth/gcp/register", req)
}

// RegisterAzure sends Azure service principal credentials.
func (c *Client) RegisterAzure(ctx context.Context, req AzureRegisterRequest) (*map[string]any, error) {
	return postJSON[map[string]any](ctx, c, "/api/v1/auth/azure/service-account", req)
}

// ---- Admin: OIDC providers --------------------------------------------------

// AdminListProviders lists all OIDC providers (admin).
func (c *Client) AdminListProviders(ctx context.Context) ([]AuthProviderResponse, error) {
	var out []AuthProviderResponse
	if err := c.GetJSON(ctx, "/api/v1/auth/providers", &out); err != nil {
		return nil, err
	}
	return out, nil
}

// AdminGetProvider fetches a single OIDC provider by ID (admin).
func (c *Client) AdminGetProvider(ctx context.Context, id string) (*AuthProviderResponse, error) {
	return getJSONPath[AuthProviderResponse](ctx, c, "/api/v1/auth/providers/"+id)
}

// AdminCreateProvider creates a new OIDC provider (admin).
func (c *Client) AdminCreateProvider(ctx context.Context, req AuthProviderInput) (*AuthProviderResponse, error) {
	return postJSON[AuthProviderResponse](ctx, c, "/api/v1/auth/providers", req)
}

// AdminUpdateProvider replaces an OIDC provider configuration (admin).
func (c *Client) AdminUpdateProvider(ctx context.Context, id string, req AuthProviderInput) (*AuthProviderResponse, error) {
	return putJSON[AuthProviderResponse](ctx, c, "/api/v1/auth/providers/"+id, req)
}

// AdminDeleteProvider deletes a provider (admin).
func (c *Client) AdminDeleteProvider(ctx context.Context, id string) (*map[string]string, error) {
	return deleteJSON[map[string]string](ctx, c, "/api/v1/auth/providers/"+id)
}

// AdminTestProvider tests OIDC provider connectivity (admin).
func (c *Client) AdminTestProvider(ctx context.Context, id string) (*map[string]any, error) {
	return postJSON[map[string]any](ctx, c, "/api/v1/auth/providers/"+id+"/test", nil)
}

// ---- internal helpers -------------------------------------------------------

func postJSON[T any](ctx context.Context, c *Client, path string, body any) (*T, error) {
	var r io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal request: %w", err)
		}
		r = bytes.NewReader(b)
	}
	resp, err := c.Do(ctx, http.MethodPost, path, r, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	var out T
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &out, nil
}

func putJSON[T any](ctx context.Context, c *Client, path string, body any) (*T, error) {
	b, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}
	resp, err := c.Do(ctx, http.MethodPut, path, bytes.NewReader(b), nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	var out T
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &out, nil
}

func deleteJSON[T any](ctx context.Context, c *Client, path string) (*T, error) {
	resp, err := c.Do(ctx, http.MethodDelete, path, nil, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	var out T
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &out, nil
}

func getJSONPath[T any](ctx context.Context, c *Client, path string) (*T, error) {
	var out T
	if err := c.GetJSON(ctx, path, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
