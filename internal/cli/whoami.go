package cli

import (
	"errors"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/acarmisc/finna-cli/internal/auth"
)

func newWhoamiCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "whoami",
		Short: "Show the identity encoded in the stored JWT",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runWhoami(cmd)
		},
	}
}

func runWhoami(cmd *cobra.Command) error {
	st := State()
	ctxName := st.Effective.ContextName
	if ctxName == "" {
		return errors.New("no context selected")
	}
	tok, err := auth.Get(ctxName)
	if err != nil {
		return err
	}
	claims, err := auth.DecodeJWT(tok)
	if err != nil {
		return fmt.Errorf("decode token: %w", err)
	}

	w := cmd.OutOrStdout()
	fmt.Fprintf(w, "context:  %s\n", ctxName)
	fmt.Fprintf(w, "username: %s\n", claims.Username)
	if claims.Sub != "" && claims.Sub != claims.Username {
		fmt.Fprintf(w, "sub:      %s\n", claims.Sub)
	}
	if claims.Provider != "" {
		fmt.Fprintf(w, "provider: %s\n", claims.Provider)
	}
	if claims.Issuer != "" {
		fmt.Fprintf(w, "issuer:   %s\n", claims.Issuer)
	}
	if claims.IsAdmin {
		fmt.Fprintf(w, "admin:    yes\n")
	}
	exp := claims.ExpiresAt()
	if !exp.IsZero() {
		fmt.Fprintf(w, "expires:  %s", exp.UTC().Format(time.RFC3339))
		remaining := time.Until(exp)
		switch {
		case claims.Expired():
			fmt.Fprint(w, " (EXPIRED)")
		case remaining < 24*time.Hour:
			fmt.Fprintf(w, " (expires in %s)", remaining.Round(time.Minute))
		}
		fmt.Fprintln(w)
	}
	if claims.ExpiresSoon(24 * time.Hour) && !claims.Expired() {
		fmt.Fprintf(cmd.ErrOrStderr(), "warning: token expires in less than 24 hours — run `finna login` to refresh\n")
	}
	if claims.Expired() {
		fmt.Fprintf(cmd.ErrOrStderr(), "warning: token has expired — run `finna login`\n")
	}
	return nil
}
