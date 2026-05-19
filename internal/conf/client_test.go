package conf

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/aurokin/atlassian-cli/internal/apperr"
	"github.com/aurokin/atlassian-cli/internal/auth"
	"github.com/aurokin/atlassian-cli/internal/httpclient"
)

// newTestClient builds a Confluence client whose requests are routed to srv.
// The data-center token style makes the configured URL the API base verbatim,
// so API-relative paths land on the test server unchanged.
func newTestClient(srv *httptest.Server) *Client {
	target := httpclient.Target{
		Product:    httpclient.ProductConfluence,
		TokenStyle: auth.StyleDataCenterPAT,
		SiteName:   "test",
		BaseURL:    srv.URL,
	}
	cred := auth.Credential{Style: auth.StyleDataCenterPAT, Token: "test-token"}
	return New(httpclient.New(target, cred, srv.Client()))
}

// serveJSON builds a test server that asserts the request is a GET against
// wantPath, records the query, and replies with body.
func serveJSON(t *testing.T, wantPath, body string) (*httptest.Server, *url.Values) {
	t.Helper()
	var query url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("request method = %q, want GET", r.Method)
		}
		if r.URL.Path != wantPath {
			t.Errorf("request path = %q, want %q", r.URL.Path, wantPath)
		}
		query = r.URL.Query()
		_, _ = w.Write([]byte(body))
	}))
	return srv, &query
}

func TestClientListSpaces(t *testing.T) {
	srv, query := serveJSON(t, "/spaces",
		`{"results":[{"id":"1","key":"DEV","name":"Development","type":"global"}]}`)
	defer srv.Close()

	raw, err := newTestClient(srv).ListSpaces(context.Background(), 25)
	if err != nil {
		t.Fatalf("ListSpaces: %v", err)
	}
	if got := query.Get("limit"); got != "25" {
		t.Errorf("limit param = %q, want 25", got)
	}
	page, err := Decode[SpacePage](raw)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if len(page.Results) != 1 || page.Results[0].Key != "DEV" {
		t.Fatalf("spaces = %+v", page.Results)
	}
}

func TestClientGetSpace(t *testing.T) {
	srv, _ := serveJSON(t, "/spaces/1",
		`{"id":"1","key":"DEV","name":"Development","type":"global"}`)
	defer srv.Close()

	raw, err := newTestClient(srv).GetSpace(context.Background(), "1")
	if err != nil {
		t.Fatalf("GetSpace: %v", err)
	}
	s, err := Decode[Space](raw)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if s.Key != "DEV" || s.Name != "Development" {
		t.Fatalf("space = %+v", s)
	}
}

func TestClientFindSpaceByKey(t *testing.T) {
	srv, query := serveJSON(t, "/spaces",
		`{"results":[{"id":"1","key":"DEV","name":"Development"}]}`)
	defer srv.Close()

	raw, err := newTestClient(srv).FindSpaceByKey(context.Background(), "DEV")
	if err != nil {
		t.Fatalf("FindSpaceByKey: %v", err)
	}
	if got := query.Get("keys"); got != "DEV" {
		t.Errorf("keys param = %q, want DEV", got)
	}
	page, err := Decode[SpacePage](raw)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if len(page.Results) != 1 || page.Results[0].ID != "1" {
		t.Fatalf("spaces = %+v", page.Results)
	}
}

func TestClientListPages(t *testing.T) {
	srv, query := serveJSON(t, "/pages",
		`{"results":[{"id":"10","title":"Home","status":"current","spaceId":"1"}]}`)
	defer srv.Close()

	raw, err := newTestClient(srv).ListPages(context.Background(), "1", 5)
	if err != nil {
		t.Fatalf("ListPages: %v", err)
	}
	if got := query.Get("space-id"); got != "1" {
		t.Errorf("space-id param = %q, want 1", got)
	}
	if got := query.Get("limit"); got != "5" {
		t.Errorf("limit param = %q, want 5", got)
	}
	page, err := Decode[PageList](raw)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if len(page.Results) != 1 || page.Results[0].Title != "Home" {
		t.Fatalf("pages = %+v", page.Results)
	}
}

