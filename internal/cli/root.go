// Package cli holds the foundation shared by every atl-* binary: the root
// command shape, the global flag set, the shared subcommands (version, auth,
// api, resolve, browse, alias, extension), and the Run/Execute entry points.
// Product command packages (atljiracmd, atlconfcmd, atlbbcmd) build on top of
// NewRoot rather than duplicating this wiring.
package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

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
	// JQ is a jq-style filter expression applied to the structured JSON
	// output (implemented via gojq in internal/output). A non-empty JQ, like
	// a non-empty JSON, selects machine-readable output over human rendering.
	JQ string
	// Site names the configured site profile a command should target.
	Site string
	// NoPrompt forces non-interactive behavior: commands fail instead of
	// prompting. This keeps agent invocations deterministic.
	NoPrompt bool
	// Trace emits verbose request and diagnostic tracing to stderr.
	Trace bool
}

// WantsStructured reports whether the caller selected machine-readable output:
// a non-empty --json field selector or a non-empty --jq filter. It is the
// single source of the human-vs-structured decision, used by every command's
// render branch and by the error renderer, so the two never diverge.
func (g *GlobalFlags) WantsStructured() bool {
	return g.JSON != "" || g.JQ != ""
}

// Command group IDs partition the help listing so a long command surface stays
// scannable. Product command packages tag their top-level commands with
// GroupProduct; the shared subcommands are assigned here.
const (
	// GroupProduct holds the product-specific commands (issue, page, repo, …).
	GroupProduct = "product"
	// GroupConfig holds authentication and configuration commands.
	GroupConfig = "config"
	// GroupAdvanced holds the lower-level escape hatches and extensibility
	// commands (api, resolve, browse, alias, extension).
	GroupAdvanced = "advanced"
)

// NewRoot builds the base root command for a binary. It registers the shared
// global flags, command groups, and the shared subcommands, then returns the
// command together with the GlobalFlags it binds. Product command packages add
// their own subcommands (tagged with GroupProduct) to the returned command.
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

	root.AddGroup(
		&cobra.Group{ID: GroupProduct, Title: "Product Commands:"},
		&cobra.Group{ID: GroupConfig, Title: "Auth & Config Commands:"},
		&cobra.Group{ID: GroupAdvanced, Title: "Advanced Commands:"},
	)

	pf := root.PersistentFlags()
	pf.StringVar(&g.JSON, "json", "", "render JSON output; pass a comma-separated field list or '*' for all fields")
	// A bare --json with no value means "all fields".
	pf.Lookup("json").NoOptDefVal = "*"
	pf.StringVar(&g.JQ, "jq", "", "filter JSON output with a jq-style expression")
	pf.StringVar(&g.Site, "site", "", "named site profile to target")
	pf.BoolVar(&g.NoPrompt, "no-prompt", false, "never prompt interactively; fail instead")
	pf.BoolVar(&g.Trace, "trace", false, "emit verbose request tracing to stderr")
	// Offer configured site names when completing --site.
	_ = root.RegisterFlagCompletionFunc("site", completeSiteNames)

	addGrouped(root, GroupConfig,
		newVersionCommand(info, g),
		newAuthCommand(info, g),
	)
	addGrouped(root, GroupAdvanced,
		newAPICommand(info, g),
		newResolveCommand(info, g),
		newBrowseCommand(info, g),
		newAliasCommand(info, g),
		newExtensionCommand(info, g),
	)
	// File cobra's auto-generated help/completion commands under Advanced so
	// they do not land in an ungrouped "Additional Commands" section.
	root.SetHelpCommandGroupID(GroupAdvanced)
	root.SetCompletionCommandGroupID(GroupAdvanced)
	return root, g
}

// AddProductCommands tags each command with the product group and adds it to
// root. Product command packages (jiracmd, confcmd, bbcmd) use it so their
// top-level commands file under "Product Commands:" in the help listing.
func AddProductCommands(root *cobra.Command, cmds ...*cobra.Command) {
	addGrouped(root, GroupProduct, cmds...)
}

