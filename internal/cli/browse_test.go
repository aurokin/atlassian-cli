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
		"--url", "https://example.atlassian.net", "--token-style", "data-center-pat",
		"--token-env", "ATL_TEST_TOKEN"); err != nil {
		t.Fatalf("login: %v", err)
	}
}

func TestBrowseBareKeyWithSiteBuildsCanonicalURL(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginBrowseSite(t)

	var opened string
	defer swapBrowseOpener(func(u string) error { opened = u; return nil })()

	out, err := execRoot(t, jiraInfo(), "browse", "PROJ-123", "--site", "work")
	if err != nil {
		t.Fatalf("browse: %v", err)
	}
	const want = "https://example.atlassian.net/browse/PROJ-123"
	if opened != want {
		t.Fatalf("opened %q, want %q", opened, want)
	}
	if !strings.Contains(out, "Opened "+want) {
		t.Errorf("missing the 'Opened' confirmation line:\n%s", out)
	}
}

func TestBrowseFullURLNormalizesToCanonical(t *testing.T) {
	var opened string
	defer swapBrowseOpener(func(u string) error { opened = u; return nil })()

	out, err := execRoot(t, jiraInfo(), "browse",
		"https://x.atlassian.net/jira/software/projects/PROJ/boards/1")
	if err != nil {
		t.Fatalf("browse: %v", err)
	}
	const want = "https://x.atlassian.net/browse/PROJ"
	if opened != want {
		t.Fatalf("opened %q, want %q", opened, want)
	}
	if !strings.Contains(out, "Opened "+want) {
		t.Errorf("missing the 'Opened' confirmation line:\n%s", out)
	}
}

func TestBrowseUpgradesHTTPURLInputToHTTPS(t *testing.T) {
	var opened string
	defer swapBrowseOpener(func(u string) error { opened = u; return nil })()

	if _, err := execRoot(t, jiraInfo(), "browse",
		"http://x.atlassian.net/browse/PROJ-7"); err != nil {
		t.Fatalf("browse: %v", err)
	}
	if opened != "https://x.atlassian.net/browse/PROJ-7" {
		t.Fatalf("opened %q, want the canonical URL rooted at https", opened)
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

func TestBrowseNoBrowserJSONEmitsURLObject(t *testing.T) {
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
	var got struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("browse --json output is not a JSON object: %v\n%s", err, out)
	}
	if got.URL != "https://x.atlassian.net/browse/PROJ-7" {
		t.Fatalf("url = %q, want the canonical browse URL", got.URL)
	}
}

func TestBrowseJSONHonorsFieldSelection(t *testing.T) {
	defer swapBrowseOpener(func(string) error {
		t.Error("browse invoked the opener in --no-browser mode")
		return nil
	})()

	out, err := execRoot(t, jiraInfo(), "browse",
		"https://x.atlassian.net/browse/PROJ-7", "--no-browser", "--json=url")
	if err != nil {
		t.Fatalf("browse --json=url: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("browse --json=url output is not valid JSON: %v\n%s", err, out)
	}
	if len(got) != 1 || got["url"] != "https://x.atlassian.net/browse/PROJ-7" {
		t.Fatalf("field selection result = %v", got)
	}
}

func TestBrowseOpenerFailureIsPropagated(t *testing.T) {
	defer swapBrowseOpener(func(string) error {
		return apperr.New("browser_failed", "no browser available")
	})()

	out, err := execRoot(t, jiraInfo(), "browse", "https://x.atlassian.net/browse/PROJ-7")
	if err == nil {
		t.Fatal("browse did not propagate the opener failure")
	}
	var ae *apperr.Error
	if !errors.As(err, &ae) {
		t.Fatalf("error type = %T, want *apperr.Error", err)
	}
	if strings.Contains(out, "Opened") {
		t.Errorf("browse printed a success line despite the opener failing:\n%s", out)
	}
}

func TestBrowseBareKeyWithoutSiteErrors(t *testing.T) {
	// Isolate config and the ATL_SITE env so the bare key has no site to
	// resolve from any source (flag, env, or default_site).
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv(siteEnvVar, "")
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