func TestClientGetPage(t *testing.T) {
	srv, query := serveJSON(t, "/pages/10",
		`{"id":"10","title":"Home","status":"current","spaceId":"1","version":{"number":3}}`)
	defer srv.Close()

	raw, err := newTestClient(srv).GetPage(context.Background(), "10")
	if err != nil {
		t.Fatalf("GetPage: %v", err)
	}
	if got := query.Get("body-format"); got != "storage" {
		t.Errorf("body-format param = %q, want storage", got)
	}
	p, err := Decode[Page](raw)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if p.Title != "Home" || p.Version.Number != 3 {
		t.Fatalf("page = %+v", p)
	}
}

func TestClientGetChildPages(t *testing.T) {
	srv, _ := serveJSON(t, "/pages/10/children",
		`{"results":[{"id":"11","title":"Child","status":"current","spaceId":"1"}]}`)
	defer srv.Close()

	raw, err := newTestClient(srv).GetChildPages(context.Background(), "10", 0)
	if err != nil {
		t.Fatalf("GetChildPages: %v", err)
	}
	page, err := Decode[PageList](raw)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if len(page.Results) != 1 || page.Results[0].ID != "11" {
		t.Fatalf("children = %+v", page.Results)
	}
}

func TestClientCurrentUser(t *testing.T) {
	srv, _ := serveJSON(t, "/rest/api/user/current",
		`{"accountId":"a1","displayName":"Test User"}`)
	defer srv.Close()

	raw, err := newTestClient(srv).CurrentUser(context.Background())
	if err != nil {
		t.Fatalf("CurrentUser: %v", err)
	}
	user, err := Decode[User](raw)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if user.AccountID != "a1" || user.DisplayName != "Test User" {
		t.Fatalf("user = %+v", user)
	}
}

func TestClientSearchCQL(t *testing.T) {
	srv, query := serveJSON(t, "/rest/api/search",
		`{"results":[{"content":{"id":"10","type":"page","title":"Home"}}]}`)
	defer srv.Close()

	raw, err := newTestClient(srv).SearchCQL(context.Background(), "type = page", 7)
	if err != nil {
		t.Fatalf("SearchCQL: %v", err)
	}
	if got := query.Get("cql"); got != "type = page" {
		t.Errorf("cql param = %q, want %q", got, "type = page")
	}
	if got := query.Get("limit"); got != "7" {
		t.Errorf("limit param = %q, want 7", got)
	}
	results, err := Decode[SearchResults](raw)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if len(results.Results) != 1 || results.Results[0].Content.ID != "10" {
		t.Fatalf("results = %+v", results.Results)
	}
}

