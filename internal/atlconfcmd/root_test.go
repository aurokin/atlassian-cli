package atlconfcmd

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestNewRootUse(t *testing.T) {
	root, _ := NewRoot("test", "", "")
	if root.Use != "atl-conf" {
		t.Fatalf("Use = %q, want %q", root.Use, "atl-conf")
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
	if !strings.Contains(buf.String(), "atl-conf") {
		t.Fatalf("help output missing binary name:\n%s", buf.String())
	}
}

// TestRootRegistersSharedAliasAndExtension confirms the alias and extension
// command groups, promoted from atl-bb into the shared root, are now present on
// atl-conf too.
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
			t.Errorf("atl-conf root is missing the shared %q command", name)
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
	if got["binary"] != "atl-conf" {
		t.Errorf("binary = %v, want atl-conf", got["binary"])
	}
	if got["product"] != "confluence" {
		t.Errorf("product = %v, want confluence", got["product"])
	}
	if got["version"] != "9.9.9" {
		t.Errorf("version = %v, want 9.9.9", got["version"])
	}
}
