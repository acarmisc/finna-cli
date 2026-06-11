package cli

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	finnaapi "github.com/acarmisc/finna-cli/internal/api"
	"github.com/acarmisc/finna-cli/internal/auth"
	"github.com/acarmisc/finna-cli/internal/ui"
)

func newLoginCmd() *cobra.Command {
	var (
		tokenFlag  string
		githubFlag bool
		oidcFlag   string
	)

	cmd := &cobra.Command{
		Use:   "login",
		Short: "Authenticate and store a token in the OS keyring",
		Long: `Authenticate against the Finna backend and store the JWT in the OS keyring.

Default: prompts for username and password (requires a TTY).
--token <jwt>: store a pre-obtained JWT directly.
--github: start GitHub OAuth flow (opens browser).
--oidc <provider-id>: start PKCE OIDC flow.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runLogin(cmd, tokenFlag, githubFlag, oidcFlag)
		},
	}

	cmd.Flags().StringVar(&tokenFlag, "token", "", "store a pre-obtained JWT without prompting")
	cmd.Flags().BoolVar(&githubFlag, "github", false, "log in via GitHub OAuth")
	cmd.Flags().StringVar(&oidcFlag, "oidc", "", "log in via OIDC provider ID")
	return cmd
}

func runLogin(cmd *cobra.Command, tokenFlag string, githubFlag bool, oidcFlag string) error {
	st := State()
	ctxName := st.Effective.ContextName
	if ctxName == "" {
		return errors.New("no context selected — run `finna context add` first")
	}
	server := st.Effective.Server
	if server == "" {
		return errors.New("no server configured for current context")
	}

	// --token: store directly, no API call.
	if tokenFlag != "" {
		if err := auth.Set(ctxName, tokenFlag); err != nil {
			return fmt.Errorf("store token: %w", err)
		}
		// Decode to show who we are.
		if c, err := auth.DecodeJWT(tokenFlag); err == nil {
			fmt.Fprintf(cmd.OutOrStdout(), "logged in as %s (context: %s)\n", c.Username, ctxName)
		} else {
			fmt.Fprintf(cmd.OutOrStdout(), "token stored for context %q\n", ctxName)
		}
		return nil
	}

	// Build an unauthenticated API client (no token yet).
	apiClient := finnaapi.New(server, nil, buildClientOpts(st)...)

	switch {
	case githubFlag:
		return loginGitHub(cmd, apiClient, ctxName)
	case oidcFlag != "":
		return loginOIDC(cmd, apiClient, ctxName, oidcFlag)
	default:
		return loginPassword(cmd, apiClient, ctxName)
	}
}

// loginPassword prompts for username/password (requires TTY) and calls
// POST /api/v1/auth/login.
func loginPassword(cmd *cobra.Command, apiClient *finnaapi.Client, ctxName string) error {
	if !isInteractive() {
		return errors.New("not a TTY — use `finna login --token <jwt>` or set FINNA_TOKEN")
	}
	var username, password string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Username").
				Value(&username).
				Validate(func(s string) error {
					if s == "" {
						return errors.New("required")
					}
					return nil
				}),
			huh.NewInput().
				Title("Password").
				EchoMode(huh.EchoModePassword).
				Value(&password).
				Validate(func(s string) error {
					if s == "" {
						return errors.New("required")
					}
					return nil
				}),
		),
	)
	if err := form.Run(); err != nil {
		return fmt.Errorf("login cancelled: %w", err)
	}
	resp, err := apiClient.Login(cmd.Context(), finnaapi.LoginRequest{
		Username: username,
		Password: password,
	})
	if err != nil {
		ui.FormatAPIError(cmd.ErrOrStderr(), err)
		return fmt.Errorf("login failed")
	}
	tok := resp.AccessTokenValue()
	if tok == "" {
		return errors.New("server returned empty token")
	}
	if err := auth.Set(ctxName, tok); err != nil {
		return fmt.Errorf("store token: %w", err)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "logged in as %s (context: %s)\n", username, ctxName)
	return nil
}

// loginGitHub fetches the GitHub OAuth URL, opens the browser, then waits for
// the user to paste the callback code.
func loginGitHub(cmd *cobra.Command, apiClient *finnaapi.Client, ctxName string) error {
	redir, err := apiClient.GitHubRedirectURL(cmd.Context())
	if err != nil {
		return fmt.Errorf("get github auth URL: %w", err)
	}
	if redir.URL == "" {
		return errors.New("server returned empty GitHub URL — is GITHUB_CLIENT_ID configured?")
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Open this URL in your browser:\n\n  %s\n\n", redir.URL)
	fmt.Fprint(cmd.OutOrStdout(), "After authorizing, GitHub will redirect you. Paste the `code` parameter here: ")

	var code string
	if _, err := fmt.Fscan(cmd.InOrStdin(), &code); err != nil || code == "" {
		return errors.New("no code provided")
	}

	cbResp, err := apiClient.GitHubCallback(cmd.Context(), code)
	if err != nil {
		ui.FormatAPIError(cmd.ErrOrStderr(), err)
		return fmt.Errorf("github callback failed")
	}
	if cbResp.Token == "" {
		return errors.New("server returned empty token")
	}
	if err := auth.Set(ctxName, cbResp.Token); err != nil {
		return fmt.Errorf("store token: %w", err)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "GitHub login successful (context: %s)\n", ctxName)
	return nil
}

// loginOIDC runs the PKCE OIDC flow: open authorization URL in browser,
// listen on a loopback callback server, POST the code to /api/v1/auth/oidc/callback.
func loginOIDC(cmd *cobra.Command, apiClient *finnaapi.Client, ctxName, providerID string) error {
	loginResp, err := apiClient.OIDCLogin(cmd.Context(), providerID)
	if err != nil {
		return fmt.Errorf("initiate OIDC login: %w", err)
	}

	// Start a one-shot loopback HTTP server to receive the callback.
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("start callback listener: %w", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port

	// Generate a random nonce and embed it in the callback path so that
	// other local processes cannot forge a callback request to this port.
	var nonce [16]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return fmt.Errorf("generate callback nonce: %w", err)
	}
	callbackPath := "/callback/" + hex.EncodeToString(nonce[:])
	callbackURL := fmt.Sprintf("http://127.0.0.1:%d%s", port, callbackPath)

	type result struct {
		code string
		err  error
	}
	ch := make(chan result, 1)

	srv := &http.Server{
		ReadHeaderTimeout: 10 * time.Second,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != callbackPath {
				http.NotFound(w, r)
				return
			}
			code := r.URL.Query().Get("code")
			errParam := r.URL.Query().Get("error")
			if errParam != "" {
				http.Error(w, "Login failed: "+errParam, http.StatusBadRequest)
				ch <- result{err: fmt.Errorf("OIDC error: %s", errParam)}
				return
			}
			if code == "" {
				http.Error(w, "No code in callback", http.StatusBadRequest)
				ch <- result{err: errors.New("no code in OIDC callback")}
				return
			}
			fmt.Fprint(w, "<html><body>Login successful! You may close this tab.</body></html>")
			ch <- result{code: code}
		}),
	}

	go func() { _ = srv.Serve(listener) }()
	defer func() {
		shutCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutCtx)
	}()

	// Build auth URL with redirect_uri appended.
	authURL := appendRedirectURI(loginResp.AuthorizationURL, callbackURL)
	fmt.Fprintf(cmd.OutOrStdout(), "Opening OIDC authorization URL:\n\n  %s\n\n", authURL)
	fmt.Fprintf(cmd.OutOrStdout(), "Waiting for callback on %s ...\n", callbackURL)
	_ = openBrowser(authURL) // best-effort

	ctx := cmd.Context()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case res := <-ch:
		if res.err != nil {
			return res.err
		}
		cbResp, err := apiClient.OIDCCallback(ctx, finnaapi.OIDCCallbackRequest{
			ProviderID: providerID,
			Code:       res.code,
			State:      loginResp.State,
		})
		if err != nil {
			ui.FormatAPIError(cmd.ErrOrStderr(), err)
			return fmt.Errorf("OIDC callback failed")
		}
		if cbResp.Token == "" {
			return errors.New("server returned empty token")
		}
		if err := auth.Set(ctxName, cbResp.Token); err != nil {
			return fmt.Errorf("store token: %w", err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "logged in as %s (context: %s)\n", cbResp.Username, ctxName)
		return nil
	}
}

// appendRedirectURI appends redirect_uri to a URL, preserving existing query
// parameters. If the URL can't be parsed it is returned unchanged.
func appendRedirectURI(authURL, redirectURI string) string {
	u, err := url.Parse(authURL)
	if err != nil {
		return authURL + "&redirect_uri=" + url.QueryEscape(redirectURI)
	}
	q := u.Query()
	q.Set("redirect_uri", redirectURI)
	u.RawQuery = q.Encode()
	return u.String()
}

// openBrowser attempts to open a URL in the default browser. Best-effort.
func openBrowser(rawURL string) error {
	return openBrowserPlatform(rawURL)
}

// buildClientOpts returns api.Options derived from AppState for use in auth
// commands that construct their own temporary client.
func buildClientOpts(st *AppState) []finnaapi.Option {
	var opts []finnaapi.Option
	if st.Flags.Debug {
		opts = append(opts, finnaapi.WithDebug(os.Stderr))
	}
	return opts
}
