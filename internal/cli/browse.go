package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/aurokin/atlassian-cli/internal/appinfo"
	"github.com/aurokin/atlassian-cli/internal/browser"
	"github.com/aurokin/atlassian-cli/internal/resolve"
)

// browseOpener opens a URL in a browser. It is a package variable so tests
// can substitute a recorder for the real browser.Open.
var browseOpener = browser.Open

// browseResult is the JSON representation of a browse target. Wrapping the URL
// in an object keeps --json field selection usable, as for every other command.
type browseResult struct {
	URL string `json:"url"`
}

// newBrowseCommand builds the "browse" subcommand: it resolves a URL or bare
// key/id, builds the canonical browser URL, and opens it.
func newBrowseCommand(info appinfo.Info, g *GlobalFlags) *cobra.Command {
	var noBrowser bool
	cmd := &cobra.Command{
		Use:   "browse <url-or-key>",
		Short: "Open a resolved resource in a browser",
		Long: "Resolve a URL or a bare key/id, build its canonical browser URL, and " +
			"open it. A bare key/id needs --site to supply the site root; a full " +
			"URL carries its own. With --no-browser, or the global --no-prompt, the " +
			"URL is printed instead of opened.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runBrowse(cmd, info, g, args[0], noBrowser)
		},
	}
	cmd.Flags().BoolVar(&noBrowser, "no-browser", false, "print the URL instead of opening a browser")
	return cmd
}

func runBrowse(cmd *cobra.Command, info appinfo.Info, g *GlobalFlags, input string, noBrowser bool) error {
	parser, err := resolve.ParserFor(string(info.Product))
	if err != nil {
		return err
	}
	r, err := resolve.Resolve(parser, input)
	if err != nil {
		return err
	}
	base, err := browseBaseURL(info, g, r)
	if err != nil {
		return err
	}
	canonical, err := parser.CanonicalURL(base, r)
	if err != nil {
		return err
	}

	// --no-browser and the global --no-prompt both force print-only, keeping
	// the command safe in non-interactive and agent contexts.
	if noBrowser || g.NoPrompt {
		if g.WantsStructured() {
			return Render(cmd, g, browseResult{URL: canonical})
		}
		fmt.Fprintln(cmd.OutOrStdout(), canonical)
		return nil
	}
	if err := browseOpener(canonical); err != nil {
		return err
	}
	fmt.Fprintln(cmd.ErrOrStderr(), "Opened", canonical)
	return nil
}

// browseBaseURL determines the site root for the canonical URL. A URL input
// carries its own host; a bare key/id needs the --site profile's base URL.
func browseBaseURL(info appinfo.Info, g *GlobalFlags, r resolve.Resource) (string, error) {
	if r.SiteHost != "" {
		// A URL input records only its host. Phase 2 targets Atlassian Cloud,
		// which is always https, so the canonical URL is rooted at https even
		// if the input used http. Data Center URL shapes are a Phase 2 non-goal.
		return "https://" + r.SiteHost, nil
	}
	_, profile, err := loadSiteProfile(info, g.Site)
	if err != nil {
		return "", err
	}
	return profile.BaseURL, nil
}
