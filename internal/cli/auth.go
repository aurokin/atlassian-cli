package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"

	"github.com/aurokin/atlassian-cli/internal/apperr"
	"github.com/aurokin/atlassian-cli/internal/appinfo"
	"github.com/aurokin/atlassian-cli/internal/auth"
	"github.com/aurokin/atlassian-cli/internal/config"
	"github.com/aurokin/atlassian-cli/internal/httpclient"
	"github.com/aurokin/atlassian-cli/internal/oauth"
	"github.com/aurokin/atlassian-cli/internal/secrets"
)

// tokenRefEnvPrefix marks a token reference that names an environment
// variable. Phase 1 stores only this reference, never a raw token.
const tokenRefEnvPrefix = "env:"

// defaultCallbackPort is the loopback port the oauth-3lo flow listens on for
// the authorization redirect. It must match the callback registered on the
// OAuth app; Atlassian compares the redirect_uri byte-for-byte, so the port is
// fixed (overridable with --callback-port) rather than ephemeral.
const defaultCallbackPort = 8976

// oauthLoginTimeout bounds how long the loopback flow waits for the user to
// complete consent in the browser.
const oauthLoginTimeout = 5 * time.Minute

// oauthEndpoints is the set of OAuth endpoints the login flow uses. It is a
// package variable so tests can point it at httptest servers; production uses
// the real Atlassian endpoints.
var oauthEndpoints = oauth.DefaultEndpoints()

// oauthNow is the clock used to compute token expiry during login. It is a
// package variable so tests can pin it.
var oauthNow = time.Now

// siteView is the JSON-and-human representation of a configured site. It
// deliberately omits any token value and reports only token availability.
type siteView struct {
	Site        string `json:"site"`
	Product     string `json:"product"`
	Deployment  string `json:"deployment"`
	BaseURL     string `json:"base_url"`
	APIBaseURL  string `json:"api_base_url,omitempty"`
	CloudID     string `json:"cloud_id,omitempty"`
	Username    string `json:"username,omitempty"`
	TokenStyle  string `json:"token_style"`
	AuthType    string `json:"auth_type"`
	TokenRef    string `json:"token_ref,omitempty"`
	TokenStatus string `json:"token_status"`
}

// statusAll wraps a list of site views so it renders as a JSON object,
// keeping --json field selection usable.
type statusAll struct {
	Sites []siteView `json:"sites"`
}

// toView maps a stored profile to a siteView, resolving token availability
// without ever reading the token value into the result.
func toView(name string, p config.SiteProfile) siteView {
	return siteView{
		Site:        name,
		Product:     p.Product,
		Deployment:  p.Deployment,
		BaseURL:     p.BaseURL,
		APIBaseURL:  p.APIBaseURL,
		CloudID:     p.CloudID,
		Username:    p.Username,
		TokenStyle:  p.TokenStyle,
		AuthType:    p.AuthType,
		TokenRef:    p.TokenRef,
		TokenStatus: tokenStatus(name, p),
	}
}

// tokenStatus describes whether the token a profile points at is currently
// resolvable for the named site. It never includes the token value; for an
// oauth-3lo profile it additionally reports the access-token expiry (a
// timestamp, never the token itself).
func tokenStatus(site string, p config.SiteProfile) string {
	ref := p.TokenRef
	if ref == "" {
		return "no token reference configured"
	}
	if name, ok := strings.CutPrefix(ref, tokenRefEnvPrefix); ok {
		if v, present := os.LookupEnv(name); present && v != "" {
			return fmt.Sprintf("token available from environment variable %s", name)
		}
		return fmt.Sprintf("environment variable %s is not set", name)
	}
	credPath, err := config.CredentialsPath()
	if err != nil {
		return "token reference configured"
	}
	store, err := secrets.ForRef(ref, credPath)
	if err != nil {
		return fmt.Sprintf("unrecognized token reference %q", ref)
	}
	value, err := store.Get(site)
	if err != nil {
		var ae *apperr.Error
		if errors.As(err, &ae) && ae.Code != "token_unavailable" {
			return "token reference configured but the stored credential could not be read: " + ae.Message
		}
		return "token reference configured but no stored credential was found"
	}
	var backend string
	switch ref {
	case secrets.BackendKeyring:
		backend = "the OS keychain"
	case secrets.BackendFile:
		backend = "the local credentials file"
	default:
		return "token reference configured"
	}
	if p.TokenStyle == string(auth.StyleOAuth3LO) {
		return oauthTokenStatus(value, backend)
	}
	return "token available from " + backend
}

