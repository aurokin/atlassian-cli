package bitbucket

import (
	"context"
	"encoding/base64"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aurokin/atlassian-cli/internal/apperr"
	"github.com/aurokin/atlassian-cli/internal/auth"
	"github.com/aurokin/atlassian-cli/internal/httpclient"
	"github.com/aurokin/atlassian-cli/internal/restutil"
)

// newTestClient builds a Bitbucket client whose requests are routed to srv.
// The cloud-classic (Basic) style with srv.URL as the base URL makes the test
// server the API base, so API-relative paths land on it unchanged.
func newTestClient(srv *httptest.Server) *Client {
	target := httpclient.Target{
		Product:    httpclient.ProductBitbucket,
		TokenStyle: auth.StyleCloudClassic,
		SiteName:   "test",
		BaseURL:    srv.URL,
	}
	cred := auth.Credential{
		Style:    auth.StyleCloudClassic,
		Username: "auro@example.com",
		Token:    "api-token-secret",
	}
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

func TestCurrentUser(t *testing.T) {
	srv := serveJSON(t, "/user", `{"account_id":"123","display_name":"Auro","username":"auro"}`)
	defer srv.Close()

	raw, err := newTestClient(srv).CurrentUser(context.Background())
	if err != nil {
		t.Fatalf("CurrentUser: %v", err)
	}
	user, err := Decode[CurrentUser](raw)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if user.AccountID != "123" || user.DisplayName != "Auro" || user.Username != "auro" {
		t.Fatalf("user = %+v", user)
	}
}

func TestCurrentUserSendsBasicAuth(t *testing.T) {
	want := "Basic " + base64.StdEncoding.EncodeToString([]byte("auro@example.com:api-token-secret"))
	var got string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Get("Authorization")
		_, _ = w.Write([]byte(`{"account_id":"123","display_name":"Auro"}`))
	}))
	defer srv.Close()

	if _, err := newTestClient(srv).CurrentUser(context.Background()); err != nil {
		t.Fatalf("CurrentUser: %v", err)
	}
	if got != want {
		t.Fatalf("Authorization = %q, want %q", got, want)
	}
}

func TestGetRepository(t *testing.T) {
	srv := serveJSON(t, "/repositories/acme/widgets",
		`{"full_name":"acme/widgets","name":"widgets","is_private":true,"mainbranch":{"name":"main"}}`)
	defer srv.Close()

	raw, err := newTestClient(srv).GetRepository(context.Background(), "acme", "widgets")
	if err != nil {
		t.Fatalf("GetRepository: %v", err)
	}
	repo, err := Decode[Repository](raw)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if repo.FullName != "acme/widgets" || !repo.IsPrivate {
		t.Fatalf("repo = %+v", repo)
	}
	if repo.MainBranch == nil || repo.MainBranch.Name != "main" {
		t.Fatalf("mainbranch = %+v", repo.MainBranch)
	}
}

func TestListRepositoriesAllFollowsNext(t *testing.T) {
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repositories/acme" {
			t.Errorf("path = %q", r.URL.Path)
		}
		switch r.URL.Query().Get("page") {
		case "", "1":
			// First page links to page 2 via an absolute same-origin URL.
			_, _ = w.Write([]byte(`{"values":[{"full_name":"acme/a"}],"next":"` +
				srv.URL + `/repositories/acme?page=2"}`))
		case "2":
			_, _ = w.Write([]byte(`{"values":[{"full_name":"acme/b"}]}`))
		default:
			t.Errorf("unexpected page %q", r.URL.Query().Get("page"))
		}
	}))
	defer srv.Close()

	raw, err := newTestClient(srv).ListRepositoriesAll(context.Background(), "acme", 0)
	if err != nil {
		t.Fatalf("ListRepositoriesAll: %v", err)
	}
	page, err := Decode[RepositoryPage](raw)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if len(page.Values) != 2 || page.Values[0].FullName != "acme/a" || page.Values[1].FullName != "acme/b" {
		t.Fatalf("values = %+v", page.Values)
	}
	if page.Next != "" {
		t.Fatalf("aggregated body should not carry a next cursor, got %q", page.Next)
	}
}

// TestListRepositoriesAllTruncates verifies that following more than the page
// cap (a server that never stops paginating) returns a truncation error rather
// than silently aggregating a partial, whole-looking result.
func TestListRepositoriesAllTruncates(t *testing.T) {
	var srv *httptest.Server
	pages := 0
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pages++
		// Always advertise a further page, so the follow loop only ever exits
		// at the page cap.
		_, _ = w.Write([]byte(`{"values":[{"full_name":"acme/a"}],"next":"` +
			srv.URL + `/repositories/acme?page=next"}`))
	}))
	defer srv.Close()

	_, err := newTestClient(srv).ListRepositoriesAll(context.Background(), "acme", 0)
	if err == nil {
		t.Fatal("expected a truncation error when the API never stops paginating")
	}
	if !strings.Contains(err.Error(), "result_truncated") {
		t.Fatalf("error = %q, want a result_truncated error", err)
	}
	if pages != restutil.MaxFollowPages {
		t.Fatalf("followed %d pages, want the cap of %d", pages, restutil.MaxFollowPages)
	}
}

