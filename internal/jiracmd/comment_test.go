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

const adfComment = `{"id":"10","author":{"displayName":"Ada Lovelace"},` +
	`"created":"2026-05-18T10:00:00.000+0000",` +
	`"body":{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Looks good to me"}]}]}}`

func TestCommentListHumanOutput(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/issue/PROJ-1/comment" {
			t.Errorf("path = %q, want /issue/PROJ-1/comment", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"comments":[` + adfComment + `],"total":1}`))
	}))
	defer srv.Close()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginJiraSite(t, srv.URL)

	out, err := execJira(t, "issue", "comment", "list", "PROJ-1", "--site", "work")
	if err != nil {
		t.Fatalf("issue comment list: %v", err)
	}
	for _, want := range []string{"Ada Lovelace", "Looks good to me"} {
		if !strings.Contains(out, want) {
			t.Errorf("comment list output missing %q:\n%s", want, out)
		}
	}
}

func TestCommentListEmpty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"comments":[],"total":0}`))
	}))
	defer srv.Close()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginJiraSite(t, srv.URL)

	out, err := execJira(t, "issue", "comment", "list", "PROJ-1", "--site", "work")
	if err != nil {
		t.Fatalf("issue comment list: %v", err)
	}
	if !strings.Contains(out, "No comments found") {
		t.Fatalf("empty comment list output:\n%s", out)
	}
}

func TestCommentViewHumanOutput(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/issue/PROJ-1/comment/10" {
			t.Errorf("path = %q, want /issue/PROJ-1/comment/10", r.URL.Path)
		}
		_, _ = w.Write([]byte(adfComment))
	}))
	defer srv.Close()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginJiraSite(t, srv.URL)

	out, err := execJira(t, "issue", "comment", "view", "PROJ-1", "10", "--site", "work")
	if err != nil {
		t.Fatalf("issue comment view: %v", err)
	}
	for _, want := range []string{"10", "Ada Lovelace", "Looks good to me"} {
		if !strings.Contains(out, want) {
			t.Errorf("comment view output missing %q:\n%s", want, out)
		}
	}
}

func TestCommentViewJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(adfComment))
	}))
	defer srv.Close()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginJiraSite(t, srv.URL)

	out, err := execJira(t, "issue", "comment", "view", "PROJ-1", "10", "--site", "work", "--json")
	if err != nil {
		t.Fatalf("issue comment view --json: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("comment view --json output is not valid JSON: %v\n%s", err, out)
	}
	if got["id"] != "10" {
		t.Fatalf("unexpected comment JSON: %v", got)
	}
}

func TestCommentListMapsNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"errorMessages":["Issue does not exist."]}`))
	}))
	defer srv.Close()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginJiraSite(t, srv.URL)

	_, err := execJira(t, "issue", "comment", "list", "PROJ-404", "--site", "work")
	if err == nil {
		t.Fatal("comment list of a missing issue returned no error")
	}
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Code != apperr.CodeNotFoundOrNotVisible {
		t.Fatalf("error = %v, want a not_found_or_not_visible *apperr.Error", err)
	}
}

func TestCommentViewRequiresTwoArgs(t *testing.T) {
	if _, err := execJira(t, "issue", "comment", "view", "PROJ-1", "--site", "work"); err == nil {
		t.Fatal("comment view with only one arg returned no error")
	}
}
