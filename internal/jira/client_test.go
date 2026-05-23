package jira

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
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

// serveJSON builds a test server that asserts the request is a GET against
// wantPath and replies with body.
func serveJSON(t *testing.T, wantPath, body string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("request method = %q, want GET", r.Method)
		}
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

func TestClientSearchUsers(t *testing.T) {
	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %q, want GET", r.Method)
		}
		if r.URL.Path != "/user/search" {
			t.Errorf("path = %q, want /user/search", r.URL.Path)
		}
		gotQuery = r.URL.Query().Get("query")
		_, _ = w.Write([]byte(`[{"accountId":"a1","displayName":"Ada","emailAddress":"ada@example.com"}]`))
	}))
	defer srv.Close()

	raw, err := newTestClient(srv).SearchUsers(context.Background(), "ada@example.com")
	if err != nil {
		t.Fatalf("SearchUsers: %v", err)
	}
	if gotQuery != "ada@example.com" {
		t.Errorf("query = %q, want ada@example.com", gotQuery)
	}
	users, err := Decode[[]User](raw)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if len(users) != 1 || users[0].AccountID != "a1" {
		t.Fatalf("users = %+v", users)
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
		if r.Method != http.MethodGet {
			t.Errorf("method = %q, want GET", r.Method)
		}
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

	raw, err := newTestClient(srv).GetIssue(context.Background(), "PROJ-1", "", "")
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

func TestClientListFields(t *testing.T) {
	srv := serveJSON(t, "/field",
		`[{"id":"summary","name":"Summary","custom":false,"schema":{"type":"string"}},`+
			`{"id":"customfield_10010","name":"Sprint","custom":true,"schema":{"type":"array"}}]`)
	defer srv.Close()

	raw, err := newTestClient(srv).ListFields(context.Background())
	if err != nil {
		t.Fatalf("ListFields: %v", err)
	}
	fields, err := Decode[[]Field](raw)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if len(fields) != 2 {
		t.Fatalf("got %d fields, want 2", len(fields))
	}
	if fields[1].ID != "customfield_10010" || !fields[1].Custom || fields[1].Schema.Type != "array" {
		t.Fatalf("field[1] = %+v", fields[1])
	}
}

func TestClientGetIssuePassesFieldsAndExpand(t *testing.T) {
	var gotFields, gotExpand string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotFields = r.URL.Query().Get("fields")
		gotExpand = r.URL.Query().Get("expand")
		_, _ = w.Write([]byte(`{"id":"1","key":"PROJ-1"}`))
	}))
	defer srv.Close()

	if _, err := newTestClient(srv).GetIssue(context.Background(), "PROJ-1", "summary,comment", "changelog"); err != nil {
		t.Fatalf("GetIssue: %v", err)
	}
	if gotFields != "summary,comment" {
		t.Fatalf("fields query = %q, want summary,comment", gotFields)
	}
	if gotExpand != "changelog" {
		t.Fatalf("expand query = %q, want changelog", gotExpand)
	}
}

func TestClientGetIssueOmitsEmptyFieldsAndExpand(t *testing.T) {
	var hadFields, hadExpand bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, hadFields = r.URL.Query()["fields"]
		_, hadExpand = r.URL.Query()["expand"]
		_, _ = w.Write([]byte(`{"id":"1","key":"PROJ-1"}`))
	}))
	defer srv.Close()

	if _, err := newTestClient(srv).GetIssue(context.Background(), "PROJ-1", "", ""); err != nil {
		t.Fatalf("GetIssue: %v", err)
	}
	if hadFields || hadExpand {
		t.Fatalf("empty fields/expand must not be sent (fields=%v expand=%v)", hadFields, hadExpand)
	}
}

func TestClientSearchIssues(t *testing.T) {
	var gotJQL, gotMax, gotFields string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %q, want GET", r.Method)
		}
		if r.URL.Path != "/search/jql" {
			t.Errorf("path = %q, want /search/jql", r.URL.Path)
		}
		gotJQL = r.URL.Query().Get("jql")
		gotMax = r.URL.Query().Get("maxResults")
		gotFields = r.URL.Query().Get("fields")
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
	if gotFields != "*navigable" {
		t.Errorf("fields param = %q, want *navigable", gotFields)
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

	raw, err := newTestClient(srv).ListComments(context.Background(), "PROJ-1", "", 0)
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

func TestClientListCommentsPassesOrderBy(t *testing.T) {
	var gotOrderBy string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotOrderBy = r.URL.Query().Get("orderBy")
		_, _ = w.Write([]byte(`{"comments":[]}`))
	}))
	defer srv.Close()
	if _, err := newTestClient(srv).ListComments(context.Background(), "PROJ-1", "-created", 0); err != nil {
		t.Fatalf("ListComments: %v", err)
	}
	if gotOrderBy != "-created" {
		t.Errorf("orderBy = %q, want -created", gotOrderBy)
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

func TestClientMapsHTTPStatusToStructuredError(t *testing.T) {
	cases := []struct {
		status int
		want   string
	}{
		{http.StatusUnauthorized, apperr.CodeUnauthorized},
		{http.StatusForbidden, apperr.CodeForbidden},
		{http.StatusNotFound, apperr.CodeNotFoundOrNotVisible},
		{http.StatusTooManyRequests, apperr.CodeRateLimited},
	}
	for _, tc := range cases {
		t.Run(http.StatusText(tc.status), func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(`{"errorMessages":["nope"]}`))
			}))
			defer srv.Close()

			_, err := newTestClient(srv).GetIssue(context.Background(), "PROJ-1", "", "")
			if err == nil {
				t.Fatalf("status %d returned no error", tc.status)
			}
			var ae *apperr.Error
			if !errors.As(err, &ae) {
				t.Fatalf("error type = %T, want *apperr.Error", err)
			}
			if ae.Code != tc.want {
				t.Errorf("Code = %q, want %q", ae.Code, tc.want)
			}
		})
	}
}

