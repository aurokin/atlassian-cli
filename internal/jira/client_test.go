package jira

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aurokin/atlassian-cli/internal/apperr"
	"github.com/aurokin/atlassian-cli/internal/auth"
	"github.com/aurokin/atlassian-cli/internal/httpclient"
)

// newTestClient builds a Jira client whose requests are routed to srv. The
// data-center token style makes the configured URL the API base verbatim, so
// API-relative paths land on the test server unchanged.
func newTestClient(srv *httptest.Server) *Client {
	target := httpclient.Target{
		Product:    httpclient.ProductJira,
		TokenStyle: auth.StyleDataCenterPAT,
		SiteName:   "test",
		BaseURL:    srv.URL,
	}
	cred := auth.Credential{Style: auth.StyleDataCenterPAT, Token: "test-token"}
	return New(httpclient.New(target, cred, srv.Client()))
}

// serveJSON builds a test server that asserts the request path and replies
// with body.
func serveJSON(t *testing.T, wantPath, body string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != wantPath {
			t.Errorf("request path = %q, want %q", r.URL.Path, wantPath)
		}
		_, _ = w.Write([]byte(body))
	}))
}

func TestClientMyself(t *testing.T) {
	srv := serveJSON(t, "/myself", `{"accountId":"a1","displayName":"Test User"}`)
	defer srv.Close()

	raw, err := newTestClient(srv).Myself(context.Background())
	if err != nil {
		t.Fatalf("Myself: %v", err)
	}
	user, err := Decode[User](raw)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if user.AccountID != "a1" || user.DisplayName != "Test User" {
		t.Fatalf("user = %+v", user)
	}
}

func TestClientGetProject(t *testing.T) {
	srv := serveJSON(t, "/project/PROJ",
		`{"id":"100","key":"PROJ","name":"Project X","projectTypeKey":"software"}`)
	defer srv.Close()

	raw, err := newTestClient(srv).GetProject(context.Background(), "PROJ")
	if err != nil {
		t.Fatalf("GetProject: %v", err)
	}
	p, err := Decode[Project](raw)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if p.Key != "PROJ" || p.Name != "Project X" || p.ProjectTypeKey != "software" {
		t.Fatalf("project = %+v", p)
	}
}

func TestClientSearchProjects(t *testing.T) {
	var gotMax string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/project/search" {
			t.Errorf("path = %q, want /project/search", r.URL.Path)
		}
		gotMax = r.URL.Query().Get("maxResults")
		_, _ = w.Write([]byte(`{"values":[{"key":"PROJ","name":"Project X"}],"total":1,"isLast":true}`))
	}))
	defer srv.Close()

	raw, err := newTestClient(srv).SearchProjects(context.Background(), 50)
	if err != nil {
		t.Fatalf("SearchProjects: %v", err)
	}
	if gotMax != "50" {
		t.Errorf("maxResults param = %q, want 50", gotMax)
	}
	page, err := Decode[ProjectPage](raw)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if len(page.Values) != 1 || page.Values[0].Key != "PROJ" {
		t.Fatalf("values = %+v", page.Values)
	}
}

func TestClientGetIssue(t *testing.T) {
	srv := serveJSON(t, "/issue/PROJ-1",
		`{"id":"1","key":"PROJ-1","fields":{"summary":"First","status":{"name":"To Do"},"issuetype":{"name":"Task"}}}`)
	defer srv.Close()

	raw, err := newTestClient(srv).GetIssue(context.Background(), "PROJ-1")
	if err != nil {
		t.Fatalf("GetIssue: %v", err)
	}
	iss, err := Decode[Issue](raw)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if iss.Key != "PROJ-1" || iss.Fields.Summary != "First" {
		t.Fatalf("issue = %+v", iss)
	}
	if iss.Fields.Status == nil || iss.Fields.Status.Name != "To Do" {
		t.Fatalf("status = %+v", iss.Fields.Status)
	}
}

func TestClientSearchIssues(t *testing.T) {
	var gotJQL, gotMax string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/search/jql" {
			t.Errorf("path = %q, want /search/jql", r.URL.Path)
		}
		gotJQL = r.URL.Query().Get("jql")
		gotMax = r.URL.Query().Get("maxResults")
		_, _ = w.Write([]byte(`{"issues":[{"key":"PROJ-1","fields":{"summary":"First"}}],"isLast":true}`))
	}))
	defer srv.Close()

	raw, err := newTestClient(srv).SearchIssues(context.Background(), "project = PROJ", 25)
	if err != nil {
		t.Fatalf("SearchIssues: %v", err)
	}
	if gotJQL != "project = PROJ" {
		t.Errorf("jql param = %q, want %q", gotJQL, "project = PROJ")
	}
	if gotMax != "25" {
		t.Errorf("maxResults param = %q, want 25", gotMax)
	}
	page, err := Decode[IssuePage](raw)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if len(page.Issues) != 1 || page.Issues[0].Key != "PROJ-1" {
		t.Fatalf("issues = %+v", page.Issues)
	}
}

func TestClientListComments(t *testing.T) {
	srv := serveJSON(t, "/issue/PROJ-1/comment",
		`{"comments":[{"id":"10","author":{"displayName":"Test User"}}],"total":1}`)
	defer srv.Close()

	raw, err := newTestClient(srv).ListComments(context.Background(), "PROJ-1", 0)
	if err != nil {
		t.Fatalf("ListComments: %v", err)
	}
	page, err := Decode[CommentPage](raw)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if len(page.Comments) != 1 || page.Comments[0].ID != "10" {
		t.Fatalf("comments = %+v", page.Comments)
	}
}

func TestClientGetComment(t *testing.T) {
	srv := serveJSON(t, "/issue/PROJ-1/comment/10",
		`{"id":"10","author":{"displayName":"Test User"},"created":"2026-05-18T10:00:00.000+0000"}`)
	defer srv.Close()

	raw, err := newTestClient(srv).GetComment(context.Background(), "PROJ-1", "10")
	if err != nil {
		t.Fatalf("GetComment: %v", err)
	}
	c, err := Decode[Comment](raw)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if c.ID != "10" || c.Author == nil || c.Author.DisplayName != "Test User" {
		t.Fatalf("comment = %+v", c)
	}
}

func TestClientMapsNonOKToStructuredError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"errorMessages":["Issue does not exist"]}`))
	}))
	defer srv.Close()

	_, err := newTestClient(srv).GetIssue(context.Background(), "PROJ-404")
	if err == nil {
		t.Fatal("GetIssue of a missing issue returned no error")
	}
	var ae *apperr.Error
	if !errors.As(err, &ae) {
		t.Fatalf("error type = %T, want *apperr.Error", err)
	}
	if ae.Code != apperr.CodeNotFoundOrNotVisible {
		t.Errorf("Code = %q, want %q", ae.Code, apperr.CodeNotFoundOrNotVisible)
	}
}

func TestDecodeRejectsMalformedJSON(t *testing.T) {
	if _, err := Decode[Issue]([]byte("not json")); err == nil {
		t.Fatal("Decode accepted malformed JSON")
	}
}
