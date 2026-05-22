package atljiracmd

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestNewRootUse(t *testing.T) {
	root, _ := NewRoot("test", "", "")
	if root.Use != "atl-jira" {
		t.Fatalf("Use = %q, want %q", root.Use, "atl-jira")
	}
}

func TestRootHelpContainsBinaryName(t *testing.T) {
	root, _ := NewRoot("test", "", "")
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"--help"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute(--help): %v", err)
	}
	if !strings.Contains(buf.String(), "atl-jira") {
		t.Fatalf("help output missing binary name:\n%s", buf.String())
	}
}

// TestRootRegistersSharedAliasAndExtension confirms the alias and extension
// command groups, promoted from atl-bb into the shared root, are now present on
// atl-jira too.
func TestRootRegistersSharedAliasAndExtension(t *testing.T) {
	root, _ := NewRoot("test", "", "")
	want := map[string]bool{"alias": false, "extension": false}
	for _, c := range root.Commands() {
		if _, ok := want[c.Name()]; ok {
			want[c.Name()] = true
		}
	}
	for name, found := range want {
		if !found {
			t.Errorf("atl-jira root is missing the shared %q command", name)
		}
	}
}

func TestVersionJSON(t *testing.T) {
	root, _ := NewRoot("9.9.9", "", "")
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"version", "--json"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute(version --json): %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("version --json produced invalid JSON: %v\n%s", err, buf.String())
	}
	if got["binary"] != "atl-jira" {
		t.Errorf("binary = %v, want atl-jira", got["binary"])
	}
	if got["product"] != "jira" {
		t.Errorf("product = %v, want jira", got["product"])
	}
	if got["version"] != "9.9.9" {
		t.Errorf("version = %v, want 9.9.9", got["version"])
	}
}
