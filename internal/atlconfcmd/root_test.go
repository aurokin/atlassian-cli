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