// oauthTokenStatus reports presence and access-token expiry for an oauth-3lo
// bundle. It parses the stored bundle only to read its expiry timestamp and
// never echoes any token value.
func oauthTokenStatus(value, backend string) string {
	bundle, err := oauth.ParseBundle(value)
	if err != nil {
		return "OAuth token bundle available from " + backend + " but it could not be parsed"
	}
	if bundle.Expiry.IsZero() {
		return "OAuth token available from " + backend + "; access-token expiry unknown"
	}
	exp := bundle.Expiry.UTC().Format(time.RFC3339)
	if bundle.Expired(time.Now()) {
		return fmt.Sprintf("OAuth token available from %s; access token expired at %s", backend, exp)
	}
	return fmt.Sprintf("OAuth token available from %s; access token valid until %s", backend, exp)
}

// newAuthCommand builds the "auth" subtree shared by every atl-* binary.
func newAuthCommand(info appinfo.Info, g *GlobalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Manage authentication site profiles",
	}
	cmd.AddCommand(
		newAuthLoginCommand(info, g),
		newAuthStatusCommand(g),
		newAuthLogoutCommand(info, g),
		newAuthDefaultCommand(info, g),
	)
	return cmd
}

// newAuthDefaultCommand reads or sets the default_site config key, which
// networked commands target when neither --site nor ATL_SITE is given. With no
// argument it prints the current default; with a site name it records that
// site (which must already be configured); with --clear it removes the default.
func newAuthDefaultCommand(info appinfo.Info, g *GlobalFlags) *cobra.Command {
	var clear bool
	cmd := &cobra.Command{
		Use:   "default [site]",
		Short: "Show or set the default site profile",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if clear && len(args) > 0 {
				return apperr.InvalidInput("--clear cannot be combined with a site argument")
			}
			path, err := config.DefaultPath()
			if err != nil {
				return err
			}
			cfg, err := config.Load(path)
			if err != nil {
				return err
			}
			switch {
			case clear:
				cfg.DefaultSite = ""
				if err := config.Save(path, cfg); err != nil {
					return err
				}
				fmt.Fprintln(cmd.OutOrStdout(), "Cleared the default site.")
				return nil
			case len(args) == 1:
				site := args[0]
				if _, ok := cfg.Sites[site]; !ok {
					return apperr.New("site_not_configured",
						fmt.Sprintf("site %q is not configured; run %s auth login", site, info.Binary))
				}
				cfg.DefaultSite = site
				if err := config.Save(path, cfg); err != nil {
					return err
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Set default site to %q.\n", site)
				return nil
			default:
				if g.WantsStructured() {
					return Render(cmd, g, defaultSiteView{DefaultSite: cfg.DefaultSite})
				}
				if cfg.DefaultSite == "" {
					fmt.Fprintln(cmd.OutOrStdout(), "No default site is set.")
					return nil
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Default site: %s\n", cfg.DefaultSite)
				return nil
			}
		},
	}
	cmd.Flags().BoolVar(&clear, "clear", false, "remove the configured default site")
	return cmd
}

// defaultSiteView is the structured shape for `auth default` with no argument.
type defaultSiteView struct {
	DefaultSite string `json:"default_site"`
}

func newAuthLoginCommand(info appinfo.Info, g *GlobalFlags) *cobra.Command {
	var (
		urlFlag           string
		username          string
		tokenStyle        string
		cloudID           string
		tokenEnv          string
		tokenValue        string
		tokenStdin        bool
		clientID          string
		clientSecret      string
		clientSecretStdin bool
		scopes            []string
		callbackPort      int
	)
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Record a site profile for later authenticated requests",
		Long: "Record a site profile under --site.\n\n" +
			"For the token, pass exactly one of:\n" +
			"  --token-env NAME  reference an environment variable (nothing is stored)\n" +
			"  --token-stdin     read the token from stdin and store it securely\n" +
			"  --token VALUE     store the token securely (visible in shell history)\n\n" +
			"A stored token goes to the OS keychain, or to a 0600 file when no " +
			"keychain is available. config.json never holds a raw token.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if g.Site == "" {
				return apperr.InvalidInput("a site name is required; pass --site")
			}
			if urlFlag == "" {
				return apperr.InvalidInput("a site URL is required; pass --url")
			}
			if tokenStyle == "" {
				return apperr.InvalidInput("a token style is required; pass --token-style")
			}
			style, err := auth.ParseTokenStyle(tokenStyle)
			if err != nil {
				return err
			}
			if err := validateSiteURL(urlFlag, style); err != nil {
				return err
			}
			if style == auth.StyleOAuth3LO {
				secret := clientSecret
				if clientSecretStdin {
					raw, err := io.ReadAll(cmd.InOrStdin())
					if err != nil {
						return apperr.InvalidInput("could not read the client secret from standard input: " + err.Error())
					}
					secret = strings.TrimSpace(string(raw))
				}
				return runOAuthLogin(cmd, g, oauthLoginParams{
					info:              info,
					siteURL:           urlFlag,
					clientID:          clientID,
					clientSecret:      secret,
					clientSecretGiven: clientSecret != "" || clientSecretStdin,
					scopes:            scopes,
					callbackPort:      callbackPort,
					cloudID:           cloudID,
				})
			}
			if (style == auth.StyleCloudClassic || style == auth.StyleCloudScoped) && username == "" {
				return apperr.InvalidInput(fmt.Sprintf("token style %s requires --username", style))
			}
			if style == auth.StyleCloudScoped && cloudID == "" {
				return apperr.InvalidInput("token style cloud-scoped requires --cloud-id")
			}
			sources := 0
			for _, set := range []bool{tokenEnv != "", tokenValue != "", tokenStdin} {
				if set {
					sources++
				}
			}
			if sources > 1 {
				return apperr.InvalidInput("pass at most one of --token-env, --token-stdin, or --token")
			}

			profile := config.SiteProfile{
				Product:    string(info.Product),
				Deployment: deploymentFor(style),
				BaseURL:    urlFlag,
				CloudID:    cloudID,
				Username:   username,
				TokenStyle: string(style),
				AuthType:   style.AuthType(),
			}
			switch {
			case tokenEnv != "":
				profile.TokenRef = tokenRefEnvPrefix + tokenEnv
			case tokenValue != "" || tokenStdin:
				token := tokenValue
				if tokenStdin {
					raw, err := io.ReadAll(cmd.InOrStdin())
					if err != nil {
						return apperr.InvalidInput("could not read the token from standard input: " + err.Error())
					}
					token = strings.TrimSpace(string(raw))
				}
				if token == "" {
					return apperr.InvalidInput("the token to store is empty")
				}
				credPath, err := config.CredentialsPath()
				if err != nil {
					return err
				}
				res, err := secrets.Save(credPath, g.Site, token)
				if err != nil {
					return err
				}
				profile.TokenRef = res.Backend
				warnIfTokenNotProtected(cmd.ErrOrStderr(), res, credPath)
			}
			target := httpclient.Target{
				Product:    string(info.Product),
				TokenStyle: style,
				SiteName:   g.Site,
				BaseURL:    urlFlag,
				CloudID:    cloudID,
			}
			base, err := target.APIBase()
			if err != nil {
				return err
			}
			profile.APIBaseURL = base

			return persistProfile(cmd, g, info, profile)
		},
	}
	f := cmd.Flags()
	f.StringVar(&urlFlag, "url", "", "site or instance URL")
	f.StringVar(&username, "username", "", "account email (required for cloud-classic and cloud-scoped)")
	f.StringVar(&tokenStyle, "token-style", "", "token style: cloud-classic, cloud-scoped, data-center-pat, or oauth-3lo")
	f.StringVar(&cloudID, "cloud-id", "", "Atlassian cloud ID (required for cloud-scoped; optional override for oauth-3lo)")
	f.StringVar(&tokenEnv, "token-env", "", "name of the environment variable holding the token")
	f.BoolVar(&tokenStdin, "token-stdin", false, "read the token from stdin and store it securely")
	f.StringVar(&tokenValue, "token", "", "token value to store securely (prefer --token-stdin to keep it out of shell history)")
	f.StringVar(&clientID, "client-id", "", "OAuth app client ID (oauth-3lo)")
	f.StringVar(&clientSecret, "client-secret", "", "OAuth app client secret (oauth-3lo; prefer --client-secret-stdin)")
	f.BoolVar(&clientSecretStdin, "client-secret-stdin", false, "read the OAuth client secret from stdin (oauth-3lo)")
	f.StringSliceVar(&scopes, "scopes", nil, "OAuth scopes to request (oauth-3lo; offline_access is added automatically)")
	f.IntVar(&callbackPort, "callback-port", defaultCallbackPort, "loopback port for the OAuth redirect (oauth-3lo; must match the registered callback)")
	return cmd
}

