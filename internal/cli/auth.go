package cli

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/aurokin/atlassian-cli/internal/appinfo"
	"github.com/aurokin/atlassian-cli/internal/apperr"
	"github.com/aurokin/atlassian-cli/internal/auth"
	"github.com/aurokin/atlassian-cli/internal/config"
	"github.com/aurokin/atlassian-cli/internal/httpclient"
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
		TokenStatus: tokenStatus(p.TokenRef),
	}
}

// tokenStatus describes whether a referenced token is currently resolvable.
// It never includes the token value.
func tokenStatus(ref string) string {
	if ref == "" {
		return "no token reference configured"
	}
	if name, ok := strings.CutPrefix(ref, tokenRefEnvPrefix); ok {
		if v, present := os.LookupEnv(name); present && v != "" {
			return fmt.Sprintf("token available from environment variable %s", name)
		}
		return fmt.Sprintf("environment variable %s is not set", name)
	}
	return "token reference configured"
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
	)
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Record a site profile for later authenticated requests",
		Long: "Record a site profile under --site. No raw token is stored: pass " +
			"--token-env to reference an environment variable holding the token.",
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
			if (style == auth.StyleCloudClassic || style == auth.StyleCloudScoped) && username == "" {
				return apperr.InvalidInput(fmt.Sprintf("token style %s requires --username", style))
			}
			if style == auth.StyleCloudScoped && cloudID == "" {
				return apperr.InvalidInput("token style cloud-scoped requires --cloud-id")
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
			if tokenEnv != "" {
				profile.TokenRef = tokenRefEnvPrefix + tokenEnv
			}
			target := httpclient.Target{
				Product:    string(info.Product),
				TokenStyle: style,
				SiteName:   g.Site,
				BaseURL:    urlFlag,
				CloudID:    cloudID,
			}
			if base, err := target.APIBase(); err == nil {
				profile.APIBaseURL = base
			}

			path, err := config.DefaultPath()
			if err != nil {
				return err
			}
			cfg, err := config.Load(path)
			if err != nil {
				return err
			}
			cfg.Sites[g.Site] = profile
			if err := config.Save(path, cfg); err != nil {
				return err
			}

			if g.JSON != "" {
				return render(cmd, g, toView(g.Site, profile))
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
				return render(cmd, g, toView(g.Site, p))
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
			return render(cmd, g, statusAll{Sites: views})
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
			if _, ok := cfg.Sites[g.Site]; !ok {
				return apperr.New("site_not_configured", fmt.Sprintf("site %q is not configured", g.Site))
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

// deploymentFor maps a token style to the deployment label stored in config.
func deploymentFor(style auth.TokenStyle) string {
	if style == auth.StyleDataCenterPAT {
		return "data-center"
	}
	return "cloud"
}
