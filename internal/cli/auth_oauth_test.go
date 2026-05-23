package cli

import (
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/zalando/go-keyring"

	"github.com/aurokin/atlassian-cli/internal/apperr"
	"github.com/aurokin/atlassian-cli/internal/config"
	"github.com/aurokin/atlassian-cli/internal/oauth"
	"github.com/aurokin/atlassian-cli/internal/secrets"
)

// swapOAuthEndpoints points the oauth-3lo login flow at test servers and
// returns a restore func.
func swapOAuthEndpoints(eps oauth.Endpoints) func() {
	prev := oauthEndpoints
	oauthEndpoints = eps
	return func() { oauthEndpoints = prev }
}

// freeLoopbackPort returns a currently-free loopback TCP port.
func freeLoopbackPort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve free port: %v", err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

// oauthTestServer serves the token and accessible-resources endpoints. The
// resources JSON is caller-supplied so a test can control cloud-id matching.
func oauthTestServer(t *testing.T, resourcesJSON string) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"access_token":"acc-secret","refresh_token":"ref-secret","expires_in":3600}`)
	})
	mux.HandleFunc("/resources", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, resourcesJSON)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// codeStub returns a browser stub that, instead of opening a browser, hits the
// loopback callback with the given code and the authorize URL's state (or a
// forced state when override is non-empty, to exercise state mismatch).
func codeStub(t *testing.T, code, stateOverride string) func(string) error {
	t.Helper()
	return func(rawURL string) error {
		u, err := url.Parse(rawURL)
		if err != nil {
			return err
		}
		q := u.Query()
		state := q.Get("state")
		if stateOverride != "" {
			state = stateOverride
		}
		redir := q.Get("redirect_uri")
		resp, err := http.Get(redir + "?code=" + url.QueryEscape(code) + "&state=" + url.QueryEscape(state))
		if err != nil {
			return err
		}
		_ = resp.Body.Close()
		return nil
	}
}

func TestAuthLoginOAuth3LOFullFlow(t *testing.T) {
	keyring.MockInit()
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	srv := oauthTestServer(t, `[{"id":"cloud-xyz","url":"https://example.atlassian.net","name":"Example"}]`)
	defer swapOAuthEndpoints(oauth.Endpoints{Authorize: srv.URL + "/authorize", Token: srv.URL + "/token", Resources: srv.URL + "/resources"})()
	defer swapBrowseOpener(codeStub(t, "auth-code-1", ""))()

	port := freeLoopbackPort(t)
	out, err := execRoot(t, jiraInfo(), "auth", "login", "--site", "work",
		"--url", "https://example.atlassian.net",
		"--token-style", "oauth-3lo",
		"--client-id", "client-abc",
		"--client-secret", "client-secret-xyz",
		"--scopes", "read:jira-work,write:jira-work",
		"--callback-port", strconv.Itoa(port),
		"--json=*",
	)
	if err != nil {
		t.Fatalf("oauth login: %v\n%s", err, out)
	}

	// Profile is recorded with the resolved cloud id, client id, and full scopes.
	cfg, err := config.Load(configPath(t, dir))
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	p, ok := cfg.Sites["work"]
	if !ok {
		t.Fatal("work profile not saved")
	}
	if p.TokenStyle != "oauth-3lo" || p.AuthType != "oauth-bearer" {
		t.Errorf("style/auth_type = %q/%q", p.TokenStyle, p.AuthType)
	}
	if p.CloudID != "cloud-xyz" {
		t.Errorf("cloud_id = %q, want cloud-xyz", p.CloudID)
	}
	if p.ClientID != "client-abc" {
		t.Errorf("client_id = %q", p.ClientID)
	}
	if !contains(p.Scopes, "offline_access") || !contains(p.Scopes, "read:jira-work") {
		t.Errorf("scopes = %v, want offline_access auto-added", p.Scopes)
	}
	if p.APIBaseURL != "https://api.atlassian.com/ex/jira/cloud-xyz/rest/api/3" {
		t.Errorf("api_base_url = %q", p.APIBaseURL)
	}

	// config.json must never hold the client secret or tokens.
	rawCfg, err := os.ReadFile(configPath(t, dir))
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	for _, secret := range []string{"client-secret-xyz", "acc-secret", "ref-secret"} {
		if strings.Contains(string(rawCfg), secret) {
			t.Fatalf("config.json leaked a secret (%s)", secret)
		}
		if strings.Contains(out, secret) {
			t.Fatalf("command output leaked a secret (%s):\n%s", secret, out)
		}
	}

	// The bundle is stored with secret + tokens in the keychain.
	credPath, _ := config.CredentialsPath()
	store, _ := secrets.ForRef(secrets.BackendKeyring, credPath)
	value, err := store.Get("work")
	if err != nil {
		t.Fatalf("stored bundle: %v", err)
	}
	bundle, err := oauth.ParseBundle(value)
	if err != nil {
		t.Fatalf("parse bundle: %v", err)
	}
	if bundle.ClientSecret != "client-secret-xyz" || bundle.AccessToken != "acc-secret" || bundle.RefreshToken != "ref-secret" {
		t.Errorf("stored bundle missing fields: %+v", bundle)
	}
}

func TestAuthLoginOAuth3LOClientSecretStdin(t *testing.T) {
	keyring.MockInit()
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	srv := oauthTestServer(t, `[{"id":"cloud-xyz","url":"https://example.atlassian.net"}]`)
	defer swapOAuthEndpoints(oauth.Endpoints{Token: srv.URL + "/token", Resources: srv.URL + "/resources"})()
	defer swapBrowseOpener(codeStub(t, "code", ""))()

	port := freeLoopbackPort(t)
	_, err := execRootIn(t, jiraInfo(), "stdin-secret\n", "auth", "login", "--site", "work",
		"--url", "https://example.atlassian.net",
		"--token-style", "oauth-3lo",
		"--client-id", "client-abc",
		"--client-secret-stdin",
		"--scopes", "read:jira-work",
		"--callback-port", strconv.Itoa(port),
	)
	if err != nil {
		t.Fatalf("oauth login (stdin secret): %v", err)
	}
	credPath, _ := config.CredentialsPath()
	store, _ := secrets.ForRef(secrets.BackendKeyring, credPath)
	value, err := store.Get("work")
	if err != nil {
		t.Fatalf("stored bundle: %v", err)
	}
	bundle, _ := oauth.ParseBundle(value)
	if bundle.ClientSecret != "stdin-secret" {
		t.Errorf("client secret from stdin not stored, got %q", bundle.ClientSecret)
	}
}

func TestAuthLoginOAuth3LORejectsNoPrompt(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	_, err := execRoot(t, jiraInfo(), "auth", "login", "--site", "work",
		"--url", "https://example.atlassian.net",
		"--token-style", "oauth-3lo",
		"--client-id", "client-abc",
		"--client-secret", "secret",
		"--scopes", "read:jira-work",
		"--no-prompt",
	)
	assertStructuredError(t, err)
}

func TestAuthLoginOAuth3LORequiresClientCredentials(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	// Missing --client-id.
	_, err := execRoot(t, jiraInfo(), "auth", "login", "--site", "work",
		"--url", "https://example.atlassian.net", "--token-style", "oauth-3lo",
		"--client-secret", "secret", "--scopes", "read:jira-work")
	assertStructuredError(t, err)

	// Missing --scopes.
	_, err = execRoot(t, jiraInfo(), "auth", "login", "--site", "work",
		"--url", "https://example.atlassian.net", "--token-style", "oauth-3lo",
		"--client-id", "c", "--client-secret", "secret")
	assertStructuredError(t, err)
}

func TestAuthLoginOAuth3LOStateMismatchFails(t *testing.T) {
	keyring.MockInit()
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	srv := oauthTestServer(t, `[{"id":"cloud-xyz","url":"https://example.atlassian.net"}]`)
	defer swapOAuthEndpoints(oauth.Endpoints{Token: srv.URL + "/token", Resources: srv.URL + "/resources"})()
	defer swapBrowseOpener(codeStub(t, "code", "wrong-state"))()

	port := freeLoopbackPort(t)
	_, err := execRoot(t, jiraInfo(), "auth", "login", "--site", "work",
		"--url", "https://example.atlassian.net",
		"--token-style", "oauth-3lo",
		"--client-id", "c", "--client-secret", "s",
		"--scopes", "read:jira-work",
		"--callback-port", strconv.Itoa(port),
	)
	if err == nil {
		t.Fatal("state mismatch did not fail the login")
	}
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Code != "oauth_state_mismatch" {
		t.Fatalf("error = %v, want oauth_state_mismatch", err)
	}
}

func TestAuthLoginOAuth3LOAmbiguousSiteNeedsCloudID(t *testing.T) {
	keyring.MockInit()
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	// No accessible site matches the configured URL host.
	srv := oauthTestServer(t, `[{"id":"a","url":"https://other.atlassian.net"}]`)
	defer swapOAuthEndpoints(oauth.Endpoints{Token: srv.URL + "/token", Resources: srv.URL + "/resources"})()
	defer swapBrowseOpener(codeStub(t, "code", ""))()

	port := freeLoopbackPort(t)
	_, err := execRoot(t, jiraInfo(), "auth", "login", "--site", "work",
		"--url", "https://example.atlassian.net",
		"--token-style", "oauth-3lo",
		"--client-id", "c", "--client-secret", "s",
		"--scopes", "read:jira-work",
		"--callback-port", strconv.Itoa(port),
	)
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Code != "oauth_no_matching_site" {
		t.Fatalf("error = %v, want oauth_no_matching_site", err)
	}

	// With an explicit --cloud-id override that matches a granted site, it succeeds.
	defer swapBrowseOpener(codeStub(t, "code", ""))()
	port2 := freeLoopbackPort(t)
	if _, err := execRoot(t, jiraInfo(), "auth", "login", "--site", "work",
		"--url", "https://example.atlassian.net",
		"--token-style", "oauth-3lo",
		"--client-id", "c", "--client-secret", "s",
		"--scopes", "read:jira-work",
		"--cloud-id", "a",
		"--callback-port", strconv.Itoa(port2),
	); err != nil {
		t.Fatalf("override login: %v", err)
	}
	cfg, _ := config.Load(configPath(t, dir))
	if cfg.Sites["work"].CloudID != "a" {
		t.Errorf("cloud_id override not applied: %q", cfg.Sites["work"].CloudID)
	}
}

func contains(ss []string, want string) bool {
	for _, s := range ss {
		if s == want {
			return true
		}
	}
	return false
}

func assertStructuredError(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	var ae *apperr.Error
	if !errors.As(err, &ae) {
		t.Fatalf("error type = %T, want *apperr.Error", err)
	}
}
