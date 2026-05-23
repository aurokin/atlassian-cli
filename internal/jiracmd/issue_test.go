package jiracmd

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aurokin/atlassian-cli/internal/apperr"
	"github.com/aurokin/atlassian-cli/internal/jira"
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
		{"@me maps to currentUser", "PROJ", "", "@me",
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

func TestIssueListResolvesEmailAssigneeToAccountID(t *testing.T) {
	var gotJQL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/user/search":
			if q := r.URL.Query().Get("query"); q != "ada@example.com" {
				t.Errorf("user search query = %q, want ada@example.com", q)
			}
			_, _ = w.Write([]byte(`[{"accountId":"ada-1"}]`))
		case "/search/jql":
			gotJQL = r.URL.Query().Get("jql")
			_, _ = w.Write([]byte(`{"issues":[],"isLast":true}`))
		default:
			t.Errorf("unexpected path %q", r.URL.Path)
		}
	}))
	defer srv.Close()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginJiraSite(t, srv.URL)

	out, err := execJira(t, "issue", "list", "--project", "PROJ", "--assignee", "ada@example.com", "--site", "work")
	if err != nil {
		t.Fatalf("issue list --assignee email: %v\n%s", err, out)
	}
	if gotJQL != `project = "PROJ" AND assignee = "ada-1" ORDER BY created DESC` {
		t.Fatalf("issue list sent jql %q, want resolved account id", gotJQL)
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

func TestIssueCreateHumanOutput(t *testing.T) {
	var gotMethod, gotPath, gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath = r.Method, r.URL.Path
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"1001","key":"PROJ-9"}`))
	}))
	defer srv.Close()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginJiraSite(t, srv.URL)

	out, err := execJira(t, "issue", "create", "--project", "PROJ", "--type", "Bug",
		"--summary", "It broke", "--description", "Steps to reproduce", "--site", "work")
	if err != nil {
		t.Fatalf("issue create: %v", err)
	}
	if gotMethod != http.MethodPost || gotPath != "/issue" {
		t.Errorf("request = %s %s, want POST /issue", gotMethod, gotPath)
	}
	for _, want := range []string{`"key":"PROJ"`, `"name":"Bug"`, `"summary":"It broke"`} {
		if !strings.Contains(gotBody, want) {
			t.Errorf("create request body missing %s:\n%s", want, gotBody)
		}
	}
	// A plain --description is wrapped as an ADF document.
	if !strings.Contains(gotBody, `"description":{"type":"doc"`) ||
		!strings.Contains(gotBody, `"text":"Steps to reproduce"`) {
		t.Errorf("--description not wrapped as ADF:\n%s", gotBody)
	}
	if !strings.Contains(out, "PROJ-9") {
		t.Errorf("issue create output missing the new key:\n%s", out)
	}
}

func TestIssueCreateJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"1001","key":"PROJ-9"}`))
	}))
	defer srv.Close()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginJiraSite(t, srv.URL)

	out, err := execJira(t, "issue", "create", "--project", "PROJ", "--type", "Bug",
		"--summary", "S", "--site", "work", "--json")
	if err != nil {
		t.Fatalf("issue create --json: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("issue create --json output is not valid JSON: %v\n%s", err, out)
	}
	if got["key"] != "PROJ-9" {
		t.Fatalf("unexpected create JSON: %v", got)
	}
}

func TestIssueCreateResolvesAtMeAssignee(t *testing.T) {
	var gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/myself":
			_, _ = w.Write([]byte(`{"accountId":"me-123"}`))
		case "/issue":
			b, _ := io.ReadAll(r.Body)
			gotBody = string(b)
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"key":"PROJ-9"}`))
		default:
			t.Errorf("unexpected path %q", r.URL.Path)
		}
	}))
	defer srv.Close()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginJiraSite(t, srv.URL)

	out, err := execJira(t, "issue", "create", "--project", "PROJ", "--type", "Bug",
		"--summary", "S", "--assignee", "@me", "--site", "work")
	if err != nil {
		t.Fatalf("issue create --assignee @me: %v\n%s", err, out)
	}
	if !strings.Contains(gotBody, `"assignee":{"accountId":"me-123"}`) {
		t.Errorf("create body missing resolved assignee:\n%s", gotBody)
	}
}

func TestIssueCreateRequiresFlags(t *testing.T) {
	_, err := execJira(t, "issue", "create", "--project", "PROJ", "--site", "work")
	if err == nil {
		t.Fatal("issue create without --type and --summary returned no error")
	}
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Code != apperr.CodeInvalidInput {
		t.Fatalf("error = %v, want an invalid_input *apperr.Error", err)
	}
}

func TestIssueCreateParsesFieldFlag(t *testing.T) {
	var gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"key":"PROJ-9"}`))
	}))
	defer srv.Close()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginJiraSite(t, srv.URL)

	_, err := execJira(t, "issue", "create", "--project", "PROJ", "--type", "Task",
		"--summary", "S", "--field", `labels=["urgent","ops"]`, "--field", "note=hello",
		"--site", "work")
	if err != nil {
		t.Fatalf("issue create --field: %v", err)
	}
	// A valid-JSON value is sent as parsed JSON; a plain value stays a string.
	if !strings.Contains(gotBody, `"labels":["urgent","ops"]`) {
		t.Errorf("--field JSON value not parsed as an array:\n%s", gotBody)
	}
	if !strings.Contains(gotBody, `"note":"hello"`) {
		t.Errorf("--field plain value not kept as a string:\n%s", gotBody)
	}
}

