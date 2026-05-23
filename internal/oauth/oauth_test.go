package oauth

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/aurokin/atlassian-cli/internal/apperr"
)

// fixedClock returns a clock function pinned to t.
func fixedClock(t time.Time) func() time.Time {
	return func() time.Time { return t }
}

func TestAuthorizeURL(t *testing.T) {
	c := New("client-abc", "secret", Options{Endpoints: Endpoints{Authorize: "https://auth.example/authorize"}})
	raw := c.AuthorizeURL(AuthorizeParams{
		RedirectURI:   "http://localhost:8976/callback",
		Scopes:        []string{"read:jira-work", "offline_access"},
		State:         "state-xyz",
		CodeChallenge: "chal-123",
	})
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("parse authorize URL: %v", err)
	}
	if got, want := u.Scheme+"://"+u.Host+u.Path, "https://auth.example/authorize"; got != want {
		t.Fatalf("authorize base = %q, want %q", got, want)
	}
	q := u.Query()
	checks := map[string]string{
		"audience":              "api.atlassian.com",
		"client_id":             "client-abc",
		"scope":                 "read:jira-work offline_access",
		"redirect_uri":          "http://localhost:8976/callback",
		"state":                 "state-xyz",
		"response_type":         "code",
		"code_challenge":        "chal-123",
		"code_challenge_method": "S256",
	}
	for k, want := range checks {
		if got := q.Get(k); got != want {
			t.Errorf("authorize query %q = %q, want %q", k, got, want)
		}
	}
	// Prompt is omitted when empty (lets Atlassian skip already-granted consent).
	if q.Has("prompt") {
		t.Errorf("prompt should be omitted when empty, got %q", q.Get("prompt"))
	}

	// When set, Prompt is forwarded verbatim.
	withPrompt := c.AuthorizeURL(AuthorizeParams{Prompt: "consent"})
	pu, _ := url.Parse(withPrompt)
	if got := pu.Query().Get("prompt"); got != "consent" {
		t.Errorf("prompt = %q, want consent", got)
	}
}