// warnIfTokenNotProtected prints an accurate warning when a stored credential
// fell back to the 0600 file instead of the OS keychain. It distinguishes a
// keychain that was unavailable from one that rejected the value as too large
// (which is the normal case for oauth-3lo bundles on macOS, where the keychain
// CLI caps the secret size), so the message is never misleading.
func warnIfTokenNotProtected(w io.Writer, res secrets.SaveResult, credPath string) {
	if !res.FellBack {
		return
	}
	if res.TooLargeForKeyring {
		fmt.Fprintf(w,
			"Warning: the OS keychain rejected the credential as too large; stored it in %s "+
				"with 0600 permissions instead. It is not keychain-protected.\n", credPath)
		return
	}
	fmt.Fprintf(w,
		"Warning: no OS keychain is available (%v); stored the credential in %s "+
			"with 0600 permissions instead. It is not keychain-protected.\n",
		res.KeyringErr, credPath)
}

// persistProfile saves profile under the active --site, cleaning up a now-
// unreferenced credential when a re-login switches token backends, and renders
// the result. It is shared by the static-token and oauth-3lo login paths.
func persistProfile(cmd *cobra.Command, g *GlobalFlags, info appinfo.Info, profile config.SiteProfile) error {
	path, err := config.DefaultPath()
	if err != nil {
		return err
	}
	cfg, err := config.Load(path)
	if err != nil {
		return err
	}
	prevRef := cfg.Sites[g.Site].TokenRef
	cfg.Sites[g.Site] = profile
	if err := config.Save(path, cfg); err != nil {
		return err
	}
	// A re-login that switches token backends leaves the previous secret
	// unreferenced; drop it so it does not linger in the keychain or fallback
	// file. Best-effort: a cleanup failure must not fail an otherwise
	// successful login.
	if prevRef != "" && prevRef != profile.TokenRef {
		if err := clearStoredToken(g.Site, prevRef); err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(),
				"Warning: could not remove the previously stored credential (%v).\n", err)
		}
	}

	if g.WantsStructured() {
		return Render(cmd, g, toView(g.Site, profile))
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Saved %s site profile %q (%s).\n", info.Product, g.Site, profile.TokenStyle)
	return nil
}

