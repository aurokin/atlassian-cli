package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/aurokin/atlassian-cli/internal/apperr"
	"github.com/aurokin/atlassian-cli/internal/appinfo"
	"github.com/aurokin/atlassian-cli/internal/output"
)

// extensionPrefix returns the executable-name prefix for a binary's extensions:
// an external binary named <binary>-<name> on PATH is invokable as
// `<binary> extension exec <name>` (and, for an unknown command, as
// `<binary> <name>`). For atl-bb the prefix is "atl-bb-".
func extensionPrefix(info appinfo.Info) string {
	return info.Binary + "-"
}

// execLookPath and executeExternal are package variables so tests can stub
// extension discovery and execution without real binaries.
var (
	execLookPath    = exec.LookPath
	executeExternal = runExternalProcess
)

// extensionEntry is one discovered extension: its short name and the resolved
// executable path.
type extensionEntry struct {
	Name       string `json:"name"`
	Executable string `json:"executable"`
}

// newExtensionCommand builds the "extension" command group. It is registered on
// every atl-* root by NewRoot; the prefix it discovers under is derived from
// the binary name, so each binary sees only its own <binary>-<name> extensions.
func newExtensionCommand(info appinfo.Info, g *GlobalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "extension",
		Aliases: []string{"extensions", "ext"},
		Short:   "Discover and run external commands",
		Long:    "Discover and run external commands named " + extensionPrefix(info) + "<name> found on PATH.",
	}
	cmd.AddCommand(
		newExtensionListCommand(info, g),
		newExtensionExecCommand(info, g),
	)
	return cmd
}

func newExtensionListCommand(info appinfo.Info, g *GlobalFlags) *cobra.Command {
	prefix := extensionPrefix(info)
	return &cobra.Command{
		Use:   "list",
		Short: "List discovered " + prefix + "<name> commands on PATH",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			entries := discoverExtensions(prefix)
			if g.WantsStructured() {
				return Render(cmd, g, entries)
			}
			writeExtensionList(cmd.OutOrStdout(), entries)
			return nil
		},
	}
}

func newExtensionExecCommand(info appinfo.Info, _ *GlobalFlags) *cobra.Command {
	prefix := extensionPrefix(info)
	return &cobra.Command{
		Use:                "exec <name> [args...]",
		Short:              "Run an external " + prefix + "<name> command",
		Args:               cobra.MinimumNArgs(1),
		DisableFlagParsing: true, // pass flags through to the extension verbatim
		RunE: func(cmd *cobra.Command, args []string) error {
			return runExtension(prefix, args[0], args[1:])
		},
	}
}

// writeExtensionList prints discovered extensions as aligned name/executable
// rows.
func writeExtensionList(w io.Writer, entries []extensionEntry) {
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

// runExtension resolves <prefix><name> on PATH and runs it with args, wiring the
// child's stdio to the current process. A missing extension is reported as a
// not-found error.
func runExtension(prefix, name string, args []string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return apperr.InvalidInput("an extension name is required")
	}
	executable, err := execLookPath(prefix + name)
	if err != nil {
		return apperr.NotFoundOrNotVisible(
			fmt.Sprintf("no extension named %q (expected an executable %q on PATH)", name, prefix+name))
	}
	return executeExternal(executable, args)
}

