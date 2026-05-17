package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/aurokin/atlassian-cli/internal/appinfo"
)

// newVersionCommand builds the "version" subcommand. With --json set it emits
// the appinfo.Info as JSON; otherwise it prints a short human summary.
func newVersionCommand(info appinfo.Info, g *GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print binary, product, and version information",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			out := cmd.OutOrStdout()
			if g.JSON != "" {
				enc := json.NewEncoder(out)
				enc.SetIndent("", "  ")
				return enc.Encode(info)
			}
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
