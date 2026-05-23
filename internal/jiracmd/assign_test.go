package jiracmd

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestIssueAssign(t *testing.T) {
	var (
		gotMethod string
		gotBody   map[string]any
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		if r.URL.Path != "/issue/PROJ-1/assignee" {
			t.Errorf("path = %q, want /issue/PROJ-1/assignee", r.URL.Path)
		}
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginJiraSite(t, srv.URL)

	out, err := execJira(t, "issue", "assign", "PROJ-1", "abc123", "--site", "work")
	if err != nil {
		t.Fatalf("issue assign: %v", err)
	}
	if gotMethod != http.MethodPut {
		t.Errorf("method = %q, want PUT", gotMethod)
	}
	if gotBody["accountId"] != "abc123" {
		t.Errorf("assign sent accountId %v, want abc123", gotBody["accountId"])
	}
	if !strings.Contains(out, "assigned PROJ-1 to abc123") {
		t.Errorf("assign output missing 'assigned PROJ-1 to abc123':\n%s", out)
	}
}

func TestIssueAssignJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginJiraSite(t, srv.URL)

	out, err := execJira(t, "issue", "assign", "PROJ-1", "abc123", "--site", "work", "--json")
	if err != nil {
		t.Fatalf("issue assign --json: %v\n%s", err, out)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, out)
	}
	if got["issue"] != "PROJ-1" || got["assignee"] != "abc123" || got["assigned"] != true {
		t.Fatalf("unexpected assign json: %s", out)
	}
}

func TestIssueUnassignJSONHasNullAssignee(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginJiraSite(t, srv.URL)

	out, err := execJira(t, "issue", "assign", "PROJ-1", "-", "--site", "work", "--json")
	if err != nil {
		t.Fatalf("issue assign - --json: %v\n%s", err, out)
	}
	var got map[string]json.RawMessage
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, out)
	}
	if string(got["assignee"]) != "null" {
		t.Errorf("assignee = %s, want null", got["assignee"])
	}
	if string(got["assigned"]) != "false" {
		t.Errorf("assigned = %s, want false", got["assigned"])
	}
}

func TestIssueAssignAtMe(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/myself":
			_, _ = w.Write([]byte(`{"accountId":"me-123","displayName":"Me"}`))
		case "/issue/PROJ-1/assignee":
			_ = json.NewDecoder(r.Body).Decode(&gotBody)
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Errorf("unexpected path %q", r.URL.Path)
		}
	}))
	defer srv.Close()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginJiraSite(t, srv.URL)

	out, err := execJira(t, "issue", "assign", "PROJ-1", "@me", "--site", "work")
	if err != nil {
		t.Fatalf("issue assign @me: %v\n%s", err, out)
	}
	if gotBody["accountId"] != "me-123" {
		t.Errorf("assign sent accountId %v, want me-123", gotBody["accountId"])
	}
	if !strings.Contains(out, "assigned PROJ-1 to me-123") {
		t.Errorf("assign output missing resolved id:\n%s", out)
	}
}

func TestIssueAssignByEmail(t *testing.T) {
	var (
		gotQuery string
		gotBody  map[string]any
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/user/search":
			gotQuery = r.URL.Query().Get("query")
			_, _ = w.Write([]byte(`[{"accountId":"ada-1","emailAddress":"ada@example.com"}]`))
		case "/issue/PROJ-1/assignee":
			_ = json.NewDecoder(r.Body).Decode(&gotBody)
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Errorf("unexpected path %q", r.URL.Path)
		}
	}))
	defer srv.Close()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginJiraSite(t, srv.URL)

	out, err := execJira(t, "issue", "assign", "PROJ-1", "ada@example.com", "--site", "work")
	if err != nil {
		t.Fatalf("issue assign by email: %v\n%s", err, out)
	}
	if gotQuery != "ada@example.com" {
		t.Errorf("user search query = %q, want ada@example.com", gotQuery)
	}
	if gotBody["accountId"] != "ada-1" {
		t.Errorf("assign sent accountId %v, want ada-1", gotBody["accountId"])
	}
}

func TestIssueAssignByEmailAmbiguous(t *testing.T) {
	assigned := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/user/search":
			_, _ = w.Write([]byte(`[{"accountId":"a1"},{"accountId":"a2"}]`))
		case "/issue/PROJ-1/assignee":
			assigned = true
			w.WriteHeader(http.StatusNoContent)
		}
	}))
	defer srv.Close()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginJiraSite(t, srv.URL)

	out, err := execJira(t, "issue", "assign", "PROJ-1", "dup@example.com", "--site", "work")
	if err == nil {
		t.Fatalf("expected error for ambiguous match, got none:\n%s", out)
	}
	if !strings.Contains(err.Error(), "matched 2 Jira users") {
		t.Errorf("error = %v, want ambiguity message", err)
	}
	if assigned {
		t.Error("assignee endpoint was called despite ambiguous resolution")
	}
}

func TestIssueAssignByEmailNoMatch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/user/search" {
			_, _ = w.Write([]byte(`[]`))
			return
		}
		t.Errorf("unexpected path %q", r.URL.Path)
	}))
	defer srv.Close()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginJiraSite(t, srv.URL)

	out, err := execJira(t, "issue", "assign", "PROJ-1", "ghost@example.com", "--site", "work")
	if err == nil {
		t.Fatalf("expected error for no match, got none:\n%s", out)
	}
	if !strings.Contains(err.Error(), "no Jira user matched") {
		t.Errorf("error = %v, want no-match message", err)
	}
}

func TestIssueAssignUnassign(t *testing.T) {
	var rawBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rawBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginJiraSite(t, srv.URL)

	out, err := execJira(t, "issue", "assign", "PROJ-1", "-", "--site", "work")
	if err != nil {
		t.Fatalf("issue assign -: %v", err)
	}
	// The raw body must carry a JSON null, not the literal string "-" or an
	// omitted key. Decode it and inspect the value's JSON form.
	var got map[string]json.RawMessage
	if err := json.Unmarshal(rawBody, &got); err != nil {
		t.Fatalf("assign body is not JSON: %v\n%s", err, rawBody)
	}
	v, ok := got["accountId"]
	if !ok {
		t.Fatalf("assign body missing accountId key: %s", rawBody)
	}
	if string(v) != "null" {
		t.Errorf("assign body accountId = %s, want null", v)
	}
	if !strings.Contains(out, "unassigned PROJ-1") {
		t.Errorf("unassign output missing 'unassigned PROJ-1':\n%s", out)
	}
}