// addGrouped tags each command with the group ID and adds it to parent, so the
// help listing files them under the group's title.
func addGrouped(parent *cobra.Command, groupID string, cmds ...*cobra.Command) {
	for _, c := range cmds {
		c.GroupID = groupID
		parent.AddCommand(c)
	}
}

// Render writes v to the command's stdout honoring the global --json/--jq
// flags. It is the single rendering entry point for both shared subcommands
// and product command packages.
func Render(cmd *cobra.Command, g *GlobalFlags, v any) error {
	return output.Render(cmd.OutOrStdout(), v, output.Options{JSON: g.JSON, JQ: g.JQ})
}

// RenderDecoded folds the structured-gate + decode + write boilerplate that
// every list/view command repeats. When structured output is selected it
// renders the raw upstream body verbatim; otherwise it decodes raw with the
// product's Decode function and hands the typed value to writeHuman for the
// compact per-type summary.
func RenderDecoded[T any](
	cmd *cobra.Command,
	g *GlobalFlags,
	raw json.RawMessage,
	decode func(json.RawMessage) (T, error),
	writeHuman func(w io.Writer, v T),
) error {
	if g.WantsStructured() {
		return Render(cmd, g, raw)
	}
	v, err := decode(raw)
	if err != nil {
		return err
	}
	writeHuman(cmd.OutOrStdout(), v)
	return nil
}

// Execute runs root and renders any resulting error, returning the process
// exit code. It is the minimal entry point: it performs no alias expansion or
// extension dispatch, and is used by tests and as the building block beneath
// Run.
func Execute(root *cobra.Command, g *GlobalFlags) int {
	if err := root.Execute(); err != nil {
		renderError(root.ErrOrStderr(), g, err)
		return exitCode(err)
	}
	return 0
}

// Run is the production entry point shared by every atl-* binary. It expands
// any configured command aliases against the process arguments, executes the
// command tree, and—when an unknown command names an installed
// <binary>-<name> extension on PATH—dispatches to that extension (gh-style). It
// returns the process exit code. The extension prefix is derived from
// info.Binary, so each binary discovers only its own extensions.
func Run(info appinfo.Info, root *cobra.Command, g *GlobalFlags) int {
	args, err := expandAliases(os.Args[1:])
	if err != nil {
		// A malformed alias in a hand-edited config fails before dispatch.
		// Report it plainly and stop.
		fmt.Fprintln(root.ErrOrStderr(), "Error:", err)
		return 1
	}
	root.SetArgs(args)
	execErr := root.Execute()
	if execErr == nil {
		return 0
	}
	// An extension invoked explicitly via `extension exec` that exited non-zero
	// already wrote its own diagnostics, so propagate its exit code without
	// rendering a redundant error.
	if code, ok := extensionExitCode(execErr); ok {
		return code
	}
	// gh-style fallback: an unknown command may name an external
	// <binary>-<name> extension on PATH. Only a found-and-run extension is
	// treated as handled; otherwise the original (clearer) error is rendered.
	if handled, runErr := dispatchExtensionFallback(extensionPrefix(info), execErr, args); handled {
		if runErr == nil {
			return 0
		}
		if code, ok := extensionExitCode(runErr); ok {
			return code
		}
		renderError(root.ErrOrStderr(), g, runErr)
		return exitCode(runErr)
	}
	renderError(root.ErrOrStderr(), g, execErr)
	return exitCode(execErr)
}

// exitCode returns the process exit code for err: a structured *apperr.Error
// maps its category to a stable code (see apperr.Error.ExitCode), and anything
// else exits 1.
func exitCode(err error) int {
	var ae *apperr.Error
	if errors.As(err, &ae) {
		return ae.ExitCode()
	}
	return 1
}

// renderError writes err to w. When machine-readable output is selected
// (--json or --jq) and err carries a structured *apperr.Error, the full
// machine-readable envelope is emitted; otherwise a plain text line is
// written. Gating on both flags keeps the error path consistent with the
// success path, where --jq alone also selects structured output.
func renderError(w io.Writer, g *GlobalFlags, err error) {
	var ae *apperr.Error
	if g.WantsStructured() && errors.As(err, &ae) {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		if enc.Encode(ae) == nil {
			return
		}
	}
	fmt.Fprintln(w, "Error:", err)
}
