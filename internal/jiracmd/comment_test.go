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
)

const adfComment = `{"id":"10","author":{"displayName":"Ada Lovelace"},` +
	`"created":"2026-05-18T10:00:00.000+0000",` +
	`"body":{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Looks good to me"}]}]}}`

func TestCommentListHumanOutput(t *testing.T) {
	var gotMax string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/issue/PROJ-1/comment" {
			t.Errorf("path = %q, want /issue/PROJ-1/comment", r.URL.Path)
		}
		gotMax = r.URL.Query().Get("maxResults")
		_, _ = w.Write([]byte(`{"comments":[` + adfComment + `],"total":1}`))
	}))
	defer srv.Close()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginJiraSite(t, srv.URL)

	out, err := execJira(t, "issue", "comment", "list", "PROJ-1", "--limit", "3", "--site", "work")
	if err != nil {
		t.Fatalf("issue comment list: %v", err)
	}
	if gotMax != "3" {
		t.Errorf("comment list sent maxResults %q, want 3 (--limit not plumbed)", gotMax)
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

func TestCommentViewWithoutBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		// A system comment with no body, updated later than it was created.
		_, _ = w.Write([]byte(`{"id":"11","author":{"displayName":"Automation"},` +
			`"created":"2026-05-18T10:00:00.000+0000","updated":"2026-05-19T11:00:00.000+0000"}`))
	}))
	defer srv.Close()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginJiraSite(t, srv.URL)

	out, err := execJira(t, "issue", "comment", "view", "PROJ-1", "11", "--site", "work")
	if err != nil {
		t.Fatalf("issue comment view: %v", err)
	}
	for _, want := range []string{"11", "Automation", "created:", "updated:"} {
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

func TestCommentCreateHumanOutput(t *testing.T) {
	var gotMethod, gotPath, gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath = r.Method, r.URL.Path
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"55","author":{"displayName":"Ada Lovelace"}}`))
	}))
	defer srv.Close()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginJiraSite(t, srv.URL)

	out, err := execJira(t, "issue", "comment", "create", "PROJ-1",
		"--body", "Looks good", "--site", "work")
	if err != nil {
		t.Fatalf("comment create: %v", err)
	}
	if gotMethod != http.MethodPost || gotPath != "/issue/PROJ-1/comment" {
		t.Errorf("request = %s %s, want POST /issue/PROJ-1/comment", gotMethod, gotPath)
	}
	// A plain --body is wrapped as an ADF document.
	if !strings.Contains(gotBody, `"body":{"type":"doc"`) ||
		!strings.Contains(gotBody, `"text":"Looks good"`) {
		t.Errorf("--body not wrapped as ADF:\n%s", gotBody)
	}
	if !strings.Contains(out, "created comment 55 on PROJ-1") {
		t.Errorf("comment create output:\n%s", out)
	}
}

func TestCommentCreateJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"55"}`))
	}))
	defer srv.Close()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginJiraSite(t, srv.URL)

	out, err := execJira(t, "issue", "comment", "create", "PROJ-1",
		"--body", "hi", "--site", "work", "--json")
	if err != nil {
		t.Fatalf("comment create --json: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("comment create --json output is not valid JSON: %v\n%s", err, out)
	}
	if got["id"] != "55" {
		t.Fatalf("unexpected create JSON: %v", got)
	}
}

func TestCommentCreateRequiresBody(t *testing.T) {
	_, err := execJira(t, "issue", "comment", "create", "PROJ-1", "--site", "work")
	if err == nil {
		t.Fatal("comment create without --body returned no error")
	}
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Code != apperr.CodeInvalidInput {
		t.Fatalf("error = %v, want an invalid_input *apperr.Error", err)
	}
}

func TestCommentEditHumanOutput(t *testing.T) {
	var gotMethod, gotPath, gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath = r.Method, r.URL.Path
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		_, _ = w.Write([]byte(`{"id":"10"}`))
	}))
	defer srv.Close()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginJiraSite(t, srv.URL)

	out, err := execJira(t, "issue", "comment", "edit", "PROJ-1", "10",
		"--body", "Revised", "--site", "work")
	if err != nil {
		t.Fatalf("comment edit: %v", err)
	}
	if gotMethod != http.MethodPut || gotPath != "/issue/PROJ-1/comment/10" {
		t.Errorf("request = %s %s, want PUT /issue/PROJ-1/comment/10", gotMethod, gotPath)
	}
	if !strings.Contains(gotBody, `"text":"Revised"`) {
		t.Errorf("edit request body missing the new text:\n%s", gotBody)
	}
	if !strings.Contains(out, "updated comment 10 on PROJ-1") {
		t.Errorf("comment edit output:\n%s", out)
	}
}

func TestCommentEditJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"id":"10","body":{"type":"doc"}}`))
	}))
	defer srv.Close()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginJiraSite(t, srv.URL)

	out, err := execJira(t, "issue", "comment", "edit", "PROJ-1", "10",
		"--body", "Revised", "--site", "work", "--json")
	if err != nil {
		t.Fatalf("comment edit --json: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("comment edit --json output is not valid JSON: %v\n%s", err, out)
	}
	if got["id"] != "10" {
		t.Fatalf("unexpected edit JSON: %v", got)
	}
}

func TestCommentEditRequiresBody(t *testing.T) {
	_, err := execJira(t, "issue", "comment", "edit", "PROJ-1", "10", "--site", "work")
	if err == nil {
		t.Fatal("comment edit without --body returned no error")
	}
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Code != apperr.CodeInvalidInput {
		t.Fatalf("error = %v, want an invalid_input *apperr.Error", err)
	}
}

func TestCommentDeleteHumanOutput(t *testing.T) {
	var gotMethod, gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath = r.Method, r.URL.Path
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginJiraSite(t, srv.URL)

	out, err := execJira(t, "issue", "comment", "delete", "PROJ-1", "10", "--site", "work")
	if err != nil {
		t.Fatalf("comment delete: %v", err)
	}
	if gotMethod != http.MethodDelete || gotPath != "/issue/PROJ-1/comment/10" {
		t.Errorf("request = %s %s, want DELETE /issue/PROJ-1/comment/10", gotMethod, gotPath)
	}
	if !strings.Contains(out, "deleted comment 10 on PROJ-1") {
		t.Errorf("comment delete output:\n%s", out)
	}
}

func TestCommentDeleteJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginJiraSite(t, srv.URL)

	out, err := execJira(t, "issue", "comment", "delete", "PROJ-1", "10", "--site", "work", "--json")
	if err != nil {
		t.Fatalf("comment delete --json: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("comment delete --json output is not valid JSON: %v\n%s", err, out)
	}
	if got["issue"] != "PROJ-1" || got["comment"] != "10" || got["deleted"] != true {
		t.Fatalf("unexpected delete JSON: %v", got)
	}
}

func TestCommentDeleteMapsNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"errorMessages":["Comment does not exist."]}`))
	}))
	defer srv.Close()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginJiraSite(t, srv.URL)

	_, err := execJira(t, "issue", "comment", "delete", "PROJ-1", "404", "--site", "work")
	if err == nil {
		t.Fatal("comment delete of a missing comment returned no error")
	}
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Code != apperr.CodeNotFoundOrNotVisible {
		t.Fatalf("error = %v, want a not_found_or_not_visible *apperr.Error", err)
	}
}