func TestIssueCreateRejectsMalformedFieldFlag(t *testing.T) {
	_, err := execJira(t, "issue", "create", "--project", "PROJ", "--type", "Task",
		"--summary", "S", "--field", "noequalsign", "--site", "work")
	if err == nil {
		t.Fatal("issue create with a malformed --field returned no error")
	}
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Code != apperr.CodeInvalidInput {
		t.Fatalf("error = %v, want an invalid_input *apperr.Error", err)
	}
}

func TestIssueEditHumanOutput(t *testing.T) {
	var gotMethod, gotPath, gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath = r.Method, r.URL.Path
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginJiraSite(t, srv.URL)

	out, err := execJira(t, "issue", "edit", "PROJ-1", "--summary", "Renamed", "--site", "work")
	if err != nil {
		t.Fatalf("issue edit: %v", err)
	}
	if gotMethod != http.MethodPut || gotPath != "/issue/PROJ-1" {
		t.Errorf("request = %s %s, want PUT /issue/PROJ-1", gotMethod, gotPath)
	}
	if !strings.Contains(gotBody, `"summary":"Renamed"`) {
		t.Errorf("edit request body missing the new summary:\n%s", gotBody)
	}
	if !strings.Contains(out, "updated PROJ-1") {
		t.Errorf("issue edit output:\n%s", out)
	}
}

func TestIssueEditJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginJiraSite(t, srv.URL)

	out, err := execJira(t, "issue", "edit", "PROJ-1", "--summary", "X", "--site", "work", "--json")
	if err != nil {
		t.Fatalf("issue edit --json: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("issue edit --json output is not valid JSON: %v\n%s", err, out)
	}
	if got["key"] != "PROJ-1" || got["updated"] != true {
		t.Fatalf("unexpected edit JSON: %v", got)
	}
}

func TestIssueEditRequiresAChange(t *testing.T) {
	_, err := execJira(t, "issue", "edit", "PROJ-1", "--site", "work")
	if err == nil {
		t.Fatal("issue edit with no field flags returned no error")
	}
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Code != apperr.CodeInvalidInput {
		t.Fatalf("error = %v, want an invalid_input *apperr.Error", err)
	}
}

func TestIssueEditMapsNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"errorMessages":["Issue does not exist."]}`))
	}))
	defer srv.Close()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginJiraSite(t, srv.URL)

	_, err := execJira(t, "issue", "edit", "PROJ-404", "--summary", "X", "--site", "work")
	if err == nil {
		t.Fatal("issue edit of a missing issue returned no error")
	}
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Code != apperr.CodeNotFoundOrNotVisible {
		t.Fatalf("error = %v, want a not_found_or_not_visible *apperr.Error", err)
	}
}

// transitionServer replies to the GET transitions request with a fixed list
// and records the POST apply request's path and body.
func transitionServer(gotPath, gotBody *string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			_, _ = w.Write([]byte(
				`{"transitions":[{"id":"31","name":"Done"},{"id":"11","name":"To Do"}]}`))
			return
		}
		b, _ := io.ReadAll(r.Body)
		*gotPath, *gotBody = r.URL.Path, string(b)
		w.WriteHeader(http.StatusNoContent)
	}))
}

func TestIssueTransitionListsTransitions(t *testing.T) {
	var path, body string
	srv := transitionServer(&path, &body)
	defer srv.Close()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginJiraSite(t, srv.URL)

	out, err := execJira(t, "issue", "transition", "PROJ-1", "--site", "work")
	if err != nil {
		t.Fatalf("issue transition: %v", err)
	}
	for _, want := range []string{"31", "Done", "11", "To Do"} {
		if !strings.Contains(out, want) {
			t.Errorf("transition list output missing %q:\n%s", want, out)
		}
	}
	if body != "" {
		t.Errorf("listing transitions should not POST; sent body %q", body)
	}
}

func TestIssueTransitionAppliesByName(t *testing.T) {
	var path, body string
	srv := transitionServer(&path, &body)
	defer srv.Close()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginJiraSite(t, srv.URL)

	// A case-insensitive name match resolves to the transition id.
	out, err := execJira(t, "issue", "transition", "PROJ-1", "--to", "done", "--site", "work")
	if err != nil {
		t.Fatalf("issue transition --to: %v", err)
	}
	if path != "/issue/PROJ-1/transitions" {
		t.Errorf("apply path = %q, want /issue/PROJ-1/transitions", path)
	}
	if body != `{"transition":{"id":"31"}}` {
		t.Errorf("apply request body = %q, want the resolved transition id", body)
	}
	if !strings.Contains(out, `transitioned PROJ-1 to "Done"`) {
		t.Errorf("issue transition output:\n%s", out)
	}
}

func TestIssueTransitionAppliesByID(t *testing.T) {
	var path, body string
	srv := transitionServer(&path, &body)
	defer srv.Close()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginJiraSite(t, srv.URL)

	if _, err := execJira(t, "issue", "transition", "PROJ-1", "--to", "11", "--site", "work"); err != nil {
		t.Fatalf("issue transition --to id: %v", err)
	}
	if body != `{"transition":{"id":"11"}}` {
		t.Errorf("apply request body = %q, want id 11", body)
	}
}

func TestIssueTransitionJSON(t *testing.T) {
	var path, body string
	srv := transitionServer(&path, &body)
	defer srv.Close()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginJiraSite(t, srv.URL)

	out, err := execJira(t, "issue", "transition", "PROJ-1", "--to", "Done", "--site", "work", "--json")
	if err != nil {
		t.Fatalf("issue transition --json: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("issue transition --json output is not valid JSON: %v\n%s", err, out)
	}
	if got["key"] != "PROJ-1" || got["transition"] != "Done" {
		t.Fatalf("unexpected transition JSON: %v", got)
	}
}

func TestIssueTransitionRejectsUnknownTarget(t *testing.T) {
	var path, body string
	srv := transitionServer(&path, &body)
	defer srv.Close()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginJiraSite(t, srv.URL)

	_, err := execJira(t, "issue", "transition", "PROJ-1", "--to", "Nonexistent", "--site", "work")
	if err == nil {
		t.Fatal("issue transition to an unknown target returned no error")
	}
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Code != apperr.CodeInvalidInput {
		t.Fatalf("error = %v, want an invalid_input *apperr.Error", err)
	}
	if body != "" {
		t.Errorf("an unresolved transition should not POST; sent body %q", body)
	}
}

func TestIssueTransitionMapsNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"errorMessages":["Issue does not exist."]}`))
	}))
	defer srv.Close()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginJiraSite(t, srv.URL)

	_, err := execJira(t, "issue", "transition", "PROJ-404", "--site", "work")
	if err == nil {
		t.Fatal("issue transition of a missing issue returned no error")
	}
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Code != apperr.CodeNotFoundOrNotVisible {
		t.Fatalf("error = %v, want a not_found_or_not_visible *apperr.Error", err)
	}
}

func TestResolveTransitionMatches(t *testing.T) {
	available := []jira.Transition{
		{ID: "31", Name: "Done"},
		{ID: "11", Name: "To Do"},
	}
	cases := []struct {
		name, to, wantID string
	}{
		{"by id", "31", "31"},
		{"by exact name", "Done", "31"},
		{"by case-insensitive name", "to do", "11"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tr, err := resolveTransition("PROJ-1", available, tc.to)
			if err != nil {
				t.Fatalf("resolveTransition: %v", err)
			}
			if tr.ID != tc.wantID {
				t.Errorf("resolved id = %q, want %q", tr.ID, tc.wantID)
			}
		})
	}
}

func TestResolveTransitionErrors(t *testing.T) {
	cases := []struct {
		name        string
		transitions []jira.Transition
		to          string
	}{
		{"empty list", nil, "Done"},
		{"no match", []jira.Transition{{ID: "31", Name: "Done"}}, "Nope"},
		{"ambiguous", []jira.Transition{{ID: "5", Name: "Review"}, {ID: "9", Name: "review"}}, "Review"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := resolveTransition("PROJ-1", tc.transitions, tc.to)
			if err == nil {
				t.Fatal("resolveTransition returned no error")
			}
			var ae *apperr.Error
			if !errors.As(err, &ae) || ae.Code != apperr.CodeInvalidInput {
				t.Fatalf("error = %v, want an invalid_input *apperr.Error", err)
			}
		})
	}
}

func TestIssueCreateFieldOverridesTypedFlag(t *testing.T) {
	var gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"key":"PROJ-9"}`))
	}))
	defer srv.Close()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginJiraSite(t, srv.URL)

	// The repeatable --field escape is overlaid last, so its value wins.
	_, err := execJira(t, "issue", "create", "--project", "PROJ", "--type", "Task",
		"--summary", "from typed flag", "--field", "summary=from field flag", "--site", "work")
	if err != nil {
		t.Fatalf("issue create: %v", err)
	}
	if !strings.Contains(gotBody, `"summary":"from field flag"`) {
		t.Errorf("--field did not override the typed --summary flag:\n%s", gotBody)
	}
	if strings.Contains(gotBody, "from typed flag") {
		t.Errorf("typed --summary should have been overridden:\n%s", gotBody)
	}
}
