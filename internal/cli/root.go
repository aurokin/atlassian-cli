// Package cli holds the foundation shared by every atl-* binary: the root
// command shape, the global flag set, and the version subcommand. Product
// command packages (atljiracmd, atlconfcmd) build on top of NewRoot rather
// than duplicating this wiring.
package cli

import (
	"github.com/spf13/cobra"

	"github.com/aurokin/atlassian-cli/internal/appinfo"
	"github.com/aurokin/atlassian-cli/internal/output"
)

// GlobalFlags holds the flags shared by every atl-* binary. A pointer to it is
// returned by NewRoot so product commands and subcommands can read resolved
// values at run time.
type GlobalFlags struct {
	// JSON controls JSON rendering: "" means human output, "*" means all
	// fields, and any other value is a comma-separated top-level field list.
	JSON string
	// JQ is a jq-style filter expression. Phase 1 leaves this as a documented
	// stub; the output renderer reports it as not yet implemented.
	JQ string
	// Site names the configured site profile a command should target.
	Site string
	// NoPrompt forces non-interactive behavior: commands fail instead of
	// prompting. This keeps agent invocations deterministic.
	NoPrompt bool
	// Trace emits verbose request and diagnostic tracing to stderr.
	Trace bool
}

// NewRoot builds the base root command for a binary. It registers the shared
// global flags and the version subcommand, then returns the command together
// with the GlobalFlags it binds. Product command packages add their own
// subcommands to the returned command.
func NewRoot(info appinfo.Info, short string) (*cobra.Command, *GlobalFlags) {
	g := &GlobalFlags{}
	root := &cobra.Command{
		Use:           info.Binary,
		Short:         short,
		SilenceUsage:  true,
		SilenceErrors: false,
	}

	pf := root.PersistentFlags()
	pf.StringVar(&g.JSON, "json", "", "render JSON output; pass a comma-separated field list or '*' for all fields")
	// A bare --json with no value means "all fields".
	pf.Lookup("json").NoOptDefVal = "*"
	pf.StringVar(&g.JQ, "jq", "", "filter JSON output with a jq-style expression")
	pf.StringVar(&g.Site, "site", "", "named site profile to target")
	pf.BoolVar(&g.NoPrompt, "no-prompt", false, "never prompt interactively; fail instead")
	pf.BoolVar(&g.Trace, "trace", false, "emit verbose request tracing to stderr")

	root.AddCommand(newVersionCommand(info, g))
	root.AddCommand(newAuthCommand(info, g))
	return root, g
}

// render writes v to the command's stdout honoring the global --json/--jq
// flags. It is the single rendering entry point for shared subcommands.
func render(cmd *cobra.Command, g *GlobalFlags, v any) error {
	return output.Render(cmd.OutOrStdout(), v, output.Options{JSON: g.JSON, JQ: g.JQ})
}
