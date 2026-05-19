package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/aurokin/atlassian-cli/internal/apperr"
	"github.com/aurokin/atlassian-cli/internal/appinfo"
	"github.com/aurokin/atlassian-cli/internal/config"
)

// newAPICommand builds the raw "api" escape-hatch command. It signs and sends
// a request to a configured site and renders the response.
func newAPICommand(info appinfo.Info, g *GlobalFlags) *cobra.Command {
	var (
		method string
		data   string
	)
	cmd := &cobra.Command{
		Use:   "api <path-or-url>",
		Short: "Make a raw authenticated request to the Atlassian API",
		Long: "Send a signed request to the site named by --site. A relative path " +
			"resolves against the product API base; an absolute URL must match the " +
			"configured site or Atlassian API gateway.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAPI(cmd, info, g, args[0], strings.ToUpper(method), data)
		},
	}
	f := cmd.Flags()
	f.StringVarP(&method, "method", "X", "GET", "HTTP method")
	f.StringVar(&data, "data", "", "request body to send")
	return cmd
}

func runAPI(cmd *cobra.Command, info appinfo.Info, g *GlobalFlags, pathOrURL, method, data string) error {
	client, err := SiteClient(info, g)
	if err != nil {
		return err
	}

	var body io.Reader
	if data != "" {
		body = strings.NewReader(data)
	}
	resp, err := client.Do(cmd.Context(), method, pathOrURL, body)
	if err != nil {
		// Surface the server's response body when it is JSON, so the raw api
		// command stays faithful to the underlying API even on failure.
		if resp != nil && json.Valid(bytes.TrimSpace(resp.Body)) {
			_ = Render(cmd, g, json.RawMessage(resp.Body))
		}
		return err
	}
	if len(bytes.TrimSpace(resp.Body)) == 0 {
		return nil
	}
	return Render(cmd, g, json.RawMessage(resp.Body))
}

// loadSiteProfile reads the named site profile from the on-disk config,
// returning a structured error when the config or the profile is absent.
func loadSiteProfile(info appinfo.Info, site string) (config.SiteProfile, error) {
	path, err := config.DefaultPath()
	if err != nil {
		return config.SiteProfile{}, err
	}
	cfg, err := config.Load(path)
	if err != nil {
		return config.SiteProfile{}, err
	}
	profile, ok := cfg.Sites[site]
	if !ok {
		return config.SiteProfile{}, apperr.New("site_not_configured",
			fmt.Sprintf("site %q is not configured; run %s auth login", site, info.Binary))
	}
	return profile, nil
}

// resolveToken reads the token value referenced by a profile. Phase 1
// supports only the "env:" reference form and never logs the value.
func resolveToken(ref string) (string, error) {
	if ref == "" {
		return "", apperr.New("token_unavailable",
			"this site has no token reference; re-run auth login with --token-env")
	}
	name, ok := strings.CutPrefix(ref, tokenRefEnvPrefix)
	if !ok {
		return "", apperr.New("token_unavailable", fmt.Sprintf("unsupported token reference %q", ref))
	}
	v := os.Getenv(name)
	if v == "" {
		return "", apperr.New("token_unavailable", fmt.Sprintf("environment variable %s is not set", name))
	}
	return v, nil
}
