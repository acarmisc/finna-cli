package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"strings"
	"time"

	"github.com/acarmisc/finna-cli/internal/version"
)

// TokenProvider returns a Bearer token for the current request. Returning an
// empty string (and nil error) means "no auth header" — endpoints that
// require auth will then reject with 401.
type TokenProvider func() (string, error)

// Client is the thin wrapper around the generated OpenAPI client. It is the
// only type CLI commands should depend on.
type Client struct {
	server     string
	http       *http.Client
	token      TokenProvider
	userAgent  string
	maxRetries int
	debug      bool
	debugSink  io.Writer
}

// Option mutates a Client during construction.
type Option func(*Client)

// WithHTTPClient overrides the default http.Client (useful in tests).
func WithHTTPClient(h *http.Client) Option { return func(c *Client) { c.http = h } }

// WithMaxRetries sets the maximum retry attempts for transient 5xx errors.
// Default is 3 (i.e. up to 4 total requests).
func WithMaxRetries(n int) Option { return func(c *Client) { c.maxRetries = n } }

// WithDebug enables request/response tracing to w (typically os.Stderr).
func WithDebug(w io.Writer) Option {
	return func(c *Client) {
		c.debug = true
		c.debugSink = w
	}
}

// WithUserAgent overrides the default User-Agent string.
func WithUserAgent(ua string) Option { return func(c *Client) { c.userAgent = ua } }

// New constructs a Client. server must be a base URL like "https://api.host".
func New(server string, token TokenProvider, opts ...Option) *Client {
	c := &Client{
		server:     strings.TrimRight(server, "/"),
		http:       &http.Client{Timeout: 30 * time.Second},
		token:      token,
		userAgent:  fmt.Sprintf("finna-cli/%s", version.Version),
		maxRetries: 3,
	}
	for _, o := range opts {
		o(c)
	}
	return c
}

// Server returns the configured base URL.
func (c *Client) Server() string { return c.server }

// Do issues an HTTP request with auth, retry, and error decoding. On a
// non-2xx response it returns a typed *APIError (and resp will be nil).
// On success the caller owns resp.Body and must close it.
func (c *Client) Do(ctx context.Context, method, path string, body io.Reader, headers map[string]string) (*http.Response, error) {
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	url := c.server + path

	// Buffer body once so we can replay across retries.
	var bodyBytes []byte
	if body != nil {
		var err error
		bodyBytes, err = io.ReadAll(body)
		if err != nil {
			return nil, fmt.Errorf("buffer request body: %w", err)
		}
	}

	var lastErr error
	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		var rdr io.Reader
		if bodyBytes != nil {
			rdr = strings.NewReader(string(bodyBytes))
		}
		req, err := http.NewRequestWithContext(ctx, method, url, rdr)
		if err != nil {
			return nil, fmt.Errorf("build request: %w", err)
		}
		req.Header.Set("User-Agent", c.userAgent)
		req.Header.Set("Accept", "application/json")
		if bodyBytes != nil && req.Header.Get("Content-Type") == "" {
			req.Header.Set("Content-Type", "application/json")
		}
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		if c.token != nil {
			tok, err := c.token()
			if err != nil {
				return nil, fmt.Errorf("fetch token: %w", err)
			}
			if tok != "" {
				req.Header.Set("Authorization", "Bearer "+tok)
			}
		}

		if c.debug && c.debugSink != nil {
			fmt.Fprintf(c.debugSink, "[debug] --> %s %s\n", method, url)
			for k, vals := range req.Header {
				v := strings.Join(vals, ", ")
				if strings.EqualFold(k, "Authorization") {
					v = "Bearer ***"
				}
				fmt.Fprintf(c.debugSink, "[debug]     %s: %s\n", k, v)
			}
		}
		resp, err := c.http.Do(req)
		if err != nil {
			lastErr = err
			// Network error — retry with backoff if attempts remain and
			// the context isn't done.
			if attempt < c.maxRetries && ctx.Err() == nil {
				c.sleepBackoff(ctx, attempt)
				continue
			}
			return nil, fmt.Errorf("http: %w", err)
		}
		if c.debug && c.debugSink != nil {
			fmt.Fprintf(c.debugSink, "[debug] <-- %d %s\n", resp.StatusCode, url)
		}

		// Success.
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return resp, nil
		}

		// 4xx — never retry.
		if resp.StatusCode >= 400 && resp.StatusCode < 500 {
			apiErr := DecodeError(resp)
			_ = resp.Body.Close()
			return nil, apiErr
		}

		// 5xx — retry if attempts remain.
		if attempt < c.maxRetries {
			_ = resp.Body.Close()
			c.sleepBackoff(ctx, attempt)
			continue
		}
		apiErr := DecodeError(resp)
		_ = resp.Body.Close()
		return nil, apiErr
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, fmt.Errorf("exhausted retries")
}

