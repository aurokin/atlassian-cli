package cli

import (
	"bytes"
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
