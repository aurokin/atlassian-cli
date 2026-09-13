package cli

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/aurokin/atlassian-cli/internal/apperr"
	"github.com/aurokin/atlassian-cli/internal/appinfo"
	"github.com/aurokin/atlassian-cli/internal/auth"
	"github.com/aurokin/atlassian-cli/internal/httpclient"
)

// traceOut is where --trace diagnostics are written. It is a package variable
// (defaulting to os.Stderr, keeping stdout pure for --json consumers) so tests
// can capture the trace output.
var traceOut io.Writer = os.Stderr

// timeoutEnvVar is the environment variable that sets the per-request HTTP
// timeout when no --timeout flag is given. It sits between the flag and the
// built-in default in precedence.
const timeoutEnvVar = "ATL_TIMEOUT"

// resolveTimeout applies the timeout precedence: an explicit --timeout flag
// wins, then the ATL_TIMEOUT environment variable, then
// httpclient.DefaultTimeout. A zero duration disables the timeout; anything
// that is not a non-negative Go duration is an invalid_input error naming the
// source it came from.
func resolveTimeout(flagTimeout string) (time.Duration, error) {
	raw, source := flagTimeout, "--timeout"
	if raw == "" {
		raw, source = strings.TrimSpace(os.Getenv(timeoutEnvVar)), timeoutEnvVar
	}
	if raw == "" {
		return httpclient.DefaultTimeout, nil
	}
	d, err := time.ParseDuration(raw)
	if err != nil || d < 0 {
		return 0, apperr.InvalidInput(fmt.Sprintf(
			"invalid %s value %q: expected a duration such as 30s or 2m (0 disables the timeout)", source, raw))
	}
	return d, nil
}

// httpClientFor builds the *http.Client every outbound request uses, bounded
// by the resolved --timeout. It validates the flag, so callers run it before
// any config or network work so a bad value is reported as such.
func httpClientFor(g *GlobalFlags) (*http.Client, error) {
	timeout, err := resolveTimeout(g.Timeout)
	if err != nil {
		return nil, err
	}
	// http.Client treats a zero Timeout as unbounded, so --timeout 0 needs no
	// special casing.
	return &http.Client{Timeout: timeout}, nil
}

// SiteClient builds an authenticated HTTP client for the target site. The
// site is resolved by precedence — the --site flag, then the ATL_SITE
// environment variable, then the config's default_site — so most commands need
// no --site at all. It then loads the profile, resolves the token style and
// token value, and returns a ready httpclient.Client bounded by --timeout.
//
// It is the shared entry point for every command that makes a live API call:
// the raw api command and the product command packages all build their client
// through SiteClient rather than duplicating the auth and target wiring.
func SiteClient(info appinfo.Info, g *GlobalFlags) (*httpclient.Client, error) {
	hc, err := httpClientFor(g)
	if err != nil {
		return nil, err
	}
	site, profile, err := loadSiteProfile(info, g.Site)
	if err != nil {
		return nil, err
	}
	style, err := auth.ParseTokenStyle(profile.TokenStyle)
	if err != nil {
		return nil, err
	}
	target := httpclient.Target{
		Product:    string(info.Product),
		TokenStyle: style,
		SiteName:   site,
		BaseURL:    profile.BaseURL,
		CloudID:    profile.CloudID,
	}

	var client *httpclient.Client
	if style == auth.StyleOAuth3LO {
		// oauth-3lo resolves and refreshes its access token per request, so it
		// is wired through a credential provider rather than a fixed token.
		provider, err := oauthCredentialProvider(site, profile, hc)
		if err != nil {
			return nil, err
		}
		client = httpclient.NewWithProvider(target, provider, hc)
	} else {
		token, err := resolveToken(profile.TokenRef, site)
		if err != nil {
			return nil, err
		}
		cred := auth.Credential{
			Style:    style,
			Username: profile.Username,
			Token:    token,
			CloudID:  profile.CloudID,
		}
		client = httpclient.New(target, cred, hc)
	}
	if g.Trace {
		client.EnableTrace(traceOut)
	}
	return client, nil
}
