package httpclient

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/aurokin/atlassian-cli/internal/apperr"
	"github.com/aurokin/atlassian-cli/internal/auth"
)

func TestAPIBaseJiraCloud(t *testing.T) {
	classic := Target{Product: ProductJira, TokenStyle: auth.StyleCloudClassic, BaseURL: "https://example.atlassian.net"}
	if got, err := classic.APIBase(); err != nil || got != "https://example.atlassian.net/rest/api/3" {
		t.Fatalf("classic APIBase = (%q, %v)", got, err)
	}

	scoped := Target{Product: ProductJira, TokenStyle: auth.StyleCloudScoped, CloudID: "cloud-123"}
	if got, err := scoped.APIBase(); err != nil || got != "https://api.atlassian.com/ex/jira/cloud-123/rest/api/3" {
		t.Fatalf("scoped APIBase = (%q, %v)", got, err)
	}
}

func TestAPIBaseConfluenceCloud(t *testing.T) {
	// "/wiki" is added when absent and not duplicated when already present.
	noWiki := Target{Product: ProductConfluence, TokenStyle: auth.StyleCloudClassic, BaseURL: "https://example.atlassian.net"}
	if got, err := noWiki.APIBase(); err != nil || got != "https://example.atlassian.net/wiki/api/v2" {
		t.Fatalf("classic APIBase (no /wiki) = (%q, %v)", got, err)
	}
	withWiki := Target{Product: ProductConfluence, TokenStyle: auth.StyleCloudClassic, BaseURL: "https://example.atlassian.net/wiki"}
	if got, err := withWiki.APIBase(); err != nil || got != "https://example.atlassian.net/wiki/api/v2" {
		t.Fatalf("classic APIBase (with /wiki) = (%q, %v)", got, err)
	}

	scoped := Target{Product: ProductConfluence, TokenStyle: auth.StyleCloudScoped, CloudID: "cloud-123"}
	if got, err := scoped.APIBase(); err != nil || got != "https://api.atlassian.com/ex/confluence/cloud-123/wiki/api/v2" {
		t.Fatalf("scoped APIBase = (%q, %v)", got, err)
	}
}

func TestAPIBaseStripsUserinfo(t *testing.T) {
	// A credential embedded in the configured URL must not survive into the
	// API base, which is surfaced in diagnostics and persisted to config.
	target := Target{Product: ProductJira, TokenStyle: auth.StyleCloudClassic, BaseURL: "https://user:s3cret@example.atlassian.net"}
	got, err := target.APIBase()
	if err != nil {
		t.Fatalf("APIBase: %v", err)
	}
	if got != "https://example.atlassian.net/rest/api/3" {
		t.Fatalf("APIBase kept userinfo: %q", got)
	}
}

func TestResolveURLRelativePathDropsBaseUserinfo(t *testing.T) {
	target := Target{Product: ProductJira, TokenStyle: auth.StyleCloudClassic, BaseURL: "https://user:s3cret@example.atlassian.net"}
	got, err := target.ResolveURL("/myself")
	if err != nil {
		t.Fatalf("ResolveURL: %v", err)
	}
	if got != "https://example.atlassian.net/rest/api/3/myself" {
		t.Fatalf("ResolveURL kept base userinfo: %q", got)
	}
}

func TestAPIBaseDataCenterUsesConfiguredBase(t *testing.T) {
	dc := Target{Product: ProductJira, TokenStyle: auth.StyleDataCenterPAT, BaseURL: "https://jira.internal.example.com/"}
	got, err := dc.APIBase()
	if err != nil {
		t.Fatalf("APIBase: %v", err)
	}
	if got != "https://jira.internal.example.com" {
		t.Fatalf("APIBase = %q, want the configured base verbatim", got)
	}
}

func TestAPIBaseScopedRequiresCloudID(t *testing.T) {
	_, err := Target{Product: ProductJira, TokenStyle: auth.StyleCloudScoped}.APIBase()
	if err == nil {
		t.Fatal("scoped APIBase without cloud_id returned no error")
	}
	var ae *apperr.Error
	if !errors.As(err, &ae) {
		t.Fatalf("error type = %T, want *apperr.Error", err)
	}
}

