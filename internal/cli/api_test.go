package cli

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/aurokin/atlassian-cli/internal/apperr"
)

// loginDataCenter records a data-center site pointing at srv and arms its
// token environment variable.
func loginDataCenter(t *testing.T, srvURL string) {
	t.Helper()
	t.Setenv("ATL_API_TOKEN", "test-token")
	if _, err := execRoot(t, jiraInfo(), "auth", "login", "--site", "work",
		"--url", srvURL, "--token-style", "data-center-pat", "--token-env", "ATL_API_TOKEN"); err != nil {
		t.Fatalf("login: %v", err)
	}
}

func TestAPICommandCallsServerAndRendersJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"accountId":"abc","displayName":"Test User"}`))
	}))
	defer srv.Close()

	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	loginDataCenter(t, srv.URL)

	out, err := execRoot(t, jiraInfo(), "api", "/rest/api/2/myself", "--site", "work", "--json=*")
	if err != nil {
		t.Fatalf("api: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("api output is not valid JSON: %v\n%s", err, out)
	}
	want := map[string]any{"accountId": "abc", "displayName": "Test User"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("response not rendered unchanged:\n got %v\nwant %v", got, want)
	}
}

func TestAPICommandSendsBearerForDataCenter(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	loginDataCenter(t, srv.URL)

	if _, err := execRoot(t, jiraInfo(), "api", "/rest/api/2/myself", "--site", "work"); err != nil {
		t.Fatalf("api: %v", err)
	}
	if gotAuth != "Bearer test-token" {
		t.Fatalf("Authorization = %q, want Bearer style", gotAuth)
	}
}

func TestAPICommandMapsNon2xxToStructuredError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"message":"no access"}`))
	}))
	defer srv.Close()

	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	loginDataCenter(t, srv.URL)

	_, err := execRoot(t, jiraInfo(), "api", "/rest/api/2/myself", "--site", "work")
	if err == nil {
		t.Fatal("api returned no error for a 403 response")
	}
	var ae *apperr.Error
	if !errors.As(err, &ae) {
		t.Fatalf("error type = %T, want *apperr.Error", err)
	}
	if ae.Code != apperr.CodeForbidden {
		t.Errorf("Code = %q, want %q", ae.Code, apperr.CodeForbidden)
	}
}

func TestAPICommandRejectsUntrustedAbsoluteURL(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("request reached the server; untrusted URL should be rejected first")
	}))
	defer srv.Close()

	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	loginDataCenter(t, srv.URL)

	_, err := execRoot(t, jiraInfo(), "api", "https://evil.example.com/rest/api/2/myself", "--site", "work")
	if err == nil {
		t.Fatal("api accepted an absolute URL outside the configured site")
	}
	var ae *apperr.Error
	if !errors.As(err, &ae) {
		t.Fatalf("error type = %T, want *apperr.Error", err)
	}
}

func TestAPICommandRequiresConfiguredSite(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	_, err := execRoot(t, jiraInfo(), "api", "/myself", "--site", "absent")
	if err == nil {
		t.Fatal("api returned no error for an unconfigured site")
	}
	var ae *apperr.Error
	if !errors.As(err, &ae) {
		t.Fatalf("error type = %T, want *apperr.Error", err)
	}
}
