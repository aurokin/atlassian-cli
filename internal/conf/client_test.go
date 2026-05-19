package conf

import (
	"context"
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

func TestDecodeRejectsMalformedJSON(t *testing.T) {
	if _, err := Decode[Page]([]byte("not json")); err == nil {
		t.Fatal("Decode accepted malformed JSON")
	}
}
