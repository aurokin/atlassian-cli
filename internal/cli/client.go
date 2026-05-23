package cli

import (
	"io"
	"os"

	"github.com/aurokin/atlassian-cli/internal/apperr"
	"github.com/aurokin/atlassian-cli/internal/appinfo"
	"github.com/aurokin/atlassian-cli/internal/auth"
	"github.com/aurokin/atlassian-cli/internal/httpclient"
)

// traceOut is where --trace diagnostics are written. It is a package variable
// (defaulting to os.Stderr, keeping stdout pure for --json consumers) so tests
// can capture the trace output.
var traceOut io.Writer = os.Stderr

// SiteClient builds an authenticated HTTP client for the profile named by the
// global --site flag. It enforces that --site is set, loads the profile,
// resolves the token style and token value, and returns a ready
// httpclient.Client.
//
// It is the shared entry point for every command that makes a live API call:
// the raw api command and the product command packages all build their client
// through SiteClient rather than duplicating the auth and target wiring.
func SiteClient(info appinfo.Info, g *GlobalFlags) (*httpclient.Client, error) {
	if g.Site == "" {
		return nil, apperr.InvalidInput("a site name is required; pass --site")
	}
	profile, err := loadSiteProfile(info, g.Site)
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
		SiteName:   g.Site,
		BaseURL:    profile.BaseURL,
		CloudID:    profile.CloudID,
	}

	var client *httpclient.Client
	if style == auth.StyleOAuth3LO {
		// oauth-3lo resolves and refreshes its access token per request, so it
		// is wired through a credential provider rather than a fixed token.
		provider, err := oauthCredentialProvider(g.Site, profile)
		if err != nil {
			return nil, err
		}
		client = httpclient.NewWithProvider(target, provider, nil)
	} else {
		token, err := resolveToken(profile.TokenRef, g.Site)
		if err != nil {
			return nil, err
		}
		cred := auth.Credential{
			Style:    style,
			Username: profile.Username,
			Token:    token,
			CloudID:  profile.CloudID,
		}
		client = httpclient.New(target, cred, nil)
	}
	if g.Trace {
		client.EnableTrace(traceOut)
	}
	return client, nil
}