func TestAPIBaseOAuth3LOReusesGateway(t *testing.T) {
	// oauth-3lo resolves to the same api.atlassian.com gateway base as a
	// cloud-scoped token; only the Authorization scheme differs (handled by
	// auth.Credential.Sign), not the URL.
	jira := Target{Product: ProductJira, TokenStyle: auth.StyleOAuth3LO, CloudID: "cloud-123"}
	if got, err := jira.APIBase(); err != nil || got != "https://api.atlassian.com/ex/jira/cloud-123/rest/api/3" {
		t.Fatalf("oauth-3lo Jira APIBase = (%q, %v)", got, err)
	}
	conf := Target{Product: ProductConfluence, TokenStyle: auth.StyleOAuth3LO, CloudID: "cloud-123"}
	if got, err := conf.APIBase(); err != nil || got != "https://api.atlassian.com/ex/confluence/cloud-123/wiki/api/v2" {
		t.Fatalf("oauth-3lo Confluence APIBase = (%q, %v)", got, err)
	}
}

func TestAPIBaseOAuth3LORequiresCloudID(t *testing.T) {
	_, err := Target{Product: ProductJira, TokenStyle: auth.StyleOAuth3LO}.APIBase()
	if err == nil {
		t.Fatal("oauth-3lo APIBase without cloud_id returned no error")
	}
	var ae *apperr.Error
	if !errors.As(err, &ae) {
		t.Fatalf("error type = %T, want *apperr.Error", err)
	}
}

func TestResolveURLRelativePath(t *testing.T) {
	target := Target{Product: ProductJira, TokenStyle: auth.StyleCloudClassic, BaseURL: "https://example.atlassian.net"}
	got, err := target.ResolveURL("/myself")
	if err != nil {
		t.Fatalf("ResolveURL: %v", err)
	}
	if got != "https://example.atlassian.net/rest/api/3/myself" {
		t.Fatalf("ResolveURL = %q", got)
	}
}

func TestResolveURLRejectsUntrustedAbsoluteURL(t *testing.T) {
	target := Target{Product: ProductJira, TokenStyle: auth.StyleCloudClassic, BaseURL: "https://example.atlassian.net", SiteName: "work"}
	_, err := target.ResolveURL("https://evil.example.com/rest/api/3/myself")
	if err == nil {
		t.Fatal("ResolveURL accepted an untrusted absolute URL")
	}
	var ae *apperr.Error
	if !errors.As(err, &ae) {
		t.Fatalf("error type = %T, want *apperr.Error", err)
	}
}

func TestResolveURLAllowsConfiguredSiteAbsoluteURL(t *testing.T) {
	target := Target{Product: ProductJira, TokenStyle: auth.StyleCloudClassic, BaseURL: "https://example.atlassian.net"}
	got, err := target.ResolveURL("https://example.atlassian.net/rest/api/3/myself")
	if err != nil {
		t.Fatalf("ResolveURL rejected the configured site: %v", err)
	}
	if got != "https://example.atlassian.net/rest/api/3/myself" {
		t.Fatalf("ResolveURL = %q", got)
	}
}

func TestResolveURLRejectsSchemeDowngrade(t *testing.T) {
	// An https site must not accept a plaintext-http absolute URL: the signed
	// token would otherwise travel in cleartext.
	target := Target{Product: ProductJira, TokenStyle: auth.StyleCloudClassic, BaseURL: "https://example.atlassian.net", SiteName: "work"}
	_, err := target.ResolveURL("http://example.atlassian.net/rest/api/3/myself")
	if err == nil {
		t.Fatal("ResolveURL accepted an http:// downgrade of an https site")
	}
	var ae *apperr.Error
	if !errors.As(err, &ae) {
		t.Fatalf("error type = %T, want *apperr.Error", err)
	}
}

func TestResolveURLAllowsMixedCaseHost(t *testing.T) {
	target := Target{Product: ProductJira, TokenStyle: auth.StyleCloudClassic, BaseURL: "https://example.atlassian.net"}
	got, err := target.ResolveURL("https://Example.Atlassian.NET/rest/api/3/myself")
	if err != nil {
		t.Fatalf("ResolveURL rejected a same-host URL on host-case difference: %v", err)
	}
	if got != "https://Example.Atlassian.NET/rest/api/3/myself" {
		t.Fatalf("ResolveURL = %q", got)
	}
}

