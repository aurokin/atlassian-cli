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

func TestWorklogListHumanOutput(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/issue/PROJ-1/worklog" {
			t.Errorf("path = %q, want /issue/PROJ-1/worklog", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"worklogs":[` +
			`{"id":"10000","author":{"displayName":"Alice"},"timeSpent":"1h","started":"2026-05-19T09:00:00.000-0700"},` +
			`{"id":"10001","author":{"displayName":"Bob"},"timeSpent":"30m","started":"2026-05-19T11:00:00.000-0700"}]}`))
	}))
	defer srv.Close()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginJiraSite(t, srv.URL)

	out, err := execJira(t, "issue", "worklog", "list", "PROJ-1", "--site", "work")
	if err != nil {
		t.Fatalf("worklog list: %v", err)
	}
	for _, want := range []string{"10000", "Alice", "1h", "10001", "Bob", "30m"} {
		if !strings.Contains(out, want) {
			t.Errorf("worklog list output missing %q:\n%s", want, out)
		}
	}
}

func TestWorklogListJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"worklogs":[{"id":"10000"}]}`))
	}))
	defer srv.Close()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginJiraSite(t, srv.URL)

	out, err := execJira(t, "issue", "worklog", "list", "PROJ-1", "--site", "work", "--json")
	if err != nil {
		t.Fatalf("worklog list --json: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("worklog list --json output is not valid JSON: %v\n%s", err, out)
	}
	if _, ok := got["worklogs"]; !ok {
		t.Fatalf("unexpected worklog list JSON: %v", got)
	}
}

func TestWorklogAddHumanOutput(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/issue/PROJ-1/worklog" {
			t.Errorf("path = %q, want /issue/PROJ-1/worklog", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want POST", r.Method)
		}
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_, _ = w.Write([]byte(`{"id":"10005","timeSpent":"3h 30m","timeSpentSeconds":12600}`))
	}))
	defer srv.Close()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginJiraSite(t, srv.URL)

	out, err := execJira(t, "issue", "worklog", "add", "PROJ-1",
		"--time", "3h 30m", "--comment", "deep work", "--site", "work")
	if err != nil {
		t.Fatalf("worklog add: %v", err)
	}
	if gotBody["timeSpent"] != "3h 30m" {
		t.Errorf("worklog add sent timeSpent %v, want 3h 30m (verbatim)", gotBody["timeSpent"])
	}
	// The comment must be sent as an ADF document, not a bare string.
	cm, ok := gotBody["comment"].(map[string]any)
	if !ok {
		t.Fatalf("worklog add comment = %v, want an ADF object", gotBody["comment"])
	}
	if cm["type"] != "doc" {
		t.Errorf("worklog add comment type = %v, want doc", cm["type"])
	}
	if !strings.Contains(out, "logged 3h 30m on PROJ-1") {
		t.Errorf("worklog add output missing 'logged 3h 30m on PROJ-1':\n%s", out)
	}
}

func TestWorklogAddOmitsCommentWhenAbsent(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_, _ = w.Write([]byte(`{"id":"10005","timeSpent":"15m"}`))
	}))
	defer srv.Close()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginJiraSite(t, srv.URL)

	if _, err := execJira(t, "issue", "worklog", "add", "PROJ-1",
		"--time", "15m", "--site", "work"); err != nil {
		t.Fatalf("worklog add: %v", err)
	}
	if _, ok := gotBody["comment"]; ok {
		t.Errorf("worklog add omitted-comment body included comment = %v", gotBody["comment"])
	}
}

func TestWorklogAddRequiresTime(t *testing.T) {
	_, err := execJira(t, "issue", "worklog", "add", "PROJ-1", "--site", "work")
	if err == nil {
		t.Fatal("worklog add without --time returned no error")
	}
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Code != apperr.CodeInvalidInput {
		t.Fatalf("error = %v, want an invalid_input *apperr.Error", err)
	}
}