func TestClientMapsNonOKToStructuredError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"Space not found"}`))
	}))
	defer srv.Close()

	_, err := newTestClient(srv).GetSpace(context.Background(), "404")
	if err == nil {
		t.Fatal("GetSpace of a missing space returned no error")
	}
	var ae *apperr.Error
	if !errors.As(err, &ae) {
		t.Fatalf("error type = %T, want *apperr.Error", err)
	}
	if ae.Code != apperr.CodeNotFoundOrNotVisible {
		t.Errorf("Code = %q, want %q", ae.Code, apperr.CodeNotFoundOrNotVisible)
	}
}

func TestClientCreatePage(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want POST", r.Method)
		}
		if r.URL.Path != "/pages" {
			t.Errorf("path = %q, want /pages", r.URL.Path)
		}
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_, _ = w.Write([]byte(`{"id":"99","title":"New","status":"current","version":{"number":1}}`))
	}))
	defer srv.Close()

	raw, err := newTestClient(srv).CreatePage(context.Background(), "1", "New", "storage", "<p>hi</p>")
	if err != nil {
		t.Fatalf("CreatePage: %v", err)
	}
	if gotBody["spaceId"] != "1" || gotBody["title"] != "New" {
		t.Errorf("CreatePage sent spaceId/title = %v/%v", gotBody["spaceId"], gotBody["title"])
	}
	body, _ := gotBody["body"].(map[string]any)
	if body["representation"] != "storage" || body["value"] != "<p>hi</p>" {
		t.Errorf("CreatePage body = %v, want storage/<p>hi</p>", gotBody["body"])
	}
	p, err := Decode[Page](raw)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if p.ID != "99" {
		t.Fatalf("page = %+v", p)
	}
}

func TestClientUpdatePage(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("method = %q, want PUT", r.Method)
		}
		if r.URL.Path != "/pages/10" {
			t.Errorf("path = %q, want /pages/10", r.URL.Path)
		}
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_, _ = w.Write([]byte(`{"id":"10","title":"New","status":"current","version":{"number":4}}`))
	}))
	defer srv.Close()

	raw, err := newTestClient(srv).UpdatePage(context.Background(), "10", "current", "New", "storage", "<p>v4</p>", 4)
	if err != nil {
		t.Fatalf("UpdatePage: %v", err)
	}
	if gotBody["id"] != "10" || gotBody["status"] != "current" || gotBody["title"] != "New" {
		t.Errorf("UpdatePage sent id/status/title = %v/%v/%v",
			gotBody["id"], gotBody["status"], gotBody["title"])
	}
	ver, _ := gotBody["version"].(map[string]any)
	if ver["number"] != float64(4) {
		t.Errorf("UpdatePage version = %v, want number 4", gotBody["version"])
	}
	p, err := Decode[Page](raw)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if p.Version.Number != 4 {
		t.Fatalf("page = %+v", p)
	}
}

func TestSearchCQLMapsNonOKToStructuredError(t *testing.T) {
	// SearchCQL is a v1 endpoint; this confirms v1 calls also surface the
	// structured error from httpclient, and exercises a non-404 mapping.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"message":"You do not have permission."}`))
	}))
	defer srv.Close()

	_, err := newTestClient(srv).SearchCQL(context.Background(), "type = page", 0)
	if err == nil {
		t.Fatal("SearchCQL against a 403 endpoint returned no error")
	}
	var ae *apperr.Error
	if !errors.As(err, &ae) {
		t.Fatalf("error type = %T, want *apperr.Error", err)
	}
	if ae.Code != apperr.CodeForbidden {
		t.Errorf("Code = %q, want %q", ae.Code, apperr.CodeForbidden)
	}
}

// TestV1URLDerivedFromCloudAPIBase confirms the v1 base derivation for a Cloud
// target, where the API base carries the "/wiki/api/v2" suffix: the trailing
// "/api/v2" segment is swapped for "/rest/api". The data-center helper cannot
// cover this because its API base is the configured URL verbatim.
func TestV1URLDerivedFromCloudAPIBase(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = w.Write([]byte(`{"accountId":"a1","displayName":"Test User"}`))
	}))
	defer srv.Close()

	target := httpclient.Target{
		Product:    httpclient.ProductConfluence,
		TokenStyle: auth.StyleCloudClassic,
		SiteName:   "test",
		BaseURL:    srv.URL,
	}
	cred := auth.Credential{
		Style:    auth.StyleCloudClassic,
		Username: "tester@example.com",
		Token:    "test-token",
	}
	cc := New(httpclient.New(target, cred, srv.Client()))

	if _, err := cc.CurrentUser(context.Background()); err != nil {
		t.Fatalf("CurrentUser: %v", err)
	}
	if gotPath != "/wiki/rest/api/user/current" {
		t.Errorf("request path = %q, want /wiki/rest/api/user/current", gotPath)
	}
}

func TestDecodeRejectsMalformedJSON(t *testing.T) {
	if _, err := Decode[Page]([]byte("not json")); err == nil {
		t.Fatal("Decode accepted malformed JSON")
	}
}
