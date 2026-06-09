package auth_test

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/acarmisc/finna-cli/internal/auth"
)

// makeJWT builds a minimal, unsigned JWT for testing.
func makeJWT(payload map[string]any) string {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
	b, _ := json.Marshal(payload)
	p := base64.RawURLEncoding.EncodeToString(b)
	return header + "." + p + ".fakesig"
}

func TestDecodeJWT_Fields(t *testing.T) {
	exp := time.Now().Add(2 * time.Hour).Unix()
	tok := makeJWT(map[string]any{
		"sub":      "42",
		"username": "alice",
		"provider": "local",
		"iss":      "finna",
		"exp":      float64(exp),
		"iat":      float64(time.Now().Unix()),
		"is_admin": true,
	})
	c, err := auth.DecodeJWT(tok)
	require.NoError(t, err)
	require.Equal(t, "42", c.Sub)
	require.Equal(t, "alice", c.Username)
	require.Equal(t, "local", c.Provider)
	require.Equal(t, "finna", c.Issuer)
	require.True(t, c.IsAdmin)
	require.False(t, c.Expired())
	require.False(t, c.ExpiresSoon(time.Minute)) // >1h left
}

func TestDecodeJWT_SubFallbackForUsername(t *testing.T) {
	tok := makeJWT(map[string]any{"sub": "bob"})
	c, err := auth.DecodeJWT(tok)
	require.NoError(t, err)
	require.Equal(t, "bob", c.Username)
}

func TestDecodeJWT_Expired(t *testing.T) {
	tok := makeJWT(map[string]any{"exp": float64(time.Now().Add(-1 * time.Hour).Unix())})
	c, err := auth.DecodeJWT(tok)
	require.NoError(t, err)
	require.True(t, c.Expired())
}

func TestDecodeJWT_ExpiresSoon(t *testing.T) {
	tok := makeJWT(map[string]any{"exp": float64(time.Now().Add(30 * time.Minute).Unix())})
	c, err := auth.DecodeJWT(tok)
	require.NoError(t, err)
	require.True(t, c.ExpiresSoon(24*time.Hour))
	require.False(t, c.ExpiresSoon(10*time.Minute)) // still 30m left
}

func TestDecodeJWT_Malformed_WrongParts(t *testing.T) {
	_, err := auth.DecodeJWT("notavalidjwt")
	require.ErrorIs(t, err, auth.ErrMalformedToken)
}

func TestDecodeJWT_Malformed_BadBase64(t *testing.T) {
	_, err := auth.DecodeJWT("hdr.!!!.sig")
	require.ErrorIs(t, err, auth.ErrMalformedToken)
}

func TestDecodeJWT_Malformed_BadJSON(t *testing.T) {
	badPayload := base64.RawURLEncoding.EncodeToString([]byte("not-json"))
	_, err := auth.DecodeJWT("hdr." + badPayload + ".sig")
	require.ErrorIs(t, err, auth.ErrMalformedToken)
}

func TestDecodeJWT_Empty_Token(t *testing.T) {
	_, err := auth.DecodeJWT("")
	require.ErrorIs(t, err, auth.ErrMalformedToken)
}

func TestDecodeJWT_ThreePartsMinimum(t *testing.T) {
	// Two parts only: header.payload (no sig)
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none"}`))
	payload := base64.RawURLEncoding.EncodeToString([]byte(`{"sub":"x"}`))
	_, err := auth.DecodeJWT(header + "." + payload)
	require.ErrorIs(t, err, auth.ErrMalformedToken)
}

// Ensure ExpiresAt returns zero for missing exp.
func TestDecodeJWT_NoExp(t *testing.T) {
	tok := makeJWT(map[string]any{"sub": "x"})
	c, err := auth.DecodeJWT(tok)
	require.NoError(t, err)
	require.True(t, c.ExpiresAt().IsZero())
	require.False(t, c.Expired())
	require.False(t, c.ExpiresSoon(time.Hour))
}

// Verify that strings.Count works fine for "." counting — used internally.
var _ = strings.Count
