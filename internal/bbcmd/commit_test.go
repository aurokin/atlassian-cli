package bbcmd

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCommitListHuman(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repositories/acme/widgets/commits" {
			t.Errorf("path = %q", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"values":[` +
			`{"hash":"abcdef1234567890aa","message":"first line\nbody","author":{"raw":"Auro <a@x>","user":{"display_name":"Auro"}}},` +
			`{"hash":"0123456789abcdef","summary":{"raw":"second commit"}}]}`))
	}))
	defer srv.Close()

	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginBBSite(t, srv.URL)

	out, err := execBB(t, "commit", "list", "--repo", "acme/widgets", "--site", "work")
	if err != nil {
		t.Fatalf("commit list: %v\n%s", err, out)
	}
	for _, want := range []string{"abcdef123456", "first line", "Auro", "0123456789ab", "second commit"} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q:\n%s", want, out)
		}
	}
	// Only the first line of a multi-line message should appear in the row.
	if strings.Contains(out, "body") {
		t.Fatalf("commit row should show only the first message line:\n%s", out)
	}
}

func TestCommitListRevisionFlag(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repositories/acme/widgets/commits/develop" {
			t.Errorf("path = %q", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"values":[]}`))
	}))
	defer srv.Close()

	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginBBSite(t, srv.URL)

	out, err := execBB(t, "commit", "list", "--repo", "acme/widgets", "--revision", "develop", "--site", "work")
	if err != nil {
		t.Fatalf("commit list --revision: %v\n%s", err, out)
	}
	if !strings.Contains(out, "No commits found.") {
		t.Fatalf("unexpected output:\n%s", out)
	}
}

func TestCommitViewHumanAndJSON(t *testing.T) {
	body := `{"hash":"abc123","date":"2026-01-01T00:00:00Z","message":"fix the bug","author":{"raw":"Auro <a@x>"}}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repositories/acme/widgets/commit/abc123" {
			t.Errorf("path = %q", r.URL.Path)
		}
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()

	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginBBSite(t, srv.URL)

	out, err := execBB(t, "commit", "view", "abc123", "--repo", "acme/widgets", "--site", "work")
	if err != nil {
		t.Fatalf("commit view: %v\n%s", err, out)
	}
	for _, want := range []string{"abc123", "fix the bug", "Auro"} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q:\n%s", want, out)
		}
	}

	jsonOut, err := execBB(t, "commit", "view", "abc123", "--repo", "acme/widgets", "--site", "work", "--jq", ".hash")
	if err != nil {
		t.Fatalf("commit view --jq: %v\n%s", err, jsonOut)
	}
	if strings.TrimSpace(jsonOut) != `"abc123"` {
		t.Fatalf("jq output = %q", jsonOut)
	}
}

func TestCommitViewRequiresRepo(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	_, err := execBB(t, "commit", "view", "abc123", "--site", "work")
	if err == nil || !strings.Contains(err.Error(), "a repository is required") {
		t.Fatalf("expected repo-required error, got %v", err)
	}
}
