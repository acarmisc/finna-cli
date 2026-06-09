package api_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/acarmisc/finna-cli/internal/api"
)

func newTestClient(t *testing.T, h http.Handler, opts ...api.Option) *api.Client {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	all := append([]api.Option{api.WithHTTPClient(srv.Client())}, opts...)
	return api.New(srv.URL, func() (string, error) { return "tok", nil }, all...)
}

func TestDo_SuccessAttachesAuthAndUA(t *testing.T) {
	var gotAuth, gotUA string
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotUA = r.Header.Get("User-Agent")
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))

	ctx := context.Background()
	resp, err := c.Do(ctx, http.MethodGet, "/x", nil, nil)
	require.NoError(t, err)
	_ = resp.Body.Close()
	require.Equal(t, "Bearer tok", gotAuth)
	require.True(t, strings.HasPrefix(gotUA, "finna-cli/"))
}

func TestDo_4xxNotRetried(t *testing.T) {
	var calls int32
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(404)
		_, _ = w.Write([]byte(`{"detail":"nope"}`))
	}))

	_, err := c.Do(context.Background(), http.MethodGet, "/x", nil, nil)
	require.Error(t, err)
	var apiErr *api.APIError
	require.True(t, errors.As(err, &apiErr))
	require.Equal(t, 404, apiErr.StatusCode)
	require.Equal(t, int32(1), atomic.LoadInt32(&calls), "4xx must not retry")
}

func TestDo_5xxRetriesThenSucceeds(t *testing.T) {
	var calls int32
	c := newTestClient(t,
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			n := atomic.AddInt32(&calls, 1)
			if n < 3 {
				w.WriteHeader(503)
				return
			}
			w.WriteHeader(200)
			_, _ = w.Write([]byte(`{"ok":true}`))
		}),
		api.WithMaxRetries(3),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	resp, err := c.Do(ctx, http.MethodGet, "/x", nil, nil)
	require.NoError(t, err)
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
	require.Equal(t, int32(3), atomic.LoadInt32(&calls))
}

func TestDo_5xxExhaustsRetries(t *testing.T) {
	var calls int32
	c := newTestClient(t,
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			atomic.AddInt32(&calls, 1)
			w.WriteHeader(500)
			_, _ = w.Write([]byte(`{"detail":"boom"}`))
		}),
		api.WithMaxRetries(2),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := c.Do(ctx, http.MethodGet, "/x", nil, nil)
	require.Error(t, err)
	var apiErr *api.APIError
	require.True(t, errors.As(err, &apiErr))
	require.Equal(t, 500, apiErr.StatusCode)
	// maxRetries=2 means up to 3 total requests.
	require.Equal(t, int32(3), atomic.LoadInt32(&calls))
}
