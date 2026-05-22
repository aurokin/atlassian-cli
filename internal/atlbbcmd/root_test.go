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
		"alias":       false,
		"extension":   false,
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

// TestResolveBitbucketURL confirms the shared resolve command dispatches to the
// Bitbucket parser for the atl-bb product (an offline parse, no network).
func TestResolveBitbucketURL(t *testing.T) {
	root, _ := NewRoot("test", "", "")
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"resolve", "https://bitbucket.org/acme/widgets/pull-requests/42", "--jq", ".kind + \" \" + .id"})
	if err := root.Execute(); err != nil {
		t.Fatalf("resolve: %v\n%s", err, buf.String())
	}
	if got := strings.TrimSpace(buf.String()); got != `"bitbucket_pull_request 42"` {
		t.Fatalf("resolve output = %q", got)
	}
}

// TestBrowseBitbucketURL confirms the shared browse command builds the
// canonical bitbucket.org URL and, with --no-browser, prints it.
func TestBrowseBitbucketURL(t *testing.T) {
	root, _ := NewRoot("test", "", "")
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"browse", "https://bitbucket.org/acme/widgets/commits/abc123", "--no-browser"})
	if err := root.Execute(); err != nil {
		t.Fatalf("browse: %v\n%s", err, buf.String())
	}
	if got := strings.TrimSpace(buf.String()); got != "https://bitbucket.org/acme/widgets/commits/abc123" {
		t.Fatalf("browse output = %q", got)
	}
}
