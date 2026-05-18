package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/aurokin/atlassian-cli/internal/appinfo"
)

func TestNewRootRegistersGlobalFlags(t *testing.T) {
	root, _ := NewRoot(appinfo.New("atl-jira", appinfo.ProductJira, "test", "", ""), "short")
	for _, name := range []string{"json", "jq", "site", "no-prompt", "trace"} {
		if root.PersistentFlags().Lookup(name) == nil {
			t.Errorf("missing global flag --%s", name)
		}
	}
}

func TestVersionHumanOutput(t *testing.T) {
	root, _ := NewRoot(appinfo.New("atl-conf", appinfo.ProductConfluence, "2.0.0", "", ""), "short")
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"version"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute(version): %v", err)
	}
	if !strings.Contains(buf.String(), "atl-conf 2.0.0") {
		t.Fatalf("version human output unexpected:\n%s", buf.String())
	}
}

func TestBareJSONFlagSelectsAllFields(t *testing.T) {
	root, g := NewRoot(appinfo.New("atl-jira", appinfo.ProductJira, "test", "", ""), "short")
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"version", "--json"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute(version --json): %v", err)
	}
	if g.JSON != "*" {
		t.Fatalf("bare --json set JSON = %q, want %q", g.JSON, "*")
	}
}

func TestVersionJSONHonorsFieldSelection(t *testing.T) {
	root, _ := NewRoot(appinfo.New("atl-jira", appinfo.ProductJira, "1.2.3", "", ""), "short")
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"version", "--json=version"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("version --json=version is not valid JSON: %v\n%s", err, buf.String())
	}
	if len(got) != 1 || got["version"] != "1.2.3" {
		t.Fatalf("version ignored field selection: %v", got)
	}
}

func TestVersionJQReturnsNotImplemented(t *testing.T) {
	root, _ := NewRoot(appinfo.New("atl-jira", appinfo.ProductJira, "1.2.3", "", ""), "short")
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"version", "--jq=.version"})
	if err := root.Execute(); err == nil {
		t.Fatal("version --jq returned no error; expected not-implemented")
	}
}

func TestExecuteRendersJSONErrorEnvelope(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	root, g := NewRoot(jiraInfo(), "short")
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	// api against an unconfigured site fails with a structured *apperr.Error.
	root.SetArgs([]string{"api", "/myself", "--site", "absent", "--json"})
	if code := Execute(root, g); code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	var got map[string]any
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("error was not rendered as a JSON envelope: %v\n%s", err, buf.String())
	}
	if code, _ := got["error"].(string); code == "" {
		t.Fatalf("JSON envelope missing the 'error' code: %v", got)
	}
}

func TestExecuteRendersTextErrorWithoutJSON(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	root, g := NewRoot(jiraInfo(), "short")
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"api", "/myself", "--site", "absent"})
	if code := Execute(root, g); code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.HasPrefix(buf.String(), "Error:") {
		t.Fatalf("expected a plain text error line:\n%s", buf.String())
	}
}

func TestExecuteReturnsZeroOnSuccess(t *testing.T) {
	root, g := NewRoot(jiraInfo(), "short")
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"version"})
	if code := Execute(root, g); code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
}