// oauthLoginParams carries the validated inputs of an oauth-3lo login.
type oauthLoginParams struct {
	info              appinfo.Info
	siteURL           string
	clientID          string
	clientSecret      string
	clientSecretGiven bool
	scopes            []string
	callbackPort      int
	cloudID           string
}

// runOAuthLogin performs the interactive OAuth 2.0 (3LO) authorization-code
// flow: it starts a loopback HTTP server on the fixed callback port, opens the
// browser to the authorize URL, captures the redirect (validating state),
// exchanges the code for a token bundle, resolves the cloud id from the
// authorized sites, and stores the bundle. It needs a browser, so it refuses
// to run under --no-prompt.
func runOAuthLogin(cmd *cobra.Command, g *GlobalFlags, p oauthLoginParams) error {
	if g.NoPrompt {
		return apperr.InvalidInput("oauth-3lo login needs an interactive browser and cannot run with --no-prompt; use an API-token style for non-interactive use")
	}
	if p.clientID == "" {
		return apperr.InvalidInput("token style oauth-3lo requires --client-id")
	}
	if !p.clientSecretGiven {
		return apperr.InvalidInput("token style oauth-3lo requires --client-secret or --client-secret-stdin")
	}
	if p.clientSecret == "" {
		return apperr.InvalidInput("the OAuth client secret is empty")
	}
	if len(p.scopes) == 0 {
		return apperr.InvalidInput("token style oauth-3lo requires --scopes")
	}
	if p.callbackPort < 1 || p.callbackPort > 65535 {
		return apperr.InvalidInput(fmt.Sprintf("--callback-port %d is out of range (1-65535)", p.callbackPort))
	}

	scopes := oauth.EnsureOfflineAccess(p.scopes)
	redirectURI := fmt.Sprintf("http://localhost:%d/callback", p.callbackPort)

	state, err := oauth.GenerateState()
	if err != nil {
		return err
	}
	pkce, err := oauth.GeneratePKCE()
	if err != nil {
		return err
	}

	listeners, err := listenLoopback(p.callbackPort)
	if err != nil {
		return err
	}
	resultCh := make(chan oauthCallbackResult, 1)
	mux := http.NewServeMux()
	mux.HandleFunc("/callback", oauthCallbackHandler(state, resultCh))
	srv := &http.Server{Handler: mux}
	for _, l := range listeners {
		go func(l net.Listener) { _ = srv.Serve(l) }(l)
	}
	defer func() { _ = srv.Close() }()

	client := oauth.New(p.clientID, p.clientSecret, oauth.Options{Endpoints: oauthEndpoints, Now: oauthNow})
	authURL := client.AuthorizeURL(oauth.AuthorizeParams{
		RedirectURI:   redirectURI,
		Scopes:        scopes,
		State:         state,
		CodeChallenge: pkce.Challenge,
	})

	fmt.Fprintf(cmd.ErrOrStderr(),
		"Opening your browser to authorize. If it does not open, visit:\n%s\n", authURL)
	if err := browseOpener(authURL); err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(),
			"Warning: could not open a browser automatically (%v). Open the URL above to continue.\n", err)
	}

	ctx, cancel := context.WithTimeout(cmd.Context(), oauthLoginTimeout)
	defer cancel()

	var code string
	select {
	case res := <-resultCh:
		if res.err != nil {
			return res.err
		}
		code = res.code
	case <-ctx.Done():
		return apperr.New("oauth_timeout", "timed out waiting for the authorization callback in the browser")
	}

	bundle, err := client.Exchange(ctx, code, pkce.Verifier, redirectURI)
	if err != nil {
		return err
	}
	resources, err := client.AccessibleResources(ctx, bundle.AccessToken)
	if err != nil {
		return err
	}
	cloudID, err := resolveCloudID(resources, p.siteURL, p.cloudID)
	if err != nil {
		return err
	}

	// The client secret is stored with the tokens in the keychain bundle, never
	// in config.json.
	bundle.ClientSecret = p.clientSecret
	value, err := bundle.Marshal()
	if err != nil {
		return err
	}
	credPath, err := config.CredentialsPath()
	if err != nil {
		return err
	}
	saved, err := secrets.Save(credPath, g.Site, value)
	if err != nil {
		return err
	}
	warnIfTokenNotProtected(cmd.ErrOrStderr(), saved, credPath)

	profile := config.SiteProfile{
		Product:    string(p.info.Product),
		Deployment: "cloud",
		BaseURL:    p.siteURL,
		CloudID:    cloudID,
		TokenStyle: string(auth.StyleOAuth3LO),
		AuthType:   auth.StyleOAuth3LO.AuthType(),
		TokenRef:   saved.Backend,
		ClientID:   p.clientID,
		Scopes:     scopes,
	}
	target := httpclient.Target{
		Product:    string(p.info.Product),
		TokenStyle: auth.StyleOAuth3LO,
		SiteName:   g.Site,
		BaseURL:    p.siteURL,
		CloudID:    cloudID,
	}
	base, err := target.APIBase()
	if err != nil {
		return err
	}
	profile.APIBaseURL = base

	return persistProfile(cmd, g, p.info, profile)
}

