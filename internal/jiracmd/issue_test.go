package jiracmd

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aurokin/atlassian-cli/internal/apperr"
)

func TestBuildIssueListJQL(t *testing.T) {
	cases := []struct {
		name                      string
		project, status, assignee string
		want                      string
	}{
		{"project only", "PROJ", "", "",
			`project = "PROJ" ORDER BY created DESC`},
		{"with status", "PROJ", "In Progress", "",
			`project = "PROJ" AND status = "In Progress" ORDER BY created DESC`},
		{"with assignee account id", "PROJ", "", "5b10a2",
			`project = "PROJ" AND assignee = "5b10a2" ORDER BY created DESC`},
		{"currentUser is unquoted", "PROJ", "", "currentUser()",
			`project = "PROJ" AND assignee = currentUser() ORDER BY created DESC`},
		{"all filters", "PROJ", "Done", "currentUser()",
			`project = "PROJ" AND status = "Done" AND assignee = currentUser() ORDER BY created DESC`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := buildIssueListJQL(tc.project, tc.status, tc.assignee); got != tc.want {
				t.Errorf("buildIssueListJQL = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestJQLQuoteEscapesSpecialCharacters(t *testing.T) {
	if got := jqlQuote(`a"b\c`); got != `"a\"b\\c"` {
		t.Errorf("jqlQuote = %q", got)
	}
}

func TestIssueViewHumanOutput(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"key":"PROJ-1","fields":{"summary":"Fix the bug","status":{"name":"In Progress"},"issuetype":{"name":"Bug"},"assignee":{"displayName":"Ada Lovelace"}}}`))
	}))
	defer srv.Close()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginJiraSite(t, srv.URL)

	out, err := execJira(t, "issue", "view", "PROJ-1", "--site", "work")
	if err != nil {
		t.Fatalf("issue view: %v", err)
	}
	for _, want := range []string{"PROJ-1", "Fix the bug", "In Progress", "Bug", "Ada Lovelace"} {
		if !strings.Contains(out, want) {
			t.Errorf("issue view output missing %q:\n%s", want, out)
		}
	}
}

func TestIssueViewJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"key":"PROJ-1","fields":{"summary":"Fix the bug"}}`))
	}))
	defer srv.Close()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginJiraSite(t, srv.URL)

	out, err := execJira(t, "issue", "view", "PROJ-1", "--site", "work", "--json")
	if err != nil {
		t.Fatalf("issue view --json: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("issue view --json output is not valid JSON: %v\n%s", err, out)
	}
	if got["key"] != "PROJ-1" {
		t.Fatalf("unexpected issue JSON: %v", got)
	}
}

func TestIssueListBuildsJQLFromFlags(t *testing.T) {
	var gotJQL, gotMax string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotJQL = r.URL.Query().Get("jql")
		gotMax = r.URL.Query().Get("maxResults")
		_, _ = w.Write([]byte(`{"issues":[{"key":"PROJ-1","fields":{"summary":"First","status":{"name":"To Do"}}}],"isLast":true}`))
	}))
	defer srv.Close()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginJiraSite(t, srv.URL)

	out, err := execJira(t, "issue", "list", "--project", "PROJ", "--status", "To Do", "--limit", "7", "--site", "work")
	if err != nil {
		t.Fatalf("issue list: %v", err)
	}
	if gotJQL != `project = "PROJ" AND status = "To Do" ORDER BY created DESC` {
		t.Fatalf("issue list sent jql %q", gotJQL)
	}
	if gotMax != "7" {
		t.Errorf("issue list sent maxResults %q, want 7 (--limit not plumbed)", gotMax)
	}
	if !strings.Contains(out, "PROJ-1") || !strings.Contains(out, "First") {
		t.Fatalf("issue list output:\n%s", out)
	}
}

func TestIssueViewJSONFieldSelection(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"id":"1","key":"PROJ-1","fields":{"summary":"Fix the bug"}}`))
	}))
	defer srv.Close()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginJiraSite(t, srv.URL)

	out, err := execJira(t, "issue", "view", "PROJ-1", "--site", "work", "--json=key")
	if err != nil {
		t.Fatalf("issue view --json=key: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("issue view --json=key output is not valid JSON: %v\n%s", err, out)
	}
	if len(got) != 1 || got["key"] != "PROJ-1" {
		t.Fatalf("field selection result = %v, want only {key: PROJ-1}", got)
	}
}

func TestIssueListRequiresProject(t *testing.T) {
	_, err := execJira(t, "issue", "list", "--site", "work")
	if err == nil {
		t.Fatal("issue list without --project returned no error")
	}
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Code != apperr.CodeInvalidInput {
		t.Fatalf("error = %v, want an invalid_input *apperr.Error", err)
	}
}

func TestIssueViewMapsNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"errorMessages":["Issue does not exist or you do not have permission to see it."]}`))
	}))
	defer srv.Close()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginJiraSite(t, srv.URL)

	_, err := execJira(t, "issue", "view", "PROJ-404", "--site", "work")
	if err == nil {
		t.Fatal("issue view of a missing issue returned no error")
	}
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Code != apperr.CodeNotFoundOrNotVisible {
		t.Fatalf("error = %v, want a not_found_or_not_visible *apperr.Error", err)
	}
}
