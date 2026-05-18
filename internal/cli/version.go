package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/aurokin/atlassian-cli/internal/appinfo"
)

// newVersionCommand builds the "version" subcommand. With --json or --jq it
// renders the appinfo.Info through the shared output renderer (honoring field
// selection); otherwise it prints a short human summary.
func newVersionCommand(info appinfo.Info, g *GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print binary, product, and version information",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if g.JSON != "" || g.JQ != "" {
				return Render(cmd, g, info)
			}
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "%s %s (%s)\n", info.Binary, info.Version, info.Product)
			if info.Commit != "" {
				fmt.Fprintf(out, "commit: %s\n", info.Commit)
			}
			if info.Date != "" {
				fmt.Fprintf(out, "built:  %s\n", info.Date)
			}
			return nil
		},
	}
}
