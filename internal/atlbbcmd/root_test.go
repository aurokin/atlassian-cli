package atlbbcmd

import (
	"bytes"
	"strings"
	"testing"
)

// TestRootWiring confirms the atl-bb root carries both the shared commands
// (from internal/cli) and the Bitbucket product commands (from internal/bbcmd).
func TestRootWiring(t *testing.T) {
	root, _ := NewRoot("test", "", "")

	want := map[string]bool{
		"version":     false,
		"auth":        false,
		"api":         false,
		"resolve":     false,
		"browse":      false,
		"repo":        false,
		"pr":          false,
		"pipeline":    false,
		"issue":       false,
		"workspace":   false,
		"project":     false,
		"commit":      false,
		"branch":      false,
		"tag":         false,
		"deployment":  false,
		"environment": false,
		"search":      false,
		"status":      false,
	}
	for _, c := range root.Commands() {
		if _, ok := want[c.Name()]; ok {
			want[c.Name()] = true
		}
	}
	for name, found := range want {
		if !found {
			t.Errorf("root is missing the %q command", name)
		}
	}
}

// TestRootHelp confirms the binary name and a product command surface in help.
func TestRootHelp(t *testing.T) {
	root, _ := NewRoot("test", "", "")
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetArgs([]string{"--help"})
	if err := root.Execute(); err != nil {
		t.Fatalf("help: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "atl-bb") {
		t.Fatalf("help missing binary name:\n%s", out)
	}
	if !strings.Contains(out, "repo") {
		t.Fatalf("help missing repo command:\n%s", out)
	}
}
