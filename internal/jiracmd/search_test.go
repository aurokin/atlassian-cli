package jiracmd

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
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