// extensionExitCode reports the process exit code carried by an extension's
// failure, when the extension actually ran and exited non-zero. It lets the
// caller propagate the child's status without re-rendering an error the child
// already reported on its own stderr. A signaled or otherwise codeless failure
// is normalized to 1.
func extensionExitCode(err error) (int, bool) {
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

// windowsExecSuffixes returns the lowercased executable suffixes Windows
// resolves via PATHEXT — the same list exec.LookPath consults — falling back
// to Go's default when the variable is unset.
func windowsExecSuffixes(pathext string) []string {
	if pathext == "" {
		pathext = ".COM;.EXE;.BAT;.CMD"
	}
	var out []string
	for _, e := range strings.Split(strings.ToLower(pathext), ";") {
		if e = strings.TrimSpace(e); e != "" {
			out = append(out, e)
		}
	}
	return out
}

// extensionName reports whether a PATH entry name is a <prefix><name>
// extension candidate and returns its short name; it is a pure name test so
// only candidates are stat'd. On Windows, where files carry no execute bit, a
// suffix from suffixes (see windowsExecSuffixes) decides and is stripped from
// the name so `extension list` shows what `extension exec` accepts; elsewhere
// isExecutable checks the execute bit afterwards.
//
// rank is the suffix's position in suffixes (always 0 off Windows): when one
// directory holds several candidates for a name, exec.LookPath runs the one
// with the lowest-ranked suffix, so discovery prefers it too.
func extensionName(goos string, suffixes []string, prefix, name string) (short string, rank int, ok bool) {
	short = strings.TrimPrefix(name, prefix)
	if short == name || short == "" {
		return "", 0, false // missing the prefix, or nothing after it
	}
	if goos != "windows" {
		return short, 0, true
	}
	ext := strings.ToLower(filepath.Ext(short))
	rank = slices.Index(suffixes, ext)
	if ext == "" || rank < 0 {
		return "", 0, false
	}
	if short = short[:len(short)-len(ext)]; short == "" {
		return "", 0, false
	}
	return short, rank, true
}

// extensionCandidate is a discovered executable for one short name, with the
// PATH directory it came from and its suffix rank, so collisions resolve the
// way exec.LookPath does: the earliest PATH directory wins, and within one
// directory the lowest-ranked suffix wins.
type extensionCandidate struct {
	executable string
	dir        string
	rank       int
}

// supersedes reports whether next should replace prev as the entry for a
// short name. prev is the zero value when the name is new.
func (next extensionCandidate) supersedes(prev extensionCandidate) bool {
	if prev.executable == "" {
		return true
	}
	return prev.dir == next.dir && next.rank < prev.rank
}

// isExecutable reports whether path is a regular executable file, following
// symlinks so a link to a directory or to a non-executable is excluded. On
// Windows the PATHEXT suffix has already decided, so only the directory check
// applies.
func isExecutable(path string) bool {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return false
	}
	return runtime.GOOS == "windows" || info.Mode()&0o111 != 0
}

// discoverExtensions scans PATH for <prefix><name> files that extensionName
// accepts, returning one entry per name sorted alphabetically. The first match
// for a name on PATH wins, mirroring how the shell resolves a command.
func discoverExtensions(prefix string) []extensionEntry {
	seen := map[string]extensionCandidate{}
	suffixes := windowsExecSuffixes(os.Getenv("PATHEXT"))
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
			short, rank, ok := extensionName(runtime.GOOS, suffixes, prefix, name)
			if !ok {
				continue
			}
			next := extensionCandidate{executable: filepath.Join(dir, name), dir: dir, rank: rank}
			if !next.supersedes(seen[short]) || !isExecutable(next.executable) {
				continue
			}
			seen[short] = next
		}
	}
	names := make([]string, 0, len(seen))
	for name := range seen {
		names = append(names, name)
	}
	sort.Strings(names)
	out := make([]extensionEntry, 0, len(names))
	for _, name := range names {
		out = append(out, extensionEntry{Name: name, Executable: seen[name].executable})
	}
	return out
}

// unknownCommandRe extracts the command name from cobra's unknown-command error
// (e.g. `unknown command "X" for "atl-jira"`); the trailing binary name is not
// matched, so this works for any atl-* binary.
var unknownCommandRe = regexp.MustCompile(`^unknown command "([^"]+)" for `)

// dispatchExtensionFallback attempts to handle a failed root execution as an
// extension: when the error is cobra's unknown-command error and an installed
// <prefix><name> extension matches the leading argument, that extension is run.
// It returns handled=true only when an extension was actually found and run;
// runErr is the extension's exit error (nil on success). When handled is false,
// the caller should render the original execution error — including the case
// where the command simply has no matching extension, so the clearer
// unknown-command message stands.
func dispatchExtensionFallback(prefix string, execErr error, args []string) (handled bool, runErr error) {
	name, rest, ok := extensionTarget(execErr, args)
	if !ok {
		return false, nil
	}
	if _, err := execLookPath(prefix + name); err != nil {
		return false, nil
	}
	return true, runExtension(prefix, name, rest)
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
