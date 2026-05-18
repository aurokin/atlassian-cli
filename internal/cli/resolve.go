package cli

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/aurokin/atlassian-cli/internal/appinfo"
	"github.com/aurokin/atlassian-cli/internal/resolve"
)

// newResolveCommand builds the "resolve" subcommand: it turns a URL or a bare
// key/id into a structured Resource. Resolution is pure offline string
// parsing — no network call is made.
func newResolveCommand(info appinfo.Info, g *GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "resolve <url-or-key>",
		Short: "Resolve a URL or key into a structured resource",
		Long: "Parse an Atlassian URL or a bare key/id into a structured resource. " +
			"Resolution is offline string parsing; an input that matches no known " +
			"form fails with a structured 'unresolved' error.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			parser, err := resolve.ParserFor(string(info.Product))
			if err != nil {
				return err
			}
			r, err := resolve.Resolve(parser, args[0])
			if err != nil {
				return err
			}
			if g.JSON != "" || g.JQ != "" {
				return render(cmd, g, r)
			}
			writeResourceHuman(cmd.OutOrStdout(), r)
			return nil
		},
	}
}

// writeResourceHuman prints a resolved Resource as aligned label/value lines,
// skipping the fields that are empty for this resource kind.
func writeResourceHuman(w io.Writer, r resolve.Resource) {
	fmt.Fprintf(w, "kind:  %s\n", r.Kind)
	if r.Key != "" {
		fmt.Fprintf(w, "key:   %s\n", r.Key)
	}
	if r.ID != "" {
		fmt.Fprintf(w, "id:    %s\n", r.ID)
	}
	if r.Title != "" {
		fmt.Fprintf(w, "title: %s\n", r.Title)
	}
	if r.SiteHost != "" {
		fmt.Fprintf(w, "site:  %s\n", r.SiteHost)
	}
}
