package cli

import (
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"sort"
	"strings"
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
	)
	return cmd
}

func newAuthLoginCommand(info appinfo.Info, g *GlobalFlags) *cobra.Command {
	var (
		urlFlag    string
		username   string
		tokenStyle string
		cloudID    string
		tokenEnv   string
		tokenValue string
		tokenStdin bool
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
				if res.FellBack {
					fmt.Fprintf(cmd.ErrOrStderr(),
						"Warning: no OS keychain is available (%v); stored the token in %s "+
							"with 0600 permissions instead. It is not keychain-protected.\n",
						res.KeyringErr, credPath)
				}
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
			// A re-login that switches token backends leaves the previous
			// secret unreferenced; drop it so it does not linger in the
			// keychain or fallback file. Best-effort: a cleanup failure must
			// not fail an otherwise successful login.
			if prevRef != "" && prevRef != profile.TokenRef {
				if err := clearStoredToken(g.Site, prevRef); err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(),
						"Warning: could not remove the previously stored credential (%v).\n", err)
				}
			}

			if g.JSON != "" {
				return Render(cmd, g, toView(g.Site, profile))
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Saved %s site profile %q (%s).\n", info.Product, g.Site, style)
			return nil
		},
	}
	f := cmd.Flags()
	f.StringVar(&urlFlag, "url", "", "site or instance URL")
	f.StringVar(&username, "username", "", "account email (required for cloud-classic and cloud-scoped)")
	f.StringVar(&tokenStyle, "token-style", "", "token style: cloud-classic, cloud-scoped, or data-center-pat")
	f.StringVar(&cloudID, "cloud-id", "", "Atlassian cloud ID (required for cloud-scoped)")
	f.StringVar(&tokenEnv, "token-env", "", "name of the environment variable holding the token")
	f.BoolVar(&tokenStdin, "token-stdin", false, "read the token from stdin and store it securely")
	f.StringVar(&tokenValue, "token", "", "token value to store securely (prefer --token-stdin to keep it out of shell history)")
	return cmd
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