func TestResolveURLStripsUserinfoFromAbsoluteURL(t *testing.T) {
	// Credentials embedded in a URL must not survive into the request URL,
	// where they could travel on the wire or leak into an error message.
	target := Target{Product: ProductJira, TokenStyle: auth.StyleCloudClassic, BaseURL: "https://example.atlassian.net"}
	got, err := target.ResolveURL("https://user:s3cret@example.atlassian.net/rest/api/3/myself")
	if err != nil {
		t.Fatalf("ResolveURL: %v", err)
	}
	if got != "https://example.atlassian.net/rest/api/3/myself" {
		t.Fatalf("ResolveURL kept URL userinfo: %q", got)
	}
}

func TestDoSuccessReturnsBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got == "" {
			t.Errorf("missing Authorization header")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"accountId":"abc"}`))
	}))
	defer srv.Close()

	client := New(
		Target{Product: ProductJira, TokenStyle: auth.StyleDataCenterPAT, BaseURL: srv.URL},
		auth.Credential{Style: auth.StyleDataCenterPAT, Token: "pat"},
		srv.Client(),
	)
	resp, err := client.Do(context.Background(), http.MethodGet, "/rest/api/2/myself", nil)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	if resp.Status != http.StatusOK {
		t.Fatalf("Status = %d, want 200", resp.Status)
	}
	if string(resp.Body) != `{"accountId":"abc"}` {
		t.Fatalf("Body = %q", resp.Body)
	}
}

func TestDoSetsDefaultJSONAccept(t *testing.T) {
	var gotAccept string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAccept = r.Header.Get("Accept")
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	client := New(
		Target{Product: ProductJira, TokenStyle: auth.StyleDataCenterPAT, BaseURL: srv.URL},
		auth.Credential{Style: auth.StyleDataCenterPAT, Token: "pat"},
		srv.Client(),
	)
	if _, err := client.Do(context.Background(), http.MethodGet, "/x", nil); err != nil {
		t.Fatalf("Do: %v", err)
	}
	if gotAccept != "application/json" {
		t.Fatalf("Accept = %q, want application/json", gotAccept)
	}
}

func TestDoAcceptingSetsCustomAccept(t *testing.T) {
	var gotAccept string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAccept = r.Header.Get("Accept")
		_, _ = w.Write([]byte(`binary`))
	}))
	defer srv.Close()

	client := New(
		Target{Product: ProductJira, TokenStyle: auth.StyleDataCenterPAT, BaseURL: srv.URL},
		auth.Credential{Style: auth.StyleDataCenterPAT, Token: "pat"},
		srv.Client(),
	)
	if _, err := client.DoAccepting(context.Background(), http.MethodGet, "/blob", nil, "*/*"); err != nil {
		t.Fatalf("DoAccepting: %v", err)
	}
	if gotAccept != "*/*" {
		t.Fatalf("Accept = %q, want */*", gotAccept)
	}
}

func TestDoCallsCredentialProviderPerRequest(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	calls := 0
	client := NewWithProvider(
		Target{Product: ProductJira, TokenStyle: auth.StyleOAuth3LO, CloudID: "c", BaseURL: srv.URL},
		func(context.Context) (auth.Credential, error) {
			calls++
			return auth.Credential{Style: auth.StyleOAuth3LO, Token: "access-" + strconv.Itoa(calls), CloudID: "c"}, nil
		},
		srv.Client(),
	)
	// The provider is consulted on every request (so a refreshed token is
	// picked up), and oauth-3lo signs Bearer.
	if _, err := client.Do(context.Background(), http.MethodGet, srv.URL+"/x", nil); err != nil {
		t.Fatalf("Do 1: %v", err)
	}
	if gotAuth != "Bearer access-1" {
		t.Fatalf("first auth = %q", gotAuth)
	}
	if _, err := client.Do(context.Background(), http.MethodGet, srv.URL+"/x", nil); err != nil {
		t.Fatalf("Do 2: %v", err)
	}
	if gotAuth != "Bearer access-2" {
		t.Fatalf("second auth = %q, want the provider re-consulted", gotAuth)
	}
}

func TestDoPropagatesCredentialProviderError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("request should not be sent when the provider fails")
	}))
	defer srv.Close()

	wantErr := apperr.Unauthorized("refresh failed; re-authenticate")
	client := NewWithProvider(
		Target{Product: ProductJira, TokenStyle: auth.StyleOAuth3LO, CloudID: "c", BaseURL: srv.URL},
		func(context.Context) (auth.Credential, error) { return auth.Credential{}, wantErr },
		srv.Client(),
	)
	_, err := client.Do(context.Background(), http.MethodGet, srv.URL+"/x", nil)
	if !errors.Is(err, wantErr) {
		t.Fatalf("Do error = %v, want the provider error propagated", err)
	}
}

func TestDoSignsBasicForCloudClassic(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	client := New(
		Target{Product: ProductJira, TokenStyle: auth.StyleCloudClassic, BaseURL: srv.URL},
		auth.Credential{Style: auth.StyleCloudClassic, Username: "user@example.com", Token: "classic-token"},
		srv.Client(),
	)
	if _, err := client.Do(context.Background(), http.MethodGet, "/myself", nil); err != nil {
		t.Fatalf("Do: %v", err)
	}
	if !strings.HasPrefix(gotAuth, "Basic ") {
		t.Fatalf("Authorization = %q, want Basic style", gotAuth)
	}
}

func TestDoMapsErrorStatuses(t *testing.T) {
	cases := []struct {
		status   int
		wantCode string
	}{
		{http.StatusUnauthorized, apperr.CodeUnauthorized},
		{http.StatusForbidden, apperr.CodeForbidden},
		{http.StatusNotFound, apperr.CodeNotFoundOrNotVisible},
		{http.StatusTooManyRequests, apperr.CodeRateLimited},
		{http.StatusGone, apperr.CodeGone},
		{http.StatusInternalServerError, apperr.CodeHTTPError},
	}
	for _, tc := range cases {
		t.Run(http.StatusText(tc.status), func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if tc.status == http.StatusTooManyRequests {
					w.Header().Set("Retry-After", "30")
				}
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(`{"message":"server says no"}`))
			}))
			defer srv.Close()

			client := New(
				Target{Product: ProductJira, TokenStyle: auth.StyleDataCenterPAT, BaseURL: srv.URL, SiteName: "work"},
				auth.Credential{Style: auth.StyleDataCenterPAT, Token: "pat"},
				srv.Client(),
			)
			resp, err := client.Do(context.Background(), http.MethodGet, "/rest/api/2/myself", nil)
			if err == nil {
				t.Fatal("Do returned no error for a non-2xx status")
			}
			var ae *apperr.Error
			if !errors.As(err, &ae) {
				t.Fatalf("error type = %T, want *apperr.Error", err)
			}
			if ae.Code != tc.wantCode {
				t.Errorf("Code = %q, want %q", ae.Code, tc.wantCode)
			}
			if ae.Message != "server says no" {
				t.Errorf("Message = %q, want extracted body message", ae.Message)
			}
			if ae.Site != "work" {
				t.Errorf("Site = %q, want enriched site name", ae.Site)
			}
			if resp == nil || resp.Status != tc.status {
				t.Errorf("Response not returned alongside error")
			}
			if tc.status == http.StatusTooManyRequests && ae.Next == "" {
				t.Error("rate-limit error missing retry guidance")
			}
			if tc.status == http.StatusGone && ae.Next == "" {
				t.Error("gone error missing upgrade guidance")
			}
		})
	}
}

// TestDoClassifiesTimeout verifies that a request that exceeds the context
// deadline is mapped to the retryable timeout category, not request_failed.
func TestDoClassifiesTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done() // never reply; let the client deadline fire
	}))
	defer srv.Close()

	client := New(
		Target{Product: ProductJira, TokenStyle: auth.StyleDataCenterPAT, BaseURL: srv.URL, SiteName: "work"},
		auth.Credential{Style: auth.StyleDataCenterPAT, Token: "pat"},
		srv.Client(),
	)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := client.Do(ctx, http.MethodGet, "/rest/api/2/myself", nil)
	if err == nil {
		t.Fatal("Do returned no error for a timed-out request")
	}
	var ae *apperr.Error
	if !errors.As(err, &ae) {
		t.Fatalf("error type = %T, want *apperr.Error", err)
	}
	if ae.Code != apperr.CodeTimeout {
		t.Fatalf("Code = %q, want %q", ae.Code, apperr.CodeTimeout)
	}
	if ae.Next == "" {
		t.Error("timeout error missing retry guidance")
	}
}
