// Package auth manages token storage in the OS keyring and JWT utilities.
package auth

import (
	"errors"
	"fmt"

	"github.com/zalando/go-keyring"
)

const service = "finna-cli"

// keyFor returns the keyring key for a named context.
func keyFor(contextName string) string {
	return fmt.Sprintf("finna-cli:%s", contextName)
}

// ErrNoToken is returned when no token is stored for the requested context.
var ErrNoToken = errors.New("no token stored for context (run `finna login`)")

// Get retrieves the JWT for the named context from the OS keychain.
// Returns ErrNoToken if nothing is stored.
func Get(contextName string) (string, error) {
	tok, err := keyring.Get(service, keyFor(contextName))
	if err != nil {
		if errors.Is(err, keyring.ErrNotFound) {
			return "", ErrNoToken
		}
		return "", fmt.Errorf("keyring get: %w", err)
	}
	return tok, nil
}

// Set stores jwt for the named context in the OS keychain.
func Set(contextName, jwt string) error {
	if err := keyring.Set(service, keyFor(contextName), jwt); err != nil {
		return fmt.Errorf("keyring set: %w", err)
	}
	return nil
}

// Delete removes the token for the named context. It is not an error if no
// token was stored.
func Delete(contextName string) error {
	if err := keyring.Delete(service, keyFor(contextName)); err != nil {
		if errors.Is(err, keyring.ErrNotFound) {
			return nil
		}
		return fmt.Errorf("keyring delete: %w", err)
	}
	return nil
}

// DeleteAll removes tokens for every context in the provided list.
// It continues past individual errors, collecting them all.
func DeleteAll(contextNames []string) error {
	var errs []error
	for _, name := range contextNames {
		if err := Delete(name); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

// TokenProvider returns a TokenProvider function suitable for api.New that
// reads from the keyring. A missing token returns ("", nil) so unauthenticated
// callers get a clean 401 rather than a startup failure.
func TokenProvider(contextName string) func() (string, error) {
	return func() (string, error) {
		tok, err := Get(contextName)
		if err != nil {
			if errors.Is(err, ErrNoToken) {
				return "", nil
			}
			return "", err
		}
		return tok, nil
	}
}