func TestExchangeSendsFormAndParsesBundle(t *testing.T) {
	now := time.Date(2026, 5, 22, 12, 0, 0, 0, time.UTC)
	var gotBody url.Values
	var gotContentType string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		gotContentType = r.Header.Get("Content-Type")
		raw, _ := io.ReadAll(r.Body)
		gotBody, _ = url.ParseQuery(string(raw))
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"access_token":"acc-1","refresh_token":"ref-1","expires_in":3600,"token_type":"Bearer"}`)
	}))
	defer srv.Close()

	c := New("client-abc", "secret-xyz", Options{Endpoints: Endpoints{Token: srv.URL}, Now: fixedClock(now)})
	bundle, err := c.Exchange(context.Background(), "the-code", "the-verifier", "http://localhost:8976/callback")
	if err != nil {
		t.Fatalf("Exchange: %v", err)
	}

	if gotContentType != "application/x-www-form-urlencoded" {
		t.Errorf("Content-Type = %q", gotContentType)
	}
	wantForm := map[string]string{
		"grant_type":    "authorization_code",
		"client_id":     "client-abc",
		"client_secret": "secret-xyz",
		"code":          "the-code",
		"redirect_uri":  "http://localhost:8976/callback",
		"code_verifier": "the-verifier",
	}
	for k, want := range wantForm {
		if got := gotBody.Get(k); got != want {
			t.Errorf("form %q = %q, want %q", k, got, want)
		}
	}
	if bundle.AccessToken != "acc-1" || bundle.RefreshToken != "ref-1" {
		t.Errorf("bundle tokens = (%q, %q)", bundle.AccessToken, bundle.RefreshToken)
	}
	if want := now.Add(time.Hour); !bundle.Expiry.Equal(want) {
		t.Errorf("expiry = %v, want %v", bundle.Expiry, want)
	}
}

func TestRefreshSendsRefreshGrant(t *testing.T) {
	now := time.Date(2026, 5, 22, 12, 0, 0, 0, time.UTC)
	var gotBody url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		gotBody, _ = url.ParseQuery(string(raw))
		w.Header().Set("Content-Type", "application/json")
		// Atlassian rotates the refresh token.
		_, _ = io.WriteString(w, `{"access_token":"acc-2","refresh_token":"ref-2","expires_in":7200}`)
	}))
	defer srv.Close()

	c := New("client-abc", "secret-xyz", Options{Endpoints: Endpoints{Token: srv.URL}, Now: fixedClock(now)})
	bundle, err := c.Refresh(context.Background(), "old-refresh")
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if got := gotBody.Get("grant_type"); got != "refresh_token" {
		t.Errorf("grant_type = %q, want refresh_token", got)
	}
	if got := gotBody.Get("refresh_token"); got != "old-refresh" {
		t.Errorf("refresh_token = %q, want old-refresh", got)
	}
	if bundle.AccessToken != "acc-2" || bundle.RefreshToken != "ref-2" {
		t.Errorf("rotated bundle = (%q, %q)", bundle.AccessToken, bundle.RefreshToken)
	}
	if want := now.Add(2 * time.Hour); !bundle.Expiry.Equal(want) {
		t.Errorf("expiry = %v, want %v", bundle.Expiry, want)
	}
}

func TestInvalidGrantMapsToReauthError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = io.WriteString(w, `{"error":"invalid_grant","error_description":"refresh token is invalid"}`)
	}))
	defer srv.Close()

	c := New("client-abc", "secret-xyz", Options{Endpoints: Endpoints{Token: srv.URL}})
	_, err := c.Refresh(context.Background(), "revoked")
	if err == nil {
		t.Fatal("Refresh with invalid_grant returned no error")
	}
	var ae *apperr.Error
	if !errors.As(err, &ae) {
		t.Fatalf("error type = %T, want *apperr.Error", err)
	}
	if ae.Code != apperr.CodeUnauthorized {
		t.Errorf("code = %q, want %q", ae.Code, apperr.CodeUnauthorized)
	}
	if ae.Next == "" {
		t.Error("expected a Next re-auth hint")
	}
}

func TestGenericTokenErrorIsNotReauth(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = io.WriteString(w, `{"error":"invalid_client","error_description":"client authentication failed"}`)
	}))
	defer srv.Close()

	c := New("client-abc", "bad-secret", Options{Endpoints: Endpoints{Token: srv.URL}})
	_, err := c.Exchange(context.Background(), "code", "verifier", "http://localhost:8976/callback")
	var ae *apperr.Error
	if !errors.As(err, &ae) {
		t.Fatalf("error type = %T, want *apperr.Error", err)
	}
	if ae.Code != "oauth_error" {
		t.Errorf("code = %q, want oauth_error", ae.Code)
	}
	if !strings.Contains(ae.Message, "invalid_client") {
		t.Errorf("message %q should surface the OAuth error code", ae.Message)
	}
}

func TestAccessibleResources(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `[{"id":"cloud-1","url":"https://acme.atlassian.net","name":"Acme","scopes":["read:jira-work"]}]`)
	}))
	defer srv.Close()

	c := New("client-abc", "secret", Options{Endpoints: Endpoints{Resources: srv.URL}})
	resources, err := c.AccessibleResources(context.Background(), "acc-token")
	if err != nil {
		t.Fatalf("AccessibleResources: %v", err)
	}
	if gotAuth != "Bearer acc-token" {
		t.Errorf("Authorization = %q, want Bearer acc-token", gotAuth)
	}
	if len(resources) != 1 || resources[0].ID != "cloud-1" || resources[0].URL != "https://acme.atlassian.net" {
		t.Fatalf("resources = %+v", resources)
	}
}

func TestAccessibleResourcesUnauthorizedIsReauth(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	c := New("client-abc", "secret", Options{Endpoints: Endpoints{Resources: srv.URL}})
	_, err := c.AccessibleResources(context.Background(), "expired")
	var ae *apperr.Error
	if !errors.As(err, &ae) {
		t.Fatalf("error type = %T, want *apperr.Error", err)
	}
	if ae.Code != apperr.CodeUnauthorized || ae.Next == "" {
		t.Errorf("want unauthorized + re-auth hint, got code=%q next=%q", ae.Code, ae.Next)
	}
}

func TestEnsureOfflineAccess(t *testing.T) {
	// Appends when absent.
	got := EnsureOfflineAccess([]string{"read:jira-work"})
	if len(got) != 2 || got[1] != "offline_access" {
		t.Errorf("EnsureOfflineAccess(absent) = %v", got)
	}
	// No duplicate when already present.
	got = EnsureOfflineAccess([]string{"offline_access", "read:jira-work"})
	if len(got) != 2 {
		t.Errorf("EnsureOfflineAccess(present) = %v, want no duplicate", got)
	}
	// Empty input still requests offline_access.
	got = EnsureOfflineAccess(nil)
	if len(got) != 1 || got[0] != "offline_access" {
		t.Errorf("EnsureOfflineAccess(nil) = %v", got)
	}
}

func TestGeneratePKCEProducesValidS256(t *testing.T) {
	p, err := GeneratePKCE()
	if err != nil {
		t.Fatalf("GeneratePKCE: %v", err)
	}
	if p.Verifier == "" || p.Challenge == "" {
		t.Fatal("empty PKCE pair")
	}
	sum := sha256.Sum256([]byte(p.Verifier))
	want := base64.RawURLEncoding.EncodeToString(sum[:])
	if p.Challenge != want {
		t.Errorf("challenge is not S256(verifier): got %q want %q", p.Challenge, want)
	}
	// Two calls must differ (randomness).
	p2, _ := GeneratePKCE()
	if p2.Verifier == p.Verifier {
		t.Error("two PKCE verifiers were identical")
	}
}

func TestBundleMarshalParseRoundTrip(t *testing.T) {
	exp := time.Date(2026, 5, 22, 13, 0, 0, 0, time.UTC)
	in := TokenBundle{
		ClientSecret: "secret-xyz",
		AccessToken:  "acc-1",
		RefreshToken: "ref-1",
		Expiry:       exp,
	}
	s, err := in.Marshal()
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	out, err := ParseBundle(s)
	if err != nil {
		t.Fatalf("ParseBundle: %v", err)
	}
	if out.ClientSecret != in.ClientSecret || out.AccessToken != in.AccessToken ||
		out.RefreshToken != in.RefreshToken || !out.Expiry.Equal(in.Expiry) {
		t.Fatalf("round-trip mismatch: %+v vs %+v", out, in)
	}
}

func TestParseBundleRejectsInvalidJSON(t *testing.T) {
	if _, err := ParseBundle("not json"); err == nil {
		t.Fatal("ParseBundle of invalid JSON returned no error")
	}
}

func TestBundleExpired(t *testing.T) {
	now := time.Date(2026, 5, 22, 12, 0, 0, 0, time.UTC)
	if !(TokenBundle{}).Expired(now) {
		t.Error("zero-expiry bundle should be treated as expired")
	}
	if !(TokenBundle{Expiry: now.Add(-time.Second)}).Expired(now) {
		t.Error("past-expiry bundle should be expired")
	}
	if (TokenBundle{Expiry: now.Add(time.Hour)}).Expired(now) {
		t.Error("future-expiry bundle should not be expired")
	}
	// Exactly at expiry counts as expired.
	if !(TokenBundle{Expiry: now}).Expired(now) {
		t.Error("bundle expiring exactly now should be expired")
	}
}

func TestGenerateStateIsRandom(t *testing.T) {
	a, err := GenerateState()
	if err != nil {
		t.Fatalf("GenerateState: %v", err)
	}
	b, _ := GenerateState()
	if a == "" || a == b {
		t.Errorf("state not random: %q vs %q", a, b)
	}
}
