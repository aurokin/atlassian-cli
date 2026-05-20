package jiracmd

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestIssueWatch(t *testing.T) {
	var (
		gotMethod string
		gotBody   []byte
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		if r.URL.Path != "/issue/PROJ-1/watchers" {
			t.Errorf("path = %q, want /issue/PROJ-1/watchers", r.URL.Path)
		}
		gotBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginJiraSite(t, srv.URL)

	out, err := execJira(t, "issue", "watch", "PROJ-1", "--site", "work")
	if err != nil {
		t.Fatalf("issue watch: %v", err)
	}
	if gotMethod != http.MethodPost {
		t.Errorf("method = %q, want POST", gotMethod)
	}
	if len(gotBody) != 0 {
		t.Errorf("watch sent body %q, want empty (so Jira adds the caller)", gotBody)
	}
	if !strings.Contains(out, "watching PROJ-1") {
		t.Errorf("watch output missing 'watching PROJ-1':\n%s", out)
	}
}

func TestIssueUnwatch(t *testing.T) {
	var (
		gotMyself bool
		gotMethod string
		gotPath   string
		gotQuery  string
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/myself":
			gotMyself = true
			_, _ = w.Write([]byte(`{"accountId":"caller-id","displayName":"Caller"}`))
		case "/issue/PROJ-1/watchers":
			gotMethod = r.Method
			gotPath = r.URL.Path
			gotQuery = r.URL.RawQuery
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Errorf("unexpected path %q", r.URL.Path)
		}
	}))
	defer srv.Close()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginJiraSite(t, srv.URL)

	out, err := execJira(t, "issue", "unwatch", "PROJ-1", "--site", "work")
	if err != nil {
		t.Fatalf("issue unwatch: %v", err)
	}
	if !gotMyself {
		t.Error("unwatch did not call /myself to look up the calling user")
	}
	if gotMethod != http.MethodDelete {
		t.Errorf("watchers method = %q, want DELETE", gotMethod)
	}
	if gotPath != "/issue/PROJ-1/watchers" {
		t.Errorf("watchers path = %q, want /issue/PROJ-1/watchers", gotPath)
	}
	if gotQuery != "accountId=caller-id" {
		t.Errorf("watchers query = %q, want accountId=caller-id", gotQuery)
	}
	if !strings.Contains(out, "no longer watching PROJ-1") {
		t.Errorf("unwatch output missing 'no longer watching PROJ-1':\n%s", out)
	}
}

func TestIssueWatchersHumanOutput(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/issue/PROJ-1/watchers" {
			t.Errorf("path = %q, want /issue/PROJ-1/watchers", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"isWatching":true,"watchCount":2,"watchers":[` +
			`{"accountId":"u1","displayName":"Alice"},` +
			`{"accountId":"u2","displayName":"Bob"}]}`))
	}))
	defer srv.Close()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginJiraSite(t, srv.URL)

	out, err := execJira(t, "issue", "watchers", "PROJ-1", "--site", "work")
	if err != nil {
		t.Fatalf("issue watchers: %v", err)
	}
	for _, want := range []string{"u1", "Alice", "u2", "Bob"} {
		if !strings.Contains(out, want) {
			t.Errorf("watchers output missing %q:\n%s", want, out)
		}
	}
}

func TestIssueWatchersJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"watchers":[{"accountId":"u1"}]}`))
	}))
	defer srv.Close()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginJiraSite(t, srv.URL)

	out, err := execJira(t, "issue", "watchers", "PROJ-1", "--site", "work", "--json")
	if err != nil {
		t.Fatalf("issue watchers --json: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("watchers --json output is not valid JSON: %v\n%s", err, out)
	}
	if _, ok := got["watchers"]; !ok {
		t.Fatalf("unexpected watchers JSON: %v", got)
	}
}
