package auth

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Claims is the decoded payload of a Finna JWT. We decode locally; no
// signature verification is needed since the server validates on every API
// call.
type Claims struct {
	Sub      string `json:"sub"`
	Username string `json:"username"`
	IsAdmin  bool   `json:"is_admin"`
	Provider string `json:"provider"` // "local" | "github" | "oidc"
	Issuer   string `json:"iss"`
	IssuedAt int64  `json:"iat"`
	Exp      int64  `json:"exp"`
	// Raw holds the full decoded payload for forward-compat display.
	Raw map[string]any
}

// ExpiresAt returns the expiry as a time.Time (zero if not present).
func (c *Claims) ExpiresAt() time.Time {
	if c.Exp == 0 {
		return time.Time{}
	}
	return time.Unix(c.Exp, 0)
}

// ExpiresSoon returns true when the token expires within d.
func (c *Claims) ExpiresSoon(d time.Duration) bool {
	exp := c.ExpiresAt()
	if exp.IsZero() {
		return false
	}
	return time.Until(exp) < d
}

// Expired returns true when the token has already expired.
func (c *Claims) Expired() bool {
	exp := c.ExpiresAt()
	return !exp.IsZero() && time.Now().After(exp)
}

// ErrMalformedToken is returned when the JWT cannot be decoded.
var ErrMalformedToken = errors.New("malformed token")

// DecodeJWT decodes the payload section of a JWT without verifying the
// signature. This is intentionally read-only; the server is the authority.
func DecodeJWT(token string) (*Claims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("%w: expected 3 parts, got %d", ErrMalformedToken, len(parts))
	}
	// Decode base64url payload (no padding required for standard library).
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("%w: base64 decode: %v", ErrMalformedToken, err)
	}
	var raw map[string]any
	if err := json.Unmarshal(payload, &raw); err != nil {
		return nil, fmt.Errorf("%w: json decode: %v", ErrMalformedToken, err)
	}
	c := &Claims{Raw: raw}
	// Map known string fields.
	for ptr, key := range map[*string]string{
		&c.Sub:      "sub",
		&c.Username: "username",
		&c.Provider: "provider",
		&c.Issuer:   "iss",
	} {
		if v, ok := raw[key].(string); ok {
			*ptr = v
		}
	}
	// If "sub" holds the username (common JWT convention), fall back.
	if c.Username == "" {
		c.Username = c.Sub
	}
	if v, ok := raw["is_admin"].(bool); ok {
		c.IsAdmin = v
	}
	// Numeric fields — JSON numbers decode as float64.
	for ptr, key := range map[*int64]string{
		&c.Exp:      "exp",
		&c.IssuedAt: "iat",
	} {
		if v, ok := raw[key].(float64); ok {
			*ptr = int64(v)
		}
	}
	return c, nil
}
