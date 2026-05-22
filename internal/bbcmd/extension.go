package bbcmd

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/aurokin/atlassian-cli/internal/apperr"
	"github.com/aurokin/atlassian-cli/internal/appinfo"
	"github.com/aurokin/atlassian-cli/internal/cli"
	"github.com/aurokin/atlassian-cli/internal/output"
)

// extensionPrefix is the executable-name prefix for an atl-bb extension: an
// external binary named atl-bb-<name> on PATH is invokable as
// `atl-bb extension exec <name>` (and, for an unknown command, as
// `atl-bb <name>`).
const extensionPrefix = "atl-bb-"

// execLookPath and executeExternal are package variables so tests can stub
// extension discovery and execution without real binaries.
var (
	execLookPath    = exec.LookPath
	executeExternal = runExternalProcess
)

// ExtensionEntry is one discovered extension: its short name and the resolved
// executable path.
type ExtensionEntry struct {
	Name       string `json:"name"`
	Executable string `json:"executable"`
}

// newExtensionCommand builds the "extension" command group.
func newExtensionCommand(info appinfo.Info, g *cli.GlobalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "extension",
		Aliases: []string{"extensions", "ext"},
		Short:   "Discover and run external atl-bb commands",
		Long:    "Discover and run external commands named atl-bb-<name> found on PATH.",
	}
	cmd.AddCommand(
		newExtensionListCommand(info, g),
		newExtensionExecCommand(info, g),
	)
	return cmd
}

func newExtensionListCommand(_ appinfo.Info, g *cli.GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List discovered atl-bb-<name> commands on PATH",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			entries := discoverExtensions()
			if g.JSON != "" || g.JQ != "" {
				return cli.Render(cmd, g, entries)
			}
			writeExtensionList(cmd.OutOrStdout(), entries)
			return nil
		},
	}
}

func newExtensionExecCommand(_ appinfo.Info, _ *cli.GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:                "exec <name> [args...]",
		Short:              "Run an external atl-bb-<name> command",
		Args:               cobra.MinimumNArgs(1),
		DisableFlagParsing: true, // pass flags through to the extension verbatim
		RunE: func(cmd *cobra.Command, args []string) error {
			return RunExtension(args[0], args[1:])
		},
	}
}

// writeExtensionList prints discovered extensions as aligned name/executable
// rows.
func writeExtensionList(w io.Writer, entries []ExtensionEntry) {
	if len(entries) == 0 {
		fmt.Fprintln(w, "No extensions found on PATH.")
		return
	}
	tw := output.TabWriter(w)
	for _, e := range entries {
		fmt.Fprintf(tw, "%s\t%s\n", e.Name, e.Executable)
	}
	_ = tw.Flush()
}

// RunExtension resolves atl-bb-<name> on PATH and runs it with args, wiring the
// child's stdio to the current process. A missing extension is reported as a
// not-found error.
func RunExtension(name string, args []string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return apperr.InvalidInput("an extension name is required")
	}
	executable, err := execLookPath(extensionPrefix + name)
	if err != nil {
		return apperr.NotFoundOrNotVisible(
			fmt.Sprintf("no atl-bb extension named %q (expected an executable %q on PATH)", name, extensionPrefix+name))
	}
	return executeExternal(executable, args)
}

// ExtensionExitCode reports the process exit code carried by an extension's
// failure, when the extension actually ran and exited non-zero. It lets the
// caller propagate the child's status without re-rendering an error the child
// already reported on its own stderr. A signaled or otherwise codeless failure
// is normalized to 1.
func ExtensionExitCode(err error) (int, bool) {
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		code := ee.ExitCode()
		if code < 1 {
			code = 1
		}
		return code, true
	}
	return 0, false
}

// runExternalProcess runs executable with args, connecting the child's standard
// streams to the current process's.
func runExternalProcess(executable string, args []string) error {
	cmd := exec.Command(executable, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// discoverExtensions scans PATH for executable atl-bb-<name> files, returning
// one entry per name sorted alphabetically. The first match for a name on PATH
// wins, mirroring how the shell resolves a command.
func discoverExtensions() []ExtensionEntry {
	seen := map[string]string{}
	for _, dir := range filepath.SplitList(os.Getenv("PATH")) {
		if strings.TrimSpace(dir) == "" {
			continue
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			name := entry.Name()
			short := strings.TrimPrefix(name, extensionPrefix)
			if short == name || short == "" {
				continue // missing the atl-bb- prefix, or nothing after it
			}
			info, err := entry.Info()
			if err != nil || info.IsDir() || info.Mode()&0o111 == 0 {
				continue
			}
			if _, ok := seen[short]; !ok {
				seen[short] = filepath.Join(dir, name)
			}
		}
	}
	names := make([]string, 0, len(seen))
	for name := range seen {
		names = append(names, name)
	}
	sort.Strings(names)
	out := make([]ExtensionEntry, 0, len(names))
	for _, name := range names {
		out = append(out, ExtensionEntry{Name: name, Executable: seen[name]})
	}
	return out
}

// unknownCommandRe extracts the command name from cobra's unknown-command error
// (`unknown command "X" for "atl-bb"`).
var unknownCommandRe = regexp.MustCompile(`^unknown command "([^"]+)" for `)

// DispatchExtensionFallback attempts to handle a failed root execution as an
// extension: when the error is cobra's unknown-command error and an installed
// atl-bb-<name> extension matches the leading argument, that extension is run.
// It returns handled=true only when an extension was actually found and run;
// runErr is the extension's exit error (nil on success). When handled is false,
// the caller should render the original execution error — including the case
// where the command simply has no matching extension, so the clearer
// unknown-command message stands.
func DispatchExtensionFallback(execErr error, args []string) (handled bool, runErr error) {
	name, rest, ok := extensionTarget(execErr, args)
	if !ok {
		return false, nil
	}
	if _, err := execLookPath(extensionPrefix + name); err != nil {
		return false, nil
	}
	return true, RunExtension(name, rest)
}

// extensionTarget reports whether a failed root execution should be retried as
// an extension. It returns the extension name and the arguments to forward when
// the error is cobra's unknown-command error AND that command is the leading
// argument (so no global flags preceded it, keeping forwarded args
// unambiguous). Otherwise ok is false and the original error stands.
func extensionTarget(err error, args []string) (name string, rest []string, ok bool) {
	if err == nil {
		return "", nil, false
	}
	m := unknownCommandRe.FindStringSubmatch(err.Error())
	if m == nil {
		return "", nil, false
	}
	cmdName := m[1]
	if len(args) == 0 || args[0] != cmdName {
		return "", nil, false
	}
	return cmdName, args[1:], true
}