func TestDecodeRejectsMalformedJSON(t *testing.T) {
	if _, err := Decode[Issue]([]byte("not json")); err == nil {
		t.Fatal("Decode accepted malformed JSON")
	}
}

// captured records what a write request carried.
type captured struct {
	method, path, body string
}

// serveWrite builds a test server that records the request method, path, and
// body, then replies with status and respBody.
func serveWrite(t *testing.T, status int, respBody string) (*httptest.Server, *captured) {
	t.Helper()
	got := &captured{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		got.method, got.path, got.body = r.Method, r.URL.Path, string(b)
		w.WriteHeader(status)
		if respBody != "" {
			_, _ = w.Write([]byte(respBody))
		}
	}))
	return srv, got
}

func TestClientCreateIssue(t *testing.T) {
	srv, got := serveWrite(t, http.StatusCreated, `{"id":"1001","key":"PROJ-9"}`)
	defer srv.Close()

	raw, err := newTestClient(srv).CreateIssue(context.Background(), map[string]any{
		"project": map[string]any{"key": "PROJ"},
		"summary": "New bug",
	})
	if err != nil {
		t.Fatalf("CreateIssue: %v", err)
	}
	if got.method != http.MethodPost || got.path != "/issue" {
		t.Errorf("request = %s %s, want POST /issue", got.method, got.path)
	}
	if got.body != `{"fields":{"project":{"key":"PROJ"},"summary":"New bug"}}` {
		t.Errorf("request body = %s", got.body)
	}
	iss, err := Decode[Issue](raw)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if iss.Key != "PROJ-9" {
		t.Errorf("created issue key = %q, want PROJ-9", iss.Key)
	}
}

func TestClientEditIssue(t *testing.T) {
	srv, got := serveWrite(t, http.StatusNoContent, "")
	defer srv.Close()

	if err := newTestClient(srv).EditIssue(context.Background(), "PROJ-1",
		map[string]any{"summary": "Updated"}); err != nil {
		t.Fatalf("EditIssue: %v", err)
	}
	if got.method != http.MethodPut || got.path != "/issue/PROJ-1" {
		t.Errorf("request = %s %s, want PUT /issue/PROJ-1", got.method, got.path)
	}
	if got.body != `{"fields":{"summary":"Updated"}}` {
		t.Errorf("request body = %s", got.body)
	}
}

func TestClientGetTransitions(t *testing.T) {
	srv := serveJSON(t, "/issue/PROJ-1/transitions",
		`{"transitions":[{"id":"31","name":"Done"}]}`)
	defer srv.Close()

	raw, err := newTestClient(srv).GetTransitions(context.Background(), "PROJ-1")
	if err != nil {
		t.Fatalf("GetTransitions: %v", err)
	}
	if !strings.Contains(string(raw), `"name":"Done"`) {
		t.Errorf("transitions body = %s", raw)
	}
}

func TestClientDoTransition(t *testing.T) {
	srv, got := serveWrite(t, http.StatusNoContent, "")
	defer srv.Close()

	if err := newTestClient(srv).DoTransition(context.Background(), "PROJ-1", "31"); err != nil {
		t.Fatalf("DoTransition: %v", err)
	}
	if got.method != http.MethodPost || got.path != "/issue/PROJ-1/transitions" {
		t.Errorf("request = %s %s, want POST /issue/PROJ-1/transitions", got.method, got.path)
	}
	if got.body != `{"transition":{"id":"31"}}` {
		t.Errorf("request body = %s", got.body)
	}
}

func TestClientCreateComment(t *testing.T) {
	srv, got := serveWrite(t, http.StatusCreated, `{"id":"20","author":{"displayName":"Test User"}}`)
	defer srv.Close()

	raw, err := newTestClient(srv).CreateComment(context.Background(), "PROJ-1", DocOf("Looks good"))
	if err != nil {
		t.Fatalf("CreateComment: %v", err)
	}
	if got.method != http.MethodPost || got.path != "/issue/PROJ-1/comment" {
		t.Errorf("request = %s %s, want POST /issue/PROJ-1/comment", got.method, got.path)
	}
	if !strings.Contains(got.body, `"body":{"type":"doc"`) || !strings.Contains(got.body, `"text":"Looks good"`) {
		t.Errorf("request body = %s", got.body)
	}
	c, err := Decode[Comment](raw)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if c.ID != "20" {
		t.Errorf("created comment id = %q, want 20", c.ID)
	}
}

