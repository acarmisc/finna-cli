package auth_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zalando/go-keyring"

	"github.com/acarmisc/finna-cli/internal/auth"
)

// useMock switches go-keyring to its built-in in-memory mock provider for
// the duration of the test and resets it afterwards.
func useMock(t *testing.T) {
	t.Helper()
	keyring.MockInit()
	t.Cleanup(func() { keyring.MockInit() }) // reset between tests
}

func TestStore_SetGetDelete(t *testing.T) {
	useMock(t)

	// Nothing stored yet.
	_, err := auth.Get("prod")
	require.ErrorIs(t, err, auth.ErrNoToken)

	// Set, then get.
	require.NoError(t, auth.Set("prod", "tok-abc"))
	tok, err := auth.Get("prod")
	require.NoError(t, err)
	require.Equal(t, "tok-abc", tok)

	// Delete.
	require.NoError(t, auth.Delete("prod"))
	_, err = auth.Get("prod")
	require.ErrorIs(t, err, auth.ErrNoToken)

	// Double-delete must not error.
	require.NoError(t, auth.Delete("prod"))
}

func TestStore_DeleteAll(t *testing.T) {
	useMock(t)
	require.NoError(t, auth.Set("prod", "tok-prod"))
	require.NoError(t, auth.Set("dev", "tok-dev"))

	require.NoError(t, auth.DeleteAll([]string{"prod", "dev", "nonexistent"}))

	_, err := auth.Get("prod")
	require.ErrorIs(t, err, auth.ErrNoToken)
	_, err = auth.Get("dev")
	require.ErrorIs(t, err, auth.ErrNoToken)
}

func TestStore_TokenProvider_NoToken(t *testing.T) {
	useMock(t)
	fn := auth.TokenProvider("missing")
	tok, err := fn()
	require.NoError(t, err) // ErrNoToken must be swallowed, not propagated
	require.Empty(t, tok)
}

func TestStore_TokenProvider_WithToken(t *testing.T) {
	useMock(t)
	require.NoError(t, auth.Set("ctx", "jwt-xyz"))
	fn := auth.TokenProvider("ctx")
	tok, err := fn()
	require.NoError(t, err)
	require.Equal(t, "jwt-xyz", tok)
}