// oauthCallbackResult is the outcome the loopback handler sends back: an
// authorization code on success, or a structured error.
type oauthCallbackResult struct {
	code string
	err  error
}

// oauthCallbackHandler builds the /callback handler. It validates the state
// parameter (rejecting a missing or mismatched value), surfaces an OAuth error
// returned in the redirect, and otherwise captures the authorization code. It
// writes a short page to the browser before signaling so the response is
// flushed before the server is torn down.
func oauthCallbackHandler(wantState string, resultCh chan<- oauthCallbackResult) http.HandlerFunc {
	var once sync.Once
	send := func(res oauthCallbackResult) { once.Do(func() { resultCh <- res }) }
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if oauthErr := q.Get("error"); oauthErr != "" {
			msg := oauthErr
			if d := q.Get("error_description"); d != "" {
				msg += ": " + d
			}
			writeCallbackPage(w, "Authorization failed", "You can close this tab and return to the terminal.")
			send(oauthCallbackResult{err: apperr.New("oauth_authorization_failed", "the authorization was denied or failed: "+msg)})
			return
		}
		if got := q.Get("state"); got == "" || got != wantState {
			writeCallbackPage(w, "Authorization failed", "The state parameter did not match. You can close this tab.")
			send(oauthCallbackResult{err: apperr.New("oauth_state_mismatch", "the authorization callback had a missing or mismatched state parameter")})
			return
		}
		code := q.Get("code")
		if code == "" {
			writeCallbackPage(w, "Authorization failed", "No authorization code was returned. You can close this tab.")
			send(oauthCallbackResult{err: apperr.New("oauth_no_code", "the authorization callback did not include a code")})
			return
		}
		writeCallbackPage(w, "Authorized", "Authorization complete. You can close this tab and return to the terminal.")
		send(oauthCallbackResult{code: code})
	}
}

