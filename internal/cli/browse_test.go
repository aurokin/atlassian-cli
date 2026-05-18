package cli

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/aurokin/atlassian-cli/internal/apperr"
)

// swapBrowseOpener replaces browseOpener with fn and returns a restore func.
func swapBrowseOpener(fn func(string) error) func() {
	prev := browseOpener
	browseOpener = fn
	return func() { browseOpener = prev }
}

// loginBrowseSite records a minimal site profile named "work" so browse can
// build a canonical URL from a bare key.
func loginBrowseSite(t *testing.T) {
	t.Helper()
	if _, err := execRoot(t, jiraInfo(), "auth", "login", "--site", "work",
		"--url", "https://example.atlassian.net", "--token-style", "data-center-pat"); err != nil {
		t.Fatalf("login: %v", err)
	}
}

func TestBrowseBareKeyWithSiteBuildsCanonicalURL(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginBrowseSite(t)

	var opened string
	defer swapBrowseOpener(func(u string) error { opened = u; return nil })()

	if _, err := execRoot(t, jiraInfo(), "browse", "PROJ-123", "--site", "work"); err != nil {
		t.Fatalf("browse: %v", err)
	}
	const want = "https://example.atlassian.net/browse/PROJ-123"
	if opened != want {
		t.Fatalf("opened %q, want %q", opened, want)
	}
}

func TestBrowseFullURLNormalizesToCanonical(t *testing.T) {
	var opened string
	defer swapBrowseOpener(func(u string) error { opened = u; return nil })()

	if _, err := execRoot(t, jiraInfo(), "browse",
		"https://x.atlassian.net/jira/software/projects/PROJ/boards/1"); err != nil {
		t.Fatalf("browse: %v", err)
	}
	const want = "https://x.atlassian.net/browse/PROJ"
	if opened != want {
		t.Fatalf("opened %q, want %q", opened, want)
	}
}

func TestBrowseNoBrowserPrintsAndDoesNotOpen(t *testing.T) {
	called := false
	defer swapBrowseOpener(func(string) error { called = true; return nil })()

	out, err := execRoot(t, jiraInfo(), "browse",
		"https://x.atlassian.net/browse/PROJ-7", "--no-browser")
	if err != nil {
		t.Fatalf("browse --no-browser: %v", err)
	}
	if called {
		t.Fatal("browse --no-browser invoked the opener")
	}
	if !strings.Contains(out, "https://x.atlassian.net/browse/PROJ-7") {
		t.Fatalf("browse --no-browser did not print the URL:\n%s", out)
	}
}

func TestBrowseNoPromptImpliesNoBrowser(t *testing.T) {
	called := false
	defer swapBrowseOpener(func(string) error { called = true; return nil })()

	out, err := execRoot(t, jiraInfo(), "browse",
		"https://x.atlassian.net/browse/PROJ-7", "--no-prompt")
	if err != nil {
		t.Fatalf("browse --no-prompt: %v", err)
	}
	if called {
		t.Fatal("browse --no-prompt invoked the opener")
	}
	if !strings.Contains(out, "https://x.atlassian.net/browse/PROJ-7") {
		t.Fatalf("browse --no-prompt did not print the URL:\n%s", out)
	}
}

func TestBrowseNoBrowserJSONEmitsJSONString(t *testing.T) {
	called := false
	defer swapBrowseOpener(func(string) error { called = true; return nil })()

	out, err := execRoot(t, jiraInfo(), "browse",
		"https://x.atlassian.net/browse/PROJ-7", "--no-browser", "--json")
	if err != nil {
		t.Fatalf("browse --no-browser --json: %v", err)
	}
	if called {
		t.Fatal("browse --no-browser invoked the opener")
	}
	var got string
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("browse --json output is not a JSON string: %v\n%s", err, out)
	}
	if got != "https://x.atlassian.net/browse/PROJ-7" {
		t.Fatalf("decoded URL = %q, want the canonical browse URL", got)
	}
}

func TestBrowseBareKeyWithoutSiteErrors(t *testing.T) {
	called := false
	defer swapBrowseOpener(func(string) error { called = true; return nil })()

	_, err := execRoot(t, jiraInfo(), "browse", "PROJ-123", "--no-browser")
	if err == nil {
		t.Fatal("browse of a bare key without --site returned no error")
	}
	var ae *apperr.Error
	if !errors.As(err, &ae) {
		t.Fatalf("error type = %T, want *apperr.Error", err)
	}
	if called {
		t.Fatal("browse invoked the opener despite the missing site")
	}
}

func TestBrowseUnresolvedInputErrors(t *testing.T) {
	defer swapBrowseOpener(func(string) error {
		t.Error("browse invoked the opener for an unresolved input")
		return nil
	})()

	_, err := execRoot(t, jiraInfo(), "browse", "garbage input", "--no-browser")
	if err == nil {
		t.Fatal("browse of an unresolved input returned no error")
	}
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Code != "unresolved" {
		t.Fatalf("error = %v, want an unresolved *apperr.Error", err)
	}
}
