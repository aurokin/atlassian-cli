// Package cli holds the foundation shared by every atl-* binary: the root
// command shape, the global flag set, and the version subcommand. Product
// command packages (atljiracmd, atlconfcmd) build on top of NewRoot rather
// than duplicating this wiring.
package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/aurokin/atlassian-cli/internal/apperr"
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
		Use:   info.Binary,
		Short: short,
		// Errors are rendered by Execute so they can use the structured
		// apperr envelope; cobra must not also print them.
		SilenceUsage:  true,
		SilenceErrors: true,
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
	root.AddCommand(newAPICommand(info, g))
	root.AddCommand(newResolveCommand(info, g))
	root.AddCommand(newBrowseCommand(info, g))
	return root, g
}

// Render writes v to the command's stdout honoring the global --json/--jq
// flags. It is the single rendering entry point for both shared subcommands
// and product command packages.
func Render(cmd *cobra.Command, g *GlobalFlags, v any) error {
	return output.Render(cmd.OutOrStdout(), v, output.Options{JSON: g.JSON, JQ: g.JQ})
}

// Execute runs root and renders any resulting error, returning the process
// exit code. It is the entry point used by each binary's main package.
func Execute(root *cobra.Command, g *GlobalFlags) int {
	if err := root.Execute(); err != nil {
		renderError(root.ErrOrStderr(), g, err)
		return 1
	}
	return 0
}

// RenderError writes err to w using the same formatting as Execute: the
// structured JSON envelope when --json is set and err is an *apperr.Error,
// otherwise a plain text line. It is exported so a binary that runs its own
// dispatch (for example atl-bb's extension fallback) can render errors
// identically to Execute.
func RenderError(w io.Writer, g *GlobalFlags, err error) {
	renderError(w, g, err)
}

// renderError writes err to w. When --json is set and err carries a
// structured *apperr.Error, the full machine-readable envelope is emitted;
// otherwise a plain text line is written.
func renderError(w io.Writer, g *GlobalFlags, err error) {
	var ae *apperr.Error
	if g.JSON != "" && errors.As(err, &ae) {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		if enc.Encode(ae) == nil {
			return
		}
	}
	fmt.Fprintln(w, "Error:", err)
}
