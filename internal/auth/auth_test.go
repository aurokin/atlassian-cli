package auth

import (
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/aurokin/atlassian-cli/internal/apperr"
)

func newRequest(t *testing.T) *http.Request {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, "https://example.atlassian.net/rest/api/3/myself", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	return req
}

func TestCloudClassicSignsWithBasicAuth(t *testing.T) {
	c := Credential{Style: StyleCloudClassic, Username: "user@example.com", Token: "classic-token"}
	req := newRequest(t)
	if err := c.Sign(req); err != nil {
		t.Fatalf("Sign: %v", err)
	}
	if got := req.Header.Get("Authorization"); !strings.HasPrefix(got, "Basic ") {
		t.Fatalf("Authorization = %q, want Basic prefix", got)
	}
	user, pass, ok := req.BasicAuth()
	if !ok || user != "user@example.com" || pass != "classic-token" {
		t.Fatalf("BasicAuth = (%q, %q, %v), want classic credentials", user, pass, ok)
	}
}

func TestCloudScopedSignsWithBasicAuthAndRequiresCloudID(t *testing.T) {
	missing := Credential{Style: StyleCloudScoped, Username: "user@example.com", Token: "scoped-token"}
	err := missing.Sign(newRequest(t))
	if err == nil {
		t.Fatal("Sign without cloud_id returned no error")
	}
	var ae *apperr.Error
	if !errors.As(err, &ae) {
		t.Fatalf("error type = %T, want *apperr.Error", err)
	}

	complete := Credential{Style: StyleCloudScoped, Username: "user@example.com", Token: "scoped-token", CloudID: "cloud-123"}
	req := newRequest(t)
	if err := complete.Sign(req); err != nil {
		t.Fatalf("Sign with cloud_id: %v", err)
	}
	if got := req.Header.Get("Authorization"); !strings.HasPrefix(got, "Basic ") {
		t.Fatalf("Authorization = %q, want Basic prefix", got)
	}
}

func TestDataCenterPATSignsWithBearer(t *testing.T) {
	c := Credential{Style: StyleDataCenterPAT, Token: "pat-token"}
	req := newRequest(t)
	if err := c.Sign(req); err != nil {
		t.Fatalf("Sign: %v", err)
	}
	if got := req.Header.Get("Authorization"); got != "Bearer pat-token" {
		t.Fatalf("Authorization = %q, want %q", got, "Bearer pat-token")
	}
}

func TestOAuth3LOSignsWithBearerAndRequiresCloudID(t *testing.T) {
	missing := Credential{Style: StyleOAuth3LO, Token: "access-token"}
	err := missing.Sign(newRequest(t))
	if err == nil {
		t.Fatal("Sign without cloud_id returned no error")
	}
	var ae *apperr.Error
	if !errors.As(err, &ae) {
		t.Fatalf("error type = %T, want *apperr.Error", err)
	}

	// oauth-3lo carries the access token in Token and needs no username.
	complete := Credential{Style: StyleOAuth3LO, Token: "access-token", CloudID: "cloud-123"}
	req := newRequest(t)
	if err := complete.Sign(req); err != nil {
		t.Fatalf("Sign with cloud_id: %v", err)
	}
	if got := req.Header.Get("Authorization"); got != "Bearer access-token" {
		t.Fatalf("Authorization = %q, want %q", got, "Bearer access-token")
	}
}

func TestMissingRequiredFieldsReturnStructuredErrors(t *testing.T) {
	cases := []struct {
		name string
		c    Credential
	}{
		{"missing token", Credential{Style: StyleCloudClassic, Username: "user@example.com"}},
		{"missing username", Credential{Style: StyleCloudClassic, Token: "t"}},
		{"missing cloud id", Credential{Style: StyleCloudScoped, Username: "u", Token: "t"}},
		{"oauth-3lo missing token", Credential{Style: StyleOAuth3LO, CloudID: "c"}},
		{"oauth-3lo missing cloud id", Credential{Style: StyleOAuth3LO, Token: "t"}},
		{"unknown style", Credential{Style: TokenStyle("nonsense"), Token: "t"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.c.Validate()
			if err == nil {
				t.Fatal("Validate returned no error")
			}
			var ae *apperr.Error
			if !errors.As(err, &ae) {
				t.Fatalf("error type = %T, want *apperr.Error", err)
			}
		})
	}
}

func TestAuthTypeMapping(t *testing.T) {
	if got := StyleCloudClassic.AuthType(); got != "api-token-basic" {
		t.Errorf("cloud-classic AuthType = %q, want api-token-basic", got)
	}
	if got := StyleCloudScoped.AuthType(); got != "api-token-basic" {
		t.Errorf("cloud-scoped AuthType = %q, want api-token-basic", got)
	}
	if got := StyleDataCenterPAT.AuthType(); got != "pat-bearer" {
		t.Errorf("data-center-pat AuthType = %q, want pat-bearer", got)
	}
	if got := StyleOAuth3LO.AuthType(); got != "oauth-bearer" {
		t.Errorf("oauth-3lo AuthType = %q, want oauth-bearer", got)
	}
}

func TestParseTokenStyle(t *testing.T) {
	for _, s := range AllStyles {
		got, err := ParseTokenStyle(string(s))
		if err != nil || got != s {
			t.Errorf("ParseTokenStyle(%q) = (%q, %v), want (%q, nil)", s, got, err, s)
		}
	}
	if _, err := ParseTokenStyle("bogus"); err == nil {
		t.Error("ParseTokenStyle(bogus) returned no error")
	}
}
