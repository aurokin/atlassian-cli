package jiracmd

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aurokin/atlassian-cli/internal/apperr"
	"github.com/aurokin/atlassian-cli/internal/output"
)

func TestSearchIssuesPassesRawJQL(t *testing.T) {
	var gotJQL, gotMax string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotJQL = r.URL.Query().Get("jql")
		gotMax = r.URL.Query().Get("maxResults")
		_, _ = w.Write([]byte(`{"issues":[{"key":"PROJ-7","fields":{"summary":"Found it","status":{"name":"Done"}}}],"isLast":true}`))
	}))
	defer srv.Close()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginJiraSite(t, srv.URL)

	const jql = "assignee = currentUser() AND statusCategory != Done"
	out, err := execJira(t, "search", "issues", jql, "--limit", "10", "--site", "work")
	if err != nil {
		t.Fatalf("search issues: %v", err)
	}
	if gotJQL != jql {
		t.Errorf("search issues sent jql %q, want %q", gotJQL, jql)
	}
	if gotMax != "10" {
		t.Errorf("search issues sent maxResults %q, want 10", gotMax)
	}
	if !strings.Contains(out, "PROJ-7") || !strings.Contains(out, "Found it") {
		t.Fatalf("search issues output:\n%s", out)
	}
}

func TestSearchIssuesRequiresExactlyOneArg(t *testing.T) {
	if _, err := execJira(t, "search", "issues", "--site", "work"); err == nil {
		t.Fatal("search issues with no JQL returned no error")
	}
}

func TestSearchIssuesEmpty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"issues":[],"isLast":true}`))
	}))
	defer srv.Close()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginJiraSite(t, srv.URL)

	out, err := execJira(t, "search", "issues", "project = EMPTY", "--site", "work")
	if err != nil {
		t.Fatalf("search issues: %v", err)
	}
	if !strings.Contains(out, "No issues found") {
		t.Fatalf("empty search output:\n%s", out)
	}
}

func TestSearchIssuesMapsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"errorMessages":["You do not have permission."]}`))
	}))
	defer srv.Close()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginJiraSite(t, srv.URL)

	_, err := execJira(t, "search", "issues", "project = PROJ", "--site", "work")
	if err == nil {
		t.Fatal("search issues against a 403 endpoint returned no error")
	}
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Code != apperr.CodeForbidden {
		t.Fatalf("error = %v, want a forbidden *apperr.Error", err)
	}
}

func TestSearchIssuesJQReachesRenderer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"issues":[],"isLast":true}`))
	}))
	defer srv.Close()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginJiraSite(t, srv.URL)

	// --jq is plumbed through to the shared renderer, which currently reports
	// jq filtering as unimplemented; this guards the --jq render branch.
	_, err := execJira(t, "search", "issues", "project = PROJ", "--site", "work", "--jq", ".")
	if !errors.Is(err, output.ErrJQNotImplemented) {
		t.Fatalf("error = %v, want output.ErrJQNotImplemented", err)
	}
}
