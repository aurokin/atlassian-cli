package bbcmd

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSearchReposHuman(t *testing.T) {
	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repositories/acme" {
			t.Errorf("path = %q", r.URL.Path)
		}
		gotQuery = r.URL.Query().Get("q")
		_, _ = w.Write([]byte(`{"values":[{"full_name":"acme/cli-tools","is_private":true}]}`))
	}))
	defer srv.Close()

	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginBBSite(t, srv.URL)

	out, err := execBB(t, "search", "repos", `name ~ "cli"`, "--workspace", "acme", "--site", "work")
	if err != nil {
		t.Fatalf("search repos: %v\n%s", err, out)
	}
	if gotQuery != `name ~ "cli"` {
		t.Fatalf("q = %q", gotQuery)
	}
	if !strings.Contains(out, "acme/cli-tools") {
		t.Fatalf("output missing repo:\n%s", out)
	}
}

func TestSearchReposRequiresWorkspace(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	_, err := execBB(t, "search", "repos", `name ~ "cli"`, "--site", "work")
	if err == nil || !strings.Contains(err.Error(), "a workspace is required") {
		t.Fatalf("expected workspace-required error, got %v", err)
	}
}

func TestSearchPRsHumanAndSort(t *testing.T) {
	var gotSort string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repositories/acme/widgets/pullrequests" {
			t.Errorf("path = %q", r.URL.Path)
		}
		gotSort = r.URL.Query().Get("sort")
		_, _ = w.Write([]byte(`{"values":[{"id":7,"title":"fix it","state":"OPEN"}]}`))
	}))
	defer srv.Close()

	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginBBSite(t, srv.URL)

	out, err := execBB(t, "search", "prs", `title ~ "fix"`, "--repo", "acme/widgets",
		"--sort", "-updated_on", "--site", "work")
	if err != nil {
		t.Fatalf("search prs: %v\n%s", err, out)
	}
	if gotSort != "-updated_on" {
		t.Fatalf("sort = %q", gotSort)
	}
	if !strings.Contains(out, "fix it") {
		t.Fatalf("output missing PR:\n%s", out)
	}
}

func TestSearchIssuesHumanAndJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repositories/acme/widgets/issues" {
			t.Errorf("path = %q", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"values":[{"id":3,"title":"a bug","state":"open"}]}`))
	}))
	defer srv.Close()

	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginBBSite(t, srv.URL)

	out, err := execBB(t, "search", "issues", `title ~ "bug"`, "--repo", "acme/widgets", "--site", "work")
	if err != nil {
		t.Fatalf("search issues: %v\n%s", err, out)
	}
	if !strings.Contains(out, "a bug") {
		t.Fatalf("output missing issue:\n%s", out)
	}

	jsonOut, err := execBB(t, "search", "issues", `title ~ "bug"`, "--repo", "acme/widgets", "--site", "work", "--jq", ".values[0].id")
	if err != nil {
		t.Fatalf("search issues --jq: %v\n%s", err, jsonOut)
	}
	if strings.TrimSpace(jsonOut) != "3" {
		t.Fatalf("jq output = %q", jsonOut)
	}
}

func TestSearchRequiresQuery(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	_, err := execBB(t, "search", "repos", "  ", "--workspace", "acme", "--site", "work")
	if err == nil || !strings.Contains(err.Error(), "a query is required") {
		t.Fatalf("expected query-required error, got %v", err)
	}
}