// writeCallbackPage writes a minimal HTML page to the browser. It carries no
// token values. It flushes the response before returning so the page reaches
// the browser before the caller signals completion and tears the server down.
func writeCallbackPage(w http.ResponseWriter, title, body string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, "<!doctype html><html><head><title>%s</title></head><body><h1>%s</h1><p>%s</p></body></html>", title, title, body)
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
}

// listenLoopback binds the fixed callback port on the loopback interface. The
// registered redirect host is the literal "localhost", which resolves to
// 127.0.0.1 on some systems and ::1 on others, so it binds both loopback
// families when available and succeeds as long as at least one binds.
func listenLoopback(port int) ([]net.Listener, error) {
	var listeners []net.Listener
	var firstErr error
	for _, host := range []string{"127.0.0.1", "::1"} {
		l, err := net.Listen("tcp", net.JoinHostPort(host, strconv.Itoa(port)))
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		listeners = append(listeners, l)
	}
	if len(listeners) == 0 {
		return nil, apperr.New("oauth_callback_listen_failed",
			fmt.Sprintf("could not listen on loopback port %d for the OAuth callback: %v", port, firstErr))
	}
	return listeners, nil
}

// resolveCloudID picks the cloud id for the configured site URL from the sites
// the authorization grants access to. An explicit --cloud-id override is used
// when it matches one of those sites; otherwise the site URL host is matched
// (case-insensitively) and a zero or non-unique match is a structured error
// telling the user to pass --cloud-id.
func resolveCloudID(resources []oauth.Resource, siteURL, override string) (string, error) {
	if override != "" {
		for _, r := range resources {
			if r.ID == override {
				return override, nil
			}
		}
		return "", apperr.New("oauth_cloud_id_unmatched",
			fmt.Sprintf("--cloud-id %q is not among the sites this authorization grants access to", override))
	}
	want, err := normalizeHost(siteURL)
	if err != nil {
		return "", err
	}
	var matches []string
	for _, r := range resources {
		h, err := normalizeHost(r.URL)
		if err != nil {
			continue
		}
		if h == want {
			matches = append(matches, r.ID)
		}
	}
	switch len(matches) {
	case 1:
		return matches[0], nil
	case 0:
		return "", apperr.New("oauth_no_matching_site",
			fmt.Sprintf("none of the authorized sites match %s; re-run with --cloud-id to choose one", siteURL))
	default:
		return "", apperr.New("oauth_ambiguous_site",
			fmt.Sprintf("more than one authorized site matches %s; re-run with --cloud-id to choose one", siteURL))
	}
}