// GetJSON issues GET path and decodes the JSON response into out.
func (c *Client) GetJSON(ctx context.Context, path string, out any) error {
	resp, err := c.Do(ctx, http.MethodGet, path, nil, nil)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if out == nil {
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

// HealthInfo is the minimal shape we read from /api/v1/health for version
// drift checks. The backend may return additional fields which we ignore.
type HealthInfo struct {
	Status  string `json:"status"`
	Version string `json:"version"`
}

// Health queries /api/v1/health. The endpoint is unauthenticated.
func (c *Client) Health(ctx context.Context) (*HealthInfo, error) {
	resp, err := c.Do(ctx, http.MethodGet, "/api/v1/health", nil, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	var h HealthInfo
	if err := json.NewDecoder(resp.Body).Decode(&h); err != nil {
		// Body may be empty / not strictly typed; treat as ok.
		return &HealthInfo{}, nil
	}
	return &h, nil
}

// HealthzInfo is the minimal shape returned by /healthz.
type HealthzInfo struct {
	// The /healthz endpoint returns a free-form object; we capture the
	// commonly-used fields and store the rest in Extra.
	Status string         `json:"status"`
	DB     string         `json:"db"`
	Extra  map[string]any `json:"-"`
}

// Healthz queries /healthz (Kubernetes liveness probe style).
func (c *Client) Healthz(ctx context.Context) (*HealthzInfo, error) {
	resp, err := c.Do(ctx, http.MethodGet, "/healthz", nil, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	var raw map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return &HealthzInfo{}, nil
	}
	h := &HealthzInfo{Extra: raw}
	if v, ok := raw["status"].(string); ok {
		h.Status = v
	}
	if v, ok := raw["db"].(string); ok {
		h.DB = v
	}
	return h, nil
}

// DBStats queries /api/v1/db/stats and returns the raw map returned by the
// server. The schema is open (additionalProperties) so we return map[string]any.
func (c *Client) DBStats(ctx context.Context) (map[string]any, error) {
	resp, err := c.Do(ctx, http.MethodGet, "/api/v1/db/stats", nil, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	var out map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return map[string]any{}, nil
	}
	return out, nil
}

// Page is the generic page envelope expected by FetchAll.
// Implementations are expected to satisfy this by wrapping API responses.
type Page[T any] struct {
	Items   []T
	HasNext bool
}

// FetchAll loops with limit+offset until the page is empty, collecting all
// items. It is generic and relies on the caller-supplied fetch function.
// limit controls the page size; use 0 to let the server decide.
func FetchAll[T any](ctx context.Context, limit int, fetch func(ctx context.Context, offset, limit int) ([]T, error)) ([]T, error) {
	if limit <= 0 {
		limit = 100
	}
	var all []T
	offset := 0
	for {
		page, err := fetch(ctx, offset, limit)
		if err != nil {
			return nil, err
		}
		all = append(all, page...)
		if len(page) < limit {
			break
		}
		offset += limit
	}
	return all, nil
}

func (c *Client) sleepBackoff(ctx context.Context, attempt int) {
	// Exponential: 200ms, 400ms, 800ms ... plus up to 100ms jitter.
	base := time.Duration(200*(1<<attempt)) * time.Millisecond
	jitter := rand.IntN(100)
	d := base + time.Duration(jitter)*time.Millisecond
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
	case <-t.C:
	}
}