// errCase pins the apperr code produced for a given Bitbucket HTTP status and
// body.
func TestErrorMapping(t *testing.T) {
	cases := []struct {
		name     string
		status   int
		body     string
		wantCode string
		wantMsg  string
	}{
		{"unauthorized", http.StatusUnauthorized,
			`{"type":"error","error":{"message":"Token is invalid or expired"}}`,
			apperr.CodeUnauthorized, "Token is invalid or expired"},
		{"forbidden", http.StatusForbidden,
			`{"type":"error","error":{"message":"Insufficient permissions"}}`,
			apperr.CodeForbidden, "Insufficient permissions"},
		{"not found", http.StatusNotFound,
			`{"type":"error","error":{"message":"Repository acme/ghost not found"}}`,
			apperr.CodeNotFoundOrNotVisible, "Repository acme/ghost not found"},
		{"rate limited", http.StatusTooManyRequests,
			`{"type":"error","error":{"message":"slow down"}}`,
			apperr.CodeRateLimited, "slow down"},
		{"issue tracker disabled", http.StatusNotFound,
			`{"type":"error","error":{"message":"Repository has no issue tracker."}}`,
			apperr.CodeFeatureDisabled, "Repository has no issue tracker."},
		{"wiki disabled", http.StatusNotFound,
			`{"type":"error","error":{"message":"Repository has no wiki."}}`,
			apperr.CodeFeatureDisabled, "Repository has no wiki."},
		// An ordinary not-found for a repository that merely contains "wiki"
		// in its name must NOT be re-coded as feature_disabled.
		{"repo named wiki not found", http.StatusNotFound,
			`{"type":"error","error":{"message":"Repository acme/wiki not found"}}`,
			apperr.CodeNotFoundOrNotVisible, "Repository acme/wiki not found"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer srv.Close()

			_, err := newTestClient(srv).GetRepository(context.Background(), "acme", "widgets")
			if err == nil {
				t.Fatalf("expected error for status %d", tc.status)
			}
			var ae *apperr.Error
			if !errors.As(err, &ae) {
				t.Fatalf("error is %T, want *apperr.Error", err)
			}
			if ae.Code != tc.wantCode {
				t.Fatalf("code = %q, want %q", ae.Code, tc.wantCode)
			}
			if !strings.Contains(ae.Message, tc.wantMsg) {
				t.Fatalf("message = %q, want substring %q", ae.Message, tc.wantMsg)
			}
			if ae.Product != httpclient.ProductBitbucket {
				t.Fatalf("product = %q, want %q", ae.Product, httpclient.ProductBitbucket)
			}
		})
	}
}

func TestAPIBaseDefault(t *testing.T) {
	target := httpclient.Target{
		Product:    httpclient.ProductBitbucket,
		TokenStyle: auth.StyleCloudClassic,
		SiteName:   "bitbucket",
	}
	base, err := target.APIBase()
	if err != nil {
		t.Fatalf("APIBase: %v", err)
	}
	if base != "https://api.bitbucket.org/2.0" {
		t.Fatalf("APIBase = %q", base)
	}
}

// TestSendEncodesJSONBody confirms send marshals the payload and the server
// receives it. It is a placeholder for the write methods ported in B3b.
func TestSendEncodesJSONBody(t *testing.T) {
	var gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf, _ := io.ReadAll(r.Body)
		gotBody = string(buf)
		_, _ = w.Write([]byte(`{"ok":"1"}`))
	}))
	defer srv.Close()

	c := newTestClient(srv)
	if _, err := c.Send(context.Background(), http.MethodPost, "/repositories/acme/widgets/issues",
		map[string]string{"title": "hello"}); err != nil {
		t.Fatalf("send: %v", err)
	}
	if !strings.Contains(gotBody, `"title":"hello"`) {
		t.Fatalf("request body = %q", gotBody)
	}
}

func TestCreateRepository(t *testing.T) {
	var gotMethod, gotPath, gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath = r.Method, r.URL.Path
		buf, _ := io.ReadAll(r.Body)
		gotBody = string(buf)
		_, _ = w.Write([]byte(`{"full_name":"acme/widgets"}`))
	}))
	defer srv.Close()

	private := true
	_, err := newTestClient(srv).CreateRepository(context.Background(), "acme", "widgets",
		CreateRepositoryOptions{Description: "the repo", IsPrivate: &private, ProjectKey: "WID"})
	if err != nil {
		t.Fatalf("CreateRepository: %v", err)
	}
	if gotMethod != http.MethodPost || gotPath != "/repositories/acme/widgets" {
		t.Errorf("request = %s %s", gotMethod, gotPath)
	}
	for _, want := range []string{`"scm":"git"`, `"description":"the repo"`, `"is_private":true`, `"key":"WID"`} {
		if !strings.Contains(gotBody, want) {
			t.Errorf("body %q missing %q", gotBody, want)
		}
	}

	// A nil IsPrivate and empty project/description send only the SCM.
	gotBody = ""
	if _, err := newTestClient(srv).CreateRepository(context.Background(), "acme", "widgets",
		CreateRepositoryOptions{}); err != nil {
		t.Fatalf("CreateRepository: %v", err)
	}
	if strings.Contains(gotBody, "is_private") || strings.Contains(gotBody, "project") {
		t.Errorf("unset fields should be omitted: %q", gotBody)
	}
}

func TestDeleteRepository(t *testing.T) {
	var gotMethod, gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath = r.Method, r.URL.Path
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	if err := newTestClient(srv).DeleteRepository(context.Background(), "acme", "widgets"); err != nil {
		t.Fatalf("DeleteRepository: %v", err)
	}
	if gotMethod != http.MethodDelete || gotPath != "/repositories/acme/widgets" {
		t.Errorf("request = %s %s", gotMethod, gotPath)
	}
}
