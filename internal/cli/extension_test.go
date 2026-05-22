package cli

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/aurokin/atlassian-cli/internal/appinfo"
)

// bbInfo is the appinfo used by the extension tests; its binary name fixes the
// discovery prefix to "atl-bb-".
func bbInfo() appinfo.Info {
	return appinfo.New("atl-bb", appinfo.ProductBitbucket, "test", "", "")
}

// writeFakeExtension creates an executable file named <prefix><name> in dir.
func writeFakeExtension(t *testing.T, dir, prefix, name string) {
	t.Helper()
	path := filepath.Join(dir, prefix+name)
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatalf("write fake extension: %v", err)
	}
}

func TestExtensionPrefixDerivedFromBinary(t *testing.T) {
	if got := extensionPrefix(bbInfo()); got != "atl-bb-" {
		t.Fatalf("extensionPrefix(atl-bb) = %q, want atl-bb-", got)
	}
	if got := extensionPrefix(jiraInfo()); got != "atl-jira-" {
		t.Fatalf("extensionPrefix(atl-jira) = %q, want atl-jira-", got)
	}
}

func TestExtensionListHuman(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("executable-bit discovery is POSIX-specific")
	}
	prefix := extensionPrefix(bbInfo())
	dir := t.TempDir()
	writeFakeExtension(t, dir, prefix, "hello")
	writeFakeExtension(t, dir, prefix, "world")
	// A non-extension file and a non-executable one must be ignored.
	if err := os.WriteFile(filepath.Join(dir, "unrelated"), []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, prefix+"plain"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)

	out, err := execRoot(t, bbInfo(), "extension", "list")
	if err != nil {
		t.Fatalf("extension list: %v\n%s", err, out)
	}
	for _, want := range []string{"hello", "world"} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "plain") || strings.Contains(out, "unrelated") {
		t.Fatalf("output should exclude non-executable / non-extension files:\n%s", out)
	}
}

func TestExtensionListEmpty(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	out, err := execRoot(t, bbInfo(), "extension", "list")
	if err != nil {
		t.Fatalf("extension list: %v\n%s", err, out)
	}
	if !strings.Contains(out, "No extensions found on PATH.") {
		t.Fatalf("unexpected output:\n%s", out)
	}
}

func TestExtensionExecRunsBinary(t *testing.T) {
	prefix := extensionPrefix(bbInfo())
	var gotExe string
	var gotArgs []string
	origLook, origExec := execLookPath, executeExternal
	execLookPath = func(file string) (string, error) {
		if file != prefix+"hello" {
			return "", errors.New("not found")
		}
		return "/usr/local/bin/" + file, nil
	}
	executeExternal = func(executable string, args []string) error {
		gotExe = executable
		gotArgs = args
		return nil
	}
	t.Cleanup(func() { execLookPath, executeExternal = origLook, origExec })

	out, err := execRoot(t, bbInfo(), "extension", "exec", "hello", "--flag", "value")
	if err != nil {
		t.Fatalf("extension exec: %v\n%s", err, out)
	}
	if gotExe != "/usr/local/bin/atl-bb-hello" {
		t.Fatalf("executable = %q", gotExe)
	}
	if !reflect.DeepEqual(gotArgs, []string{"--flag", "value"}) {
		t.Fatalf("args = %v", gotArgs)
	}
}

func TestExtensionExecMissing(t *testing.T) {
	origLook := execLookPath
	execLookPath = func(string) (string, error) { return "", errors.New("not found") }
	t.Cleanup(func() { execLookPath = origLook })

	_, err := execRoot(t, bbInfo(), "extension", "exec", "ghost")
	if err == nil || !strings.Contains(err.Error(), `no extension named "ghost"`) {
		t.Fatalf("expected not-found error, got %v", err)
	}
}

func TestExtensionTarget(t *testing.T) {
	err := errors.New(`unknown command "deploy-helper" for "atl-bb"`)
	name, rest, ok := extensionTarget(err, []string{"deploy-helper", "a", "b"})
	if !ok || name != "deploy-helper" || !reflect.DeepEqual(rest, []string{"a", "b"}) {
		t.Fatalf("extensionTarget = %q, %v, %v", name, rest, ok)
	}

	// A global flag before the command means args[0] is not the command name;
	// the fallback must not trigger (forwarded args would be ambiguous).
	if _, _, ok := extensionTarget(err, []string{"--site", "x", "deploy-helper"}); ok {
		t.Fatal("extensionTarget should not trigger when a flag precedes the command")
	}
	// A non-unknown-command error must not trigger.
	if _, _, ok := extensionTarget(errors.New("some other error"), []string{"deploy-helper"}); ok {
		t.Fatal("extensionTarget should only match the unknown-command error")
	}
	// A nil error must not trigger.
	if _, _, ok := extensionTarget(nil, []string{"deploy-helper"}); ok {
		t.Fatal("extensionTarget should not match a nil error")
	}
}

func TestExtensionExitCode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses a POSIX shell to produce a real ExitError")
	}
	// A real child that exits non-zero yields an *exec.ExitError carrying the
	// code.
	err := runExternalProcess("/bin/sh", []string{"-c", "exit 3"})
	code, ok := extensionExitCode(err)
	if !ok || code != 3 {
		t.Fatalf("extensionExitCode = %d, %v; want 3, true", code, ok)
	}
	// A non-exit error (e.g. not-found) is not an exit code.
	if _, ok := extensionExitCode(errors.New("boom")); ok {
		t.Fatal("extensionExitCode should not report a code for a non-exit error")
	}
	if _, ok := extensionExitCode(nil); ok {
		t.Fatal("extensionExitCode should report false for a nil error")
	}
}

func TestDispatchExtensionFallback(t *testing.T) {
	prefix := extensionPrefix(bbInfo())
	var ran bool
	origLook, origExec := execLookPath, executeExternal
	execLookPath = func(file string) (string, error) {
		if file == prefix+"helper" {
			return "/bin/" + file, nil
		}
		return "", errors.New("not found")
	}
	executeExternal = func(string, []string) error { ran = true; return nil }
	t.Cleanup(func() { execLookPath, executeExternal = origLook, origExec })

	// A matching extension is found and run → handled.
	handled, runErr := dispatchExtensionFallback(prefix,
		errors.New(`unknown command "helper" for "atl-bb"`), []string{"helper", "x"})
	if !handled || runErr != nil || !ran {
		t.Fatalf("handled=%v runErr=%v ran=%v", handled, runErr, ran)
	}

	// No matching extension → not handled (the original error stands).
	handled, _ = dispatchExtensionFallback(prefix,
		errors.New(`unknown command "ghost" for "atl-bb"`), []string{"ghost"})
	if handled {
		t.Fatal("dispatchExtensionFallback should not handle a missing extension")
	}
}
