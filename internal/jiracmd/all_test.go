package jiracmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// The --all flag follows pagination to completion; each test serves two pages
// and asserts the command emitted the aggregated result.

func TestProjectListAllFollowsPages(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Query().Get("startAt") {
		case "", "0":
			_, _ = w.Write([]byte(`{"startAt":0,"maxResults":1,"total":2,"isLast":false,` +
				`"values":[{"key":"AA","name":"Apple"}]}`))
		case "1":
			_, _ = w.Write([]byte(`{"startAt":1,"maxResults":1,"total":2,"isLast":true,` +
				`"values":[{"key":"BB","name":"Banana"}]}`))
		default:
			t.Errorf("unexpected startAt %q", r.URL.Query().Get("startAt"))
		}
	}))
	defer srv.Close()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginJiraSite(t, srv.URL)

	out, err := execJira(t, "project", "list", "--all", "--limit", "1", "--site", "work", "--json")
	if err != nil {
		t.Fatalf("project list --all: %v", err)
	}
	var got struct {
		Values []json.RawMessage `json:"values"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("project list --all output is not valid JSON: %v\n%s", err, out)
	}
	if len(got.Values) != 2 {
		t.Fatalf("aggregated %d projects, want 2", len(got.Values))
	}
}

func TestSearchIssuesAllFollowsPages(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Query().Get("nextPageToken") {
		case "":
			_, _ = w.Write([]byte(`{"issues":[{"key":"Q-1"}],"nextPageToken":"n2"}`))
		case "n2":
			_, _ = w.Write([]byte(`{"issues":[{"key":"Q-2"}]}`))
		default:
			t.Errorf("unexpected nextPageToken %q", r.URL.Query().Get("nextPageToken"))
		}
	}))
	defer srv.Close()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginJiraSite(t, srv.URL)

	out, err := execJira(t, "search", "issues", "project = Q", "--all", "--site", "work", "--json")
	if err != nil {
		t.Fatalf("search issues --all: %v", err)
	}
	var got struct {
		Issues []json.RawMessage `json:"issues"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("search issues --all output is not valid JSON: %v\n%s", err, out)
	}
	if len(got.Issues) != 2 {
		t.Fatalf("aggregated %d issues, want 2", len(got.Issues))
	}
}

func TestIssueListAllFollowsPages(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Query().Get("nextPageToken") {
		case "":
			_, _ = w.Write([]byte(`{"issues":[{"key":"Q-1"}],"nextPageToken":"n2"}`))
		case "n2":
			_, _ = w.Write([]byte(`{"issues":[{"key":"Q-2"}]}`))
		default:
			t.Errorf("unexpected nextPageToken %q", r.URL.Query().Get("nextPageToken"))
		}
	}))
	defer srv.Close()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginJiraSite(t, srv.URL)

	out, err := execJira(t, "issue", "list", "--project", "Q", "--all", "--site", "work", "--json")
	if err != nil {
		t.Fatalf("issue list --all: %v", err)
	}
	var got struct {
		Issues []json.RawMessage `json:"issues"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("issue list --all output is not valid JSON: %v\n%s", err, out)
	}
	if len(got.Issues) != 2 {
		t.Fatalf("aggregated %d issues, want 2", len(got.Issues))
	}
}

func TestWorklogListAllFollowsPages(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/issue/Q-1/worklog" {
			t.Errorf("path = %q, want /issue/Q-1/worklog", r.URL.Path)
		}
		switch r.URL.Query().Get("startAt") {
		case "", "0":
			_, _ = w.Write([]byte(`{"startAt":0,"maxResults":1,"total":2,"worklogs":[{"id":"10000"}]}`))
		case "1":
			_, _ = w.Write([]byte(`{"startAt":1,"maxResults":1,"total":2,"worklogs":[{"id":"10001"}]}`))
		default:
			t.Errorf("unexpected startAt %q", r.URL.Query().Get("startAt"))
		}
	}))
	defer srv.Close()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginJiraSite(t, srv.URL)

	out, err := execJira(t, "issue", "worklog", "list", "Q-1", "--all", "--limit", "1", "--site", "work", "--json")
	if err != nil {
		t.Fatalf("worklog list --all: %v", err)
	}
	var got struct {
		Worklogs []json.RawMessage `json:"worklogs"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("worklog list --all output is not valid JSON: %v\n%s", err, out)
	}
	if len(got.Worklogs) != 2 {
		t.Fatalf("aggregated %d worklogs, want 2", len(got.Worklogs))
	}
}

func TestCommentListAllFollowsPages(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Query().Get("startAt") {
		case "", "0":
			_, _ = w.Write([]byte(`{"startAt":0,"maxResults":1,"total":2,"comments":[{"id":"c1"}]}`))
		case "1":
			_, _ = w.Write([]byte(`{"startAt":1,"maxResults":1,"total":2,"comments":[{"id":"c2"}]}`))
		default:
			t.Errorf("unexpected startAt %q", r.URL.Query().Get("startAt"))
		}
	}))
	defer srv.Close()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginJiraSite(t, srv.URL)

	out, err := execJira(t, "issue", "comment", "list", "Q-1", "--all", "--limit", "1", "--site", "work", "--json")
	if err != nil {
		t.Fatalf("comment list --all: %v", err)
	}
	var got struct {
		Comments []json.RawMessage `json:"comments"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("comment list --all output is not valid JSON: %v\n%s", err, out)
	}
	if len(got.Comments) != 2 {
		t.Fatalf("aggregated %d comments, want 2", len(got.Comments))
	}
}