func TestClientEditComment(t *testing.T) {
	srv, got := serveWrite(t, http.StatusOK, `{"id":"20"}`)
	defer srv.Close()

	if _, err := newTestClient(srv).EditComment(context.Background(), "PROJ-1", "20",
		DocOf("Revised")); err != nil {
		t.Fatalf("EditComment: %v", err)
	}
	if got.method != http.MethodPut || got.path != "/issue/PROJ-1/comment/20" {
		t.Errorf("request = %s %s, want PUT /issue/PROJ-1/comment/20", got.method, got.path)
	}
	if !strings.Contains(got.body, `"text":"Revised"`) {
		t.Errorf("request body = %s", got.body)
	}
}

func TestClientDeleteComment(t *testing.T) {
	srv, got := serveWrite(t, http.StatusNoContent, "")
	defer srv.Close()

	if err := newTestClient(srv).DeleteComment(context.Background(), "PROJ-1", "20"); err != nil {
		t.Fatalf("DeleteComment: %v", err)
	}
	if got.method != http.MethodDelete || got.path != "/issue/PROJ-1/comment/20" {
		t.Errorf("request = %s %s, want DELETE /issue/PROJ-1/comment/20", got.method, got.path)
	}
	if got.body != "" {
		t.Errorf("DELETE sent a body: %s", got.body)
	}
}

func TestClientSearchProjectsAllFollowsOffsetPages(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Query().Get("startAt") {
		case "", "0":
			_, _ = w.Write([]byte(`{"startAt":0,"maxResults":2,"total":3,"isLast":false,` +
				`"values":[{"key":"A"},{"key":"B"}]}`))
		case "2":
			_, _ = w.Write([]byte(`{"startAt":2,"maxResults":2,"total":3,"isLast":true,` +
				`"values":[{"key":"C"}]}`))
		default:
			t.Errorf("unexpected startAt %q", r.URL.Query().Get("startAt"))
		}
	}))
	defer srv.Close()

	raw, err := newTestClient(srv).SearchProjectsAll(context.Background(), 2)
	if err != nil {
		t.Fatalf("SearchProjectsAll: %v", err)
	}
	page, err := Decode[ProjectPage](raw)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if len(page.Values) != 3 {
		t.Fatalf("aggregated %d projects, want 3: %+v", len(page.Values), page.Values)
	}
}

func TestClientSearchIssuesAllFollowsTokenPages(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Query().Get("nextPageToken") {
		case "":
			_, _ = w.Write([]byte(`{"issues":[{"key":"P-1"},{"key":"P-2"}],"nextPageToken":"tok2"}`))
		case "tok2":
			_, _ = w.Write([]byte(`{"issues":[{"key":"P-3"}]}`))
		default:
			t.Errorf("unexpected nextPageToken %q", r.URL.Query().Get("nextPageToken"))
		}
	}))
	defer srv.Close()

	raw, err := newTestClient(srv).SearchIssuesAll(context.Background(), "project = P", 2)
	if err != nil {
		t.Fatalf("SearchIssuesAll: %v", err)
	}
	page, err := Decode[IssuePage](raw)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if len(page.Issues) != 3 {
		t.Fatalf("aggregated %d issues, want 3", len(page.Issues))
	}
}

func TestClientListCommentsAllFollowsOffsetPages(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Query().Get("startAt") {
		case "", "0":
			_, _ = w.Write([]byte(`{"startAt":0,"maxResults":2,"total":3,` +
				`"comments":[{"id":"1"},{"id":"2"}]}`))
		case "2":
			_, _ = w.Write([]byte(`{"startAt":2,"maxResults":2,"total":3,"comments":[{"id":"3"}]}`))
		default:
			t.Errorf("unexpected startAt %q", r.URL.Query().Get("startAt"))
		}
	}))
	defer srv.Close()

	raw, err := newTestClient(srv).ListCommentsAll(context.Background(), "P-1", "", 2)
	if err != nil {
		t.Fatalf("ListCommentsAll: %v", err)
	}
	page, err := Decode[CommentPage](raw)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if len(page.Comments) != 3 {
		t.Fatalf("aggregated %d comments, want 3", len(page.Comments))
	}
}

func TestClientSearchIssuesAllMapsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"errorMessages":["You do not have permission."]}`))
	}))
	defer srv.Close()

	_, err := newTestClient(srv).SearchIssuesAll(context.Background(), "project = P", 0)
	if err == nil {
		t.Fatal("SearchIssuesAll against a 403 endpoint returned no error")
	}
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Code != apperr.CodeForbidden {
		t.Fatalf("error = %v, want a forbidden *apperr.Error", err)
	}
}