// normalizeHost returns the lower-cased host of a URL for comparison.
func normalizeHost(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return "", apperr.InvalidInput(fmt.Sprintf("invalid site URL %q", raw))
	}
	return strings.ToLower(u.Host), nil
}

func newAuthStatusCommand(g *GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show configured site profiles and token availability",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			path, err := config.DefaultPath()
			if err != nil {
				return err
			}
			cfg, err := config.Load(path)
			if err != nil {
				return err
			}
			if g.Site != "" {
				p, ok := cfg.Sites[g.Site]
				if !ok {
					return apperr.New("site_not_configured", fmt.Sprintf("site %q is not configured", g.Site))
				}
				return Render(cmd, g, toView(g.Site, p))
			}
			names := make([]string, 0, len(cfg.Sites))
			for name := range cfg.Sites {
				names = append(names, name)
			}
			sort.Strings(names)
			views := make([]siteView, 0, len(names))
			for _, name := range names {
				views = append(views, toView(name, cfg.Sites[name]))
			}
			return Render(cmd, g, statusAll{Sites: views})
		},
	}
}

func newAuthLogoutCommand(info appinfo.Info, g *GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "Remove a configured site profile",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if g.Site == "" {
				return apperr.InvalidInput("a site name is required; pass --site")
			}
			path, err := config.DefaultPath()
			if err != nil {
				return err
			}
			cfg, err := config.Load(path)
			if err != nil {
				return err
			}
			profile, ok := cfg.Sites[g.Site]
			if !ok {
				return apperr.New("site_not_configured", fmt.Sprintf("site %q is not configured", g.Site))
			}
			if err := clearStoredToken(g.Site, profile.TokenRef); err != nil {
				return err
			}
			delete(cfg.Sites, g.Site)
			// Drop a now-dangling default rather than leave it pointing at a
			// profile that no longer exists.
			if cfg.DefaultSite == g.Site {
				cfg.DefaultSite = ""
			}
			if err := config.Save(path, cfg); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Removed %s site profile %q.\n", info.Product, g.Site)
			return nil
		},
	}
}

// clearStoredToken removes the credential a profile's token_ref points at.
// "env:" and empty references hold nothing on disk to clear; only the keyring
// and file backends have a stored secret to delete.
func clearStoredToken(site, ref string) error {
	switch ref {
	case secrets.BackendKeyring, secrets.BackendFile:
		credPath, err := config.CredentialsPath()
		if err != nil {
			return err
		}
		store, err := secrets.ForRef(ref, credPath)
		if err != nil {
			return err
		}
		return store.Delete(site)
	default:
		return nil
	}
}

// deploymentFor maps a token style to the deployment label stored in config.
func deploymentFor(style auth.TokenStyle) string {
	if style == auth.StyleDataCenterPAT {
		return "data-center"
	}
	return "cloud"
}

// validateSiteURL rejects site URLs that are unsafe to store: a missing host,
// a non-http(s) scheme, embedded credentials (the token is supplied
// separately), and a non-https scheme for cloud token styles, since Atlassian
// Cloud is always reached over https.
func validateSiteURL(raw string, style auth.TokenStyle) error {
	u, err := url.Parse(raw)
	if err != nil {
		return apperr.InvalidInput(fmt.Sprintf("invalid --url %q: %v", raw, err))
	}
	if u.Host == "" {
		return apperr.InvalidInput(fmt.Sprintf("--url %q must include a host", raw))
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return apperr.InvalidInput(fmt.Sprintf("--url scheme %q must be http or https", u.Scheme))
	}
	if u.User != nil {
		return apperr.InvalidInput("--url must not embed credentials; supply the token with --token-env")
	}
	if u.Scheme != "https" && style != auth.StyleDataCenterPAT {
		return apperr.InvalidInput(fmt.Sprintf("token style %s requires an https --url", style))
	}
	return nil
}
