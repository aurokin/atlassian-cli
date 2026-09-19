package cli

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/zalando/go-keyring"

	"github.com/aurokin/atlassian-cli/internal/apperr"
	"github.com/aurokin/atlassian-cli/internal/httpclient"
	"github.com/aurokin/atlassian-cli/internal/secrets"
)

// loginTestSite records a data-center site profile named "work" pointing at a
// clean URL, arming tokenEnv as its token reference.
func loginTestSite(t *testing.T, tokenEnv string) {
	t.Helper()
	if _, err := execRoot(t, jiraInfo(), "auth", "login", "--site", "work",
		"--url", "https://example.atlassian.net", "--token-style", "data-center-pat",
		"--token-env", tokenEnv); err != nil {
		t.Fatalf("login: %v", err)
	}
}

func TestSiteClientRequiresSiteFlag(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv(siteEnvVar, "")
	_, err := SiteClient(jiraInfo(), &GlobalFlags{})
	if err == nil {
		t.Fatal("SiteClient with no --site returned no error")
	}
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Code != apperr.CodeInvalidInput {
		t.Fatalf("error = %v, want an invalid_input *apperr.Error", err)
	}
}

func TestResolveSiteNamePrecedence(t *testing.T) {
	cases := []struct {
		name string
		flag string
		env  string
		def  string
		want string
	}{
		{"flag wins over env and default", "flag", "env", "def", "flag"},
		{"env wins over default", "", "env", "def", "env"},
		{"default is last resort", "", "", "def", "def"},
		{"env whitespace is ignored", "", "   ", "def", "def"},
		{"none set", "", "", "", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Setenv(siteEnvVar, c.env)
			if got := resolveSiteName(c.flag, c.def); got != c.want {
				t.Fatalf("resolveSiteName(%q, %q) with %s=%q = %q, want %q",
					c.flag, c.def, siteEnvVar, c.env, got, c.want)
			}
		})
	}
}

func TestSiteClientUsesATLSiteEnv(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("ATL_API_TOKEN", "test-token")
	loginTestSite(t, "ATL_API_TOKEN")
	t.Setenv(siteEnvVar, "work")

	// No --site flag: the site is taken from ATL_SITE.
	client, err := SiteClient(jiraInfo(), &GlobalFlags{})
	if err != nil {
		t.Fatalf("SiteClient: %v", err)
	}
	if client == nil {
		t.Fatal("SiteClient returned a nil client")
	}
}

func TestSiteClientUsesDefaultSite(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("ATL_API_TOKEN", "test-token")
	t.Setenv(siteEnvVar, "")
	loginTestSite(t, "ATL_API_TOKEN")
	if _, err := execRoot(t, jiraInfo(), "auth", "default", "work"); err != nil {
		t.Fatalf("auth default: %v", err)
	}

	// No --site flag and no ATL_SITE: the site is taken from default_site.
	client, err := SiteClient(jiraInfo(), &GlobalFlags{})
	if err != nil {
		t.Fatalf("SiteClient: %v", err)
	}
	if client == nil {
		t.Fatal("SiteClient returned a nil client")
	}
}

func TestSiteClientRejectsUnconfiguredSite(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	_, err := SiteClient(jiraInfo(), &GlobalFlags{Site: "absent"})
	if err == nil {
		t.Fatal("SiteClient with an unconfigured site returned no error")
	}
	var ae *apperr.Error
	if !errors.As(err, &ae) {
		t.Fatalf("error type = %T, want *apperr.Error", err)
	}
}

func TestSiteClientBuildsClientForConfiguredSite(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("ATL_API_TOKEN", "test-token")
	loginTestSite(t, "ATL_API_TOKEN")

	client, err := SiteClient(jiraInfo(), &GlobalFlags{Site: "work"})
	if err != nil {
		t.Fatalf("SiteClient: %v", err)
	}
	if client == nil {
		t.Fatal("SiteClient returned a nil client")
	}
}

func TestSiteClientReportsMissingToken(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	// Reference an env var that is deliberately left unset.
	loginTestSite(t, "ATL_TOKEN_NOT_SET")

	_, err := SiteClient(jiraInfo(), &GlobalFlags{Site: "work"})
	if err == nil {
		t.Fatal("SiteClient returned no error when the token env var is unset")
	}
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Code != "token_unavailable" {
		t.Fatalf("error = %v, want a token_unavailable *apperr.Error", err)
	}
}

func TestSiteClientResolvesKeyringToken(t *testing.T) {
	keyring.MockInit()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if _, err := execRoot(t, jiraInfo(), "auth", "login", "--site", "work",
		"--url", "https://example.atlassian.net", "--token-style", "data-center-pat",
		"--token", "keyring-stored-token"); err != nil {
		t.Fatalf("login: %v", err)
	}

	client, err := SiteClient(jiraInfo(), &GlobalFlags{Site: "work"})
	if err != nil {
		t.Fatalf("SiteClient: %v", err)
	}
	if client == nil {
		t.Fatal("SiteClient returned a nil client")
	}
}

