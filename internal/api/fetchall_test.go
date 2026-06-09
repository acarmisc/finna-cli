package api_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/acarmisc/finna-cli/internal/api"
)

func TestFetchAll_SinglePage(t *testing.T) {
	calls := 0
	fetch := func(ctx context.Context, offset, limit int) ([]string, error) {
		calls++
		if offset == 0 {
			return []string{"a", "b", "c"}, nil
		}
		return nil, nil
	}
	items, err := api.FetchAll(context.Background(), 10, fetch)
	require.NoError(t, err)
	require.Equal(t, []string{"a", "b", "c"}, items)
	require.Equal(t, 1, calls) // only one page fetched (3 < 10)
}

func TestFetchAll_MultiplePages(t *testing.T) {
	const pageSize = 3
	data := []string{"a", "b", "c", "d", "e"}
	fetch := func(ctx context.Context, offset, limit int) ([]string, error) {
		end := offset + limit
		if end > len(data) {
			end = len(data)
		}
		if offset >= len(data) {
			return nil, nil
		}
		return data[offset:end], nil
	}
	items, err := api.FetchAll(context.Background(), pageSize, fetch)
	require.NoError(t, err)
	require.Equal(t, data, items)
}

func TestFetchAll_EmptyResult(t *testing.T) {
	fetch := func(ctx context.Context, offset, limit int) ([]int, error) {
		return nil, nil
	}
	items, err := api.FetchAll(context.Background(), 10, fetch)
	require.NoError(t, err)
	require.Empty(t, items)
}

func TestFetchAll_FetchError(t *testing.T) {
	fetch := func(ctx context.Context, offset, limit int) ([]string, error) {
		return nil, fmt.Errorf("server error")
	}
	_, err := api.FetchAll(context.Background(), 10, fetch)
	require.Error(t, err)
	require.Contains(t, err.Error(), "server error")
}

func TestFetchAll_ZeroLimitDefaultsTo100(t *testing.T) {
	calls := 0
	var gotLimit int
	fetch := func(ctx context.Context, offset, limit int) ([]string, error) {
		calls++
		gotLimit = limit
		return nil, nil
	}
	_, err := api.FetchAll(context.Background(), 0, fetch)
	require.NoError(t, err)
	require.Equal(t, 100, gotLimit)
}
