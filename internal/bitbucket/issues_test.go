package bitbucket

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aurokin/atlassian-cli/internal/apperr"
)

func TestListIssuesQuery(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repositories/acme/widgets/issues" {
			t.Errorf("path = %q", r.URL.Path)
		}
		if got := r.URL.Query().Get("state"); got != "open" {
			t.Errorf("state = %q, want open", got)
		}
		if got := r.URL.Query().Get("pagelen"); got != "5" {
			t.Errorf("pagelen = %q, want 5", got)
		}
		_, _ = w.Write([]byte(`{"values":[{"id":1,"title":"t","state":"open"}]}`))
	}))
	defer srv.Close()

	raw, err := newTestClient(srv).ListIssues(context.Background(), "acme", "widgets", "open", 5)
	if err != nil {
		t.Fatalf("ListIssues: %v", err)
	}
	page, err := Decode[IssuePage](raw)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if len(page.Values) != 1 || page.Values[0].ID != 1 {
		t.Fatalf("values = %+v", page.Values)
	}
}

func TestGetIssue(t *testing.T) {
	srv := serveJSON(t, "/repositories/acme/widgets/issues/3",
		`{"id":3,"title":"Crash","state":"open","kind":"bug"}`)
	defer srv.Close()

	raw, err := newTestClient(srv).GetIssue(context.Background(), "acme", "widgets", 3)
	if err != nil {
		t.Fatalf("GetIssue: %v", err)
	}
	issue, err := Decode[Issue](raw)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if issue.ID != 3 || issue.Kind != "bug" {
		t.Fatalf("issue = %+v", issue)
	}
}

func TestCreateIssueBody(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want POST", r.Method)
		}
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &gotBody)
		_, _ = w.Write([]byte(`{"id":1,"title":"t"}`))
	}))
	defer srv.Close()

	_, err := newTestClient(srv).CreateIssue(context.Background(), "acme", "widgets",
		CreateIssueOptions{Title: "t", Body: "raw text"})
	if err != nil {
		t.Fatalf("CreateIssue: %v", err)
	}
	if gotBody["title"] != "t" {
		t.Fatalf("body = %+v", gotBody)
	}
	content, _ := gotBody["content"].(map[string]any)
	if content["raw"] != "raw text" {
		t.Fatalf("content = %+v", gotBody["content"])
	}
	// kind/priority are omitted when empty.
	if _, ok := gotBody["kind"]; ok {
		t.Fatalf("kind should be omitted: %+v", gotBody)
	}
}

func TestUpdateIssueBody(t *testing.T) {
	var gotMethod, gotPath string
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath = r.Method, r.URL.Path
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &gotBody)
		_, _ = w.Write([]byte(`{"id":3,"title":"t","state":"resolved"}`))
	}))
	defer srv.Close()

	_, err := newTestClient(srv).UpdateIssue(context.Background(), "acme", "widgets", 3,
		UpdateIssueOptions{State: "resolved"})
	if err != nil {
		t.Fatalf("UpdateIssue: %v", err)
	}
	if gotMethod != http.MethodPut || gotPath != "/repositories/acme/widgets/issues/3" {
		t.Errorf("request = %s %s", gotMethod, gotPath)
	}
	if gotBody["state"] != "resolved" {
		t.Fatalf("body = %+v", gotBody)
	}
	// Unset fields are omitted entirely.
	if _, ok := gotBody["title"]; ok {
		t.Fatalf("title should be omitted: %+v", gotBody)
	}
}

func TestUpdateIssueOptionsIsEmpty(t *testing.T) {
	if !(UpdateIssueOptions{}).IsEmpty() {
		t.Errorf("zero options should be empty")
	}
	if (UpdateIssueOptions{State: "open"}).IsEmpty() {
		t.Errorf("options with a state should not be empty")
	}
}

func TestListIssuesTrackerDisabled(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"type":"error","error":{"message":"Repository has no issue tracker."}}`))
	}))
	defer srv.Close()

	_, err := newTestClient(srv).ListIssues(context.Background(), "acme", "widgets", "", 0)
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Code != apperr.CodeFeatureDisabled {
		t.Fatalf("error = %v, want feature_disabled", err)
	}
}