func TestSiteClientReportsMissingStoredToken(t *testing.T) {
	keyring.MockInit()
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	if _, err := execRoot(t, jiraInfo(), "auth", "login", "--site", "work",
		"--url", "https://example.atlassian.net", "--token-style", "data-center-pat",
		"--token", "soon-deleted"); err != nil {
		t.Fatalf("login: %v", err)
	}
	// Drop the stored credential behind the CLI's back.
	store, err := secrets.ForRef(secrets.BackendKeyring, credentialsFilePath(t, dir))
	if err != nil {
		t.Fatalf("ForRef: %v", err)
	}
	if err := store.Delete("work"); err != nil {
		t.Fatalf("delete: %v", err)
	}

	_, err = SiteClient(jiraInfo(), &GlobalFlags{Site: "work"})
	if err == nil {
		t.Fatal("SiteClient returned no error for a missing stored token")
	}
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Code != "token_unavailable" {
		t.Fatalf("error = %v, want a token_unavailable *apperr.Error", err)
	}
}

func TestResolveTimeoutPrecedence(t *testing.T) {
	cases := []struct {
		name string
		flag string
		env  string
		want time.Duration
	}{
		{"flag wins over env", "2m", "1m", 2 * time.Minute},
		{"env wins over default", "", "45s", 45 * time.Second},
		{"default is last resort", "", "", httpclient.DefaultTimeout},
		{"zero disables the timeout", "0", "", 0},
		{"env whitespace is trimmed like ATL_SITE", "", "  45s ", 45 * time.Second},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Setenv(timeoutEnvVar, c.env)
			got, err := resolveTimeout(c.flag)
			if err != nil {
				t.Fatalf("resolveTimeout(%q) with %s=%q: %v", c.flag, timeoutEnvVar, c.env, err)
			}
			if got != c.want {
				t.Fatalf("resolveTimeout(%q) with %s=%q = %v, want %v",
					c.flag, timeoutEnvVar, c.env, got, c.want)
			}
		})
	}
}

func TestResolveTimeoutRejectsInvalidValues(t *testing.T) {
	cases := []struct {
		name       string
		flag       string
		env        string
		wantSource string
	}{
		{"malformed flag", "abc", "", "--timeout"},
		{"negative flag", "-5s", "", "--timeout"},
		{"malformed env", "", "nope", timeoutEnvVar},
		{"whitespace-only flag", "  ", "", "--timeout"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Setenv(timeoutEnvVar, c.env)
			_, err := resolveTimeout(c.flag)
			var ae *apperr.Error
			if !errors.As(err, &ae) || ae.Code != apperr.CodeInvalidInput {
				t.Fatalf("error = %v, want an invalid_input *apperr.Error", err)
			}
			if !strings.Contains(ae.Message, c.wantSource) {
				t.Fatalf("message = %q, want it to name %s", ae.Message, c.wantSource)
			}
		})
	}
}

// TestAPICommandHonorsTimeout proves both timeout sources reach the
// *http.Client SiteClient builds: against a server that never replies, the
// request fails as a timeout well before the 30s default would fire.
func TestAPICommandHonorsTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done() // never reply; let the client timeout fire
	}))
	defer srv.Close()

	for _, tc := range []struct {
		name string
		env  string
		args []string
	}{
		{name: "flag", args: []string{"--timeout", "50ms"}},
		{name: "env", env: "50ms"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("XDG_CONFIG_HOME", t.TempDir())
			t.Setenv(timeoutEnvVar, tc.env)
			loginDataCenter(t, srv.URL)

			args := append([]string{"api", "/rest/api/2/myself", "--site", "work"}, tc.args...)
			start := time.Now()
			_, err := execRoot(t, jiraInfo(), args...)
			var ae *apperr.Error
			if !errors.As(err, &ae) || ae.Code != apperr.CodeTimeout {
				t.Fatalf("error = %v, want a timeout *apperr.Error", err)
			}
			// The configured 50ms must be what fired, not the default.
			if elapsed := time.Since(start); elapsed > time.Second {
				t.Fatalf("timeout fired after %s, want the configured 50ms", elapsed)
			}
		})
	}
}

// TestAPICommandTimeoutZeroDisablesTimeout proves ATL_TIMEOUT=0 builds an
// unbounded client rather than one with an immediately expired deadline.
func TestAPICommandTimeoutZeroDisablesTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(150 * time.Millisecond)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginDataCenter(t, srv.URL)
	t.Setenv(timeoutEnvVar, "0")

	if _, err := execRoot(t, jiraInfo(), "api", "/rest/api/2/myself", "--site", "work"); err != nil {
		t.Fatalf("api with %s=0: %v", timeoutEnvVar, err)
	}
}
