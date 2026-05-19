package confcmd

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aurokin/atlassian-cli/internal/apperr"
)

func TestPageCommentListHumanOutput(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/pages/10/footer-comments" {
			t.Errorf("path = %q, want /pages/10/footer-comments", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"results":[{"id":"c1","status":"current","title":"Re: Home"},` +
			`{"id":"c2","status":"current","title":"Re: Home"}]}`))
	}))
	defer srv.Close()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginConfSite(t, srv.URL)

	out, err := execConf(t, "page", "comment", "list", "10", "--site", "work")
	if err != nil {
		t.Fatalf("page comment list: %v", err)
	}
	for _, want := range []string{"c1", "c2", "Re: Home"} {
		if !strings.Contains(out, want) {
			t.Errorf("comment list output missing %q:\n%s", want, out)
		}
	}
}

func TestPageCommentListJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"results":[{"id":"c1"}]}`))
	}))
	defer srv.Close()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginConfSite(t, srv.URL)

	out, err := execConf(t, "page", "comment", "list", "10", "--site", "work", "--json")
	if err != nil {
		t.Fatalf("page comment list --json: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("comment list --json output is not valid JSON: %v\n%s", err, out)
	}
	if _, ok := got["results"]; !ok {
		t.Fatalf("unexpected comment list JSON: %v", got)
	}
}

func TestPageCommentViewHumanOutput(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/footer-comments/c1" {
			t.Errorf("path = %q, want /footer-comments/c1", r.URL.Path)
		}
		if got := r.URL.Query().Get("body-format"); got != "storage" {
			t.Errorf("body-format = %q, want storage", got)
		}
		_, _ = w.Write([]byte(`{"id":"c1","status":"current","pageId":"10","version":{"number":2},` +
			`"body":{"storage":{"representation":"storage","value":"<p>hello</p>"}}}`))
	}))
	defer srv.Close()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginConfSite(t, srv.URL)

	out, err := execConf(t, "page", "comment", "view", "c1", "--site", "work")
	if err != nil {
		t.Fatalf("page comment view: %v", err)
	}
	for _, want := range []string{"c1", "current", "10", "<p>hello</p>"} {
		if !strings.Contains(out, want) {
			t.Errorf("comment view output missing %q:\n%s", want, out)
		}
	}
}

func TestPageCommentViewMapsNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"Comment not found"}`))
	}))
	defer srv.Close()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginConfSite(t, srv.URL)

	_, err := execConf(t, "page", "comment", "view", "404", "--site", "work")
	if err == nil {
		t.Fatal("comment view of a missing comment returned no error")
	}
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Code != apperr.CodeNotFoundOrNotVisible {
		t.Fatalf("error = %v, want a not_found_or_not_visible *apperr.Error", err)
	}
}

func TestPageCommentCreateHumanOutput(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/footer-comments" {
			t.Errorf("path = %q, want /footer-comments", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want POST", r.Method)
		}
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_, _ = w.Write([]byte(`{"id":"c9","status":"current","pageId":"10"}`))
	}))
	defer srv.Close()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginConfSite(t, srv.URL)

	out, err := execConf(t, "page", "comment", "create", "10",
		"--body", "<p>nice</p>", "--body-format", "storage", "--site", "work")
	if err != nil {
		t.Fatalf("page comment create: %v", err)
	}
	if gotBody["pageId"] != "10" {
		t.Errorf("create sent pageId %v, want 10", gotBody["pageId"])
	}
	body, _ := gotBody["body"].(map[string]any)
	if body["representation"] != "storage" || body["value"] != "<p>nice</p>" {
		t.Errorf("create sent body %v, want storage/<p>nice</p>", gotBody["body"])
	}
	if !strings.Contains(out, "created comment c9") {
		t.Errorf("create output missing 'created comment c9':\n%s", out)
	}
}

func TestPageCommentCreateRequiresFlags(t *testing.T) {
	_, err := execConf(t, "page", "comment", "create", "10", "--site", "work")
	if err == nil {
		t.Fatal("page comment create without --body returned no error")
	}
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Code != apperr.CodeInvalidInput {
		t.Fatalf("error = %v, want an invalid_input *apperr.Error", err)
	}
}

func TestPageCommentEditHumanOutput(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/footer-comments/c1" {
			t.Errorf("path = %q, want /footer-comments/c1", r.URL.Path)
		}
		switch r.Method {
		case http.MethodGet:
			_, _ = w.Write([]byte(`{"id":"c1","status":"current","pageId":"10","version":{"number":3}}`))
		case http.MethodPut:
			_ = json.NewDecoder(r.Body).Decode(&gotBody)
			_, _ = w.Write([]byte(`{"id":"c1","status":"current","version":{"number":4}}`))
		default:
			t.Errorf("method = %q, want GET or PUT", r.Method)
		}
	}))
	defer srv.Close()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginConfSite(t, srv.URL)

	out, err := execConf(t, "page", "comment", "edit", "c1",
		"--body", "<p>edited</p>", "--body-format", "storage", "--site", "work")
	if err != nil {
		t.Fatalf("page comment edit: %v", err)
	}
	ver, _ := gotBody["version"].(map[string]any)
	if ver["number"] != float64(4) {
		t.Errorf("edit sent version %v, want number 4 (current+1)", gotBody["version"])
	}
	body, _ := gotBody["body"].(map[string]any)
	if body["value"] != "<p>edited</p>" {
		t.Errorf("edit sent body %v, want value <p>edited</p>", gotBody["body"])
	}
	if !strings.Contains(out, "updated comment c1 to version 4") {
		t.Errorf("edit output missing 'updated comment c1 to version 4':\n%s", out)
	}
}

func TestPageCommentEditRequiresFlags(t *testing.T) {
	_, err := execConf(t, "page", "comment", "edit", "c1", "--site", "work")
	if err == nil {
		t.Fatal("page comment edit without --body returned no error")
	}
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Code != apperr.CodeInvalidInput {
		t.Fatalf("error = %v, want an invalid_input *apperr.Error", err)
	}
}

func TestPageCommentDeleteHumanOutput(t *testing.T) {
	var gotMethod string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		if r.URL.Path != "/footer-comments/c1" {
			t.Errorf("path = %q, want /footer-comments/c1", r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginConfSite(t, srv.URL)

	out, err := execConf(t, "page", "comment", "delete", "c1", "--site", "work")
	if err != nil {
		t.Fatalf("page comment delete: %v", err)
	}
	if gotMethod != http.MethodDelete {
		t.Errorf("delete used method %q, want DELETE", gotMethod)
	}
	if !strings.Contains(out, "deleted comment c1") {
		t.Errorf("delete output missing 'deleted comment c1':\n%s", out)
	}
}
