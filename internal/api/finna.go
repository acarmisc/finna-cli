package api

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
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

		c.tracef("--> %s %s (attempt %d)", method, url, attempt+1)
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
		c.tracef("<-- %d %s", resp.StatusCode, url)

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

func (c *Client) sleepBackoff(ctx context.Context, attempt int) {
	// Exponential: 200ms, 400ms, 800ms ... plus up to 100ms jitter.
	base := time.Duration(200*(1<<attempt)) * time.Millisecond
	jitterMax := big.NewInt(100)
	j, _ := rand.Int(rand.Reader, jitterMax)
	d := base + time.Duration(j.Int64())*time.Millisecond
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
	case <-t.C:
	}
}

func (c *Client) tracef(format string, args ...any) {
	if !c.debug || c.debugSink == nil {
		return
	}
	fmt.Fprintf(c.debugSink, "[finna debug] "+format+"\n", args...)
}
