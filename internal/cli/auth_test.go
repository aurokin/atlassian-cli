package cli

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zalando/go-keyring"

	"github.com/aurokin/atlassian-cli/internal/apperr"
	"github.com/aurokin/atlassian-cli/internal/appinfo"
	"github.com/aurokin/atlassian-cli/internal/config"
	"github.com/aurokin/atlassian-cli/internal/secrets"
)

// TestMain installs the in-memory keyring mock for every test in package cli
// so a test that stores a credential never touches the real OS keychain.
func TestMain(m *testing.M) {
	keyring.MockInit()
	os.Exit(m.Run())
}

func jiraInfo() appinfo.Info {
	return appinfo.New("atl-jira", appinfo.ProductJira, "test", "", "")
}

func confInfo() appinfo.Info {
	return appinfo.New("atl-conf", appinfo.ProductConfluence, "test", "", "")
}

// execRoot builds a fresh root for info and runs it with args, returning the
// combined output and the execution error.
func execRoot(t *testing.T, info appinfo.Info, args ...string) (string, error) {
	t.Helper()
	return execRootIn(t, info, "", args...)
}

// execRootIn is execRoot with stdin wired to in, for commands that read a
// value (such as auth login --token-stdin) from standard input.
func execRootIn(t *testing.T, info appinfo.Info, in string, args ...string) (string, error) {
	t.Helper()
	root, _ := NewRoot(info, "short")
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetIn(strings.NewReader(in))
	root.SetArgs(args)
	err := root.Execute()
	return buf.String(), err
}

func configPath(t *testing.T, dir string) string {
	t.Helper()
	return filepath.Join(dir, "atlassian-cli", "config.json")
}

func credentialsFilePath(t *testing.T, dir string) string {
	t.Helper()
	return filepath.Join(dir, "atlassian-cli", "credentials.json")
}

func TestAuthLoginCreatesProductSpecificProfile(t *testing.T) {
	cases := []struct {
		name        string
		info        appinfo.Info
		wantProduct string
	}{
		{"jira", jiraInfo(), "jira"},
		{"confluence", confInfo(), "confluence"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			t.Setenv("XDG_CONFIG_HOME", dir)

			if _, err := execRoot(t, tc.info, "auth", "login", "--site", "work",
				"--url", "https://example.atlassian.net", "--username", "user@example.com",
				"--token-style", "cloud-classic", "--token-env", "ATL_TEST_TOKEN"); err != nil {
				t.Fatalf("login: %v", err)
			}

			cfg, err := config.Load(configPath(t, dir))
			if err != nil {
				t.Fatalf("load config: %v", err)
			}
			p, ok := cfg.Sites["work"]
			if !ok {
				t.Fatal("profile 'work' was not saved")
			}
			if p.Product != tc.wantProduct {
				t.Errorf("Product = %q, want %q", p.Product, tc.wantProduct)
			}
			if p.TokenStyle != "cloud-classic" {
				t.Errorf("TokenStyle = %q, want cloud-classic", p.TokenStyle)
			}
			if p.TokenRef != "env:ATL_TEST_TOKEN" {
				t.Errorf("TokenRef = %q, want env:ATL_TEST_TOKEN", p.TokenRef)
			}
		})
	}
}

func TestAuthLoginStoresNoRawToken(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	const secret = "raw-secret-token"
	t.Setenv("ATL_TEST_TOKEN", secret)

	if _, err := execRoot(t, jiraInfo(), "auth", "login", "--site", "work",
		"--url", "https://example.atlassian.net", "--username", "user@example.com",
		"--token-style", "cloud-classic", "--token-env", "ATL_TEST_TOKEN"); err != nil {
		t.Fatalf("login: %v", err)
	}
	raw, err := os.ReadFile(configPath(t, dir))
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if strings.Contains(string(raw), secret) {
		t.Fatal("config file contains the raw token value")
	}
}

func TestAuthStatusDoesNotExposeTokenValue(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	const secret = "super-secret-token-value"
	t.Setenv("ATL_TEST_TOKEN", secret)

	if _, err := execRoot(t, jiraInfo(), "auth", "login", "--site", "work",
		"--url", "https://example.atlassian.net", "--username", "user@example.com",
		"--token-style", "cloud-classic", "--token-env", "ATL_TEST_TOKEN"); err != nil {
		t.Fatalf("login: %v", err)
	}
	out, err := execRoot(t, jiraInfo(), "auth", "status", "--site", "work", "--json=*")
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if strings.Contains(out, secret) {
		t.Fatalf("status output leaked the token value:\n%s", out)
	}
	if !strings.Contains(out, "work") {
		t.Error("status output missing site name")
	}
	if !strings.Contains(out, "ATL_TEST_TOKEN") {
		t.Error("status output missing token reference")
	}
	if !strings.Contains(out, "token available") {
		t.Error("status output missing token availability")
	}
}

func TestAuthLogoutRemovesOnlyRequestedSite(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	for _, site := range []string{"work", "personal"} {
		if _, err := execRoot(t, jiraInfo(), "auth", "login", "--site", site,
			"--url", "https://example.atlassian.net", "--username", "user@example.com",
			"--token-style", "cloud-classic"); err != nil {
			t.Fatalf("login %s: %v", site, err)
		}
	}
	if _, err := execRoot(t, jiraInfo(), "auth", "logout", "--site", "work"); err != nil {
		t.Fatalf("logout: %v", err)
	}

	cfg, err := config.Load(configPath(t, dir))
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if _, ok := cfg.Sites["work"]; ok {
		t.Error("logged-out site 'work' was not removed")
	}
	if _, ok := cfg.Sites["personal"]; !ok {
		t.Error("site 'personal' was wrongly removed")
	}
}

func TestAuthLoginMissingFieldsReturnStructuredErrors(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{"missing token style", []string{"auth", "login", "--site", "work", "--url", "https://example.atlassian.net", "--username", "u@e.com"}},
		{"missing url", []string{"auth", "login", "--site", "work", "--token-style", "cloud-classic", "--username", "u@e.com"}},
		{"missing cloud id for scoped", []string{"auth", "login", "--site", "work", "--url", "https://example.atlassian.net", "--username", "u@e.com", "--token-style", "cloud-scoped"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			t.Setenv("XDG_CONFIG_HOME", dir)
			_, err := execRoot(t, jiraInfo(), tc.args...)
			if err == nil {
				t.Fatal("expected an error")
			}
			var ae *apperr.Error
			if !errors.As(err, &ae) {
				t.Fatalf("error type = %T, want *apperr.Error", err)
			}
		})
	}
}

func TestAuthStatusUnknownSiteReturnsStructuredError(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	_, err := execRoot(t, jiraInfo(), "auth", "status", "--site", "absent")
	if err == nil {
		t.Fatal("expected an error for an unconfigured site")
	}
	var ae *apperr.Error
	if !errors.As(err, &ae) {
		t.Fatalf("error type = %T, want *apperr.Error", err)
	}
}

func TestAuthLoginRejectsUnsafeSiteURLs(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{"embedded credentials", []string{"auth", "login", "--site", "work", "--url", "https://user:pw@example.atlassian.net", "--username", "u@e.com", "--token-style", "cloud-classic"}},
		{"non-http scheme", []string{"auth", "login", "--site", "work", "--url", "ftp://example.atlassian.net", "--username", "u@e.com", "--token-style", "cloud-classic"}},
		{"http for a cloud style", []string{"auth", "login", "--site", "work", "--url", "http://example.atlassian.net", "--username", "u@e.com", "--token-style", "cloud-classic"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			t.Setenv("XDG_CONFIG_HOME", dir)
			_, err := execRoot(t, jiraInfo(), tc.args...)
			if err == nil {
				t.Fatal("expected an error for an unsafe --url")
			}
			var ae *apperr.Error
			if !errors.As(err, &ae) {
				t.Fatalf("error type = %T, want *apperr.Error", err)
			}
		})
	}
}

func TestAuthLoginAllowsHTTPForDataCenter(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	if _, err := execRoot(t, jiraInfo(), "auth", "login", "--site", "dc",
		"--url", "http://jira.internal.example.com", "--token-style", "data-center-pat",
		"--token-env", "ATL_TEST_TOKEN"); err != nil {
		t.Fatalf("data-center login over http should be allowed: %v", err)
	}
}

func TestAuthLoginStoresTokenInKeyring(t *testing.T) {
	keyring.MockInit()
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	const secret = "stored-secret-token"

	if _, err := execRoot(t, jiraInfo(), "auth", "login", "--site", "work",
		"--url", "https://example.atlassian.net", "--username", "user@example.com",
		"--token-style", "cloud-classic", "--token", secret); err != nil {
		t.Fatalf("login: %v", err)
	}

	cfg, err := config.Load(configPath(t, dir))
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if ref := cfg.Sites["work"].TokenRef; ref != secrets.BackendKeyring {
		t.Errorf("TokenRef = %q, want %q", ref, secrets.BackendKeyring)
	}
	// config.json never holds the raw token, whatever the backend.
	raw, _ := os.ReadFile(configPath(t, dir))
	if strings.Contains(string(raw), secret) {
		t.Fatal("config file contains the raw token value")
	}
	store, err := secrets.ForRef(secrets.BackendKeyring, credentialsFilePath(t, dir))
	if err != nil {
		t.Fatalf("ForRef: %v", err)
	}
	if got, err := store.Get("work"); err != nil || got != secret {
		t.Fatalf("keyring Get = %q, %v; want %q", got, err, secret)
	}
}

func TestAuthLoginReadsTokenFromStdin(t *testing.T) {
	keyring.MockInit()
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	const secret = "stdin-secret-token"

	if _, err := execRootIn(t, jiraInfo(), secret+"\n", "auth", "login", "--site", "work",
		"--url", "https://example.atlassian.net", "--username", "user@example.com",
		"--token-style", "cloud-classic", "--token-stdin"); err != nil {
		t.Fatalf("login: %v", err)
	}
	store, _ := secrets.ForRef(secrets.BackendKeyring, credentialsFilePath(t, dir))
	if got, err := store.Get("work"); err != nil || got != secret {
		t.Fatalf("keyring Get = %q, %v; want %q (stdin should be trimmed)", got, err, secret)
	}
}

func TestAuthLoginRejectsMultipleTokenSources(t *testing.T) {
	keyring.MockInit()
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	_, err := execRoot(t, jiraInfo(), "auth", "login", "--site", "work",
		"--url", "https://example.atlassian.net", "--username", "user@example.com",
		"--token-style", "cloud-classic", "--token-env", "ATL_TEST_TOKEN", "--token", "x")
	if err == nil {
		t.Fatal("expected an error for multiple token sources")
	}
	var ae *apperr.Error
	if !errors.As(err, &ae) {
		t.Fatalf("error type = %T, want *apperr.Error", err)
	}
}

func TestAuthLoginRejectsEmptyStdinToken(t *testing.T) {
	keyring.MockInit()
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	_, err := execRootIn(t, jiraInfo(), "   \n", "auth", "login", "--site", "work",
		"--url", "https://example.atlassian.net", "--username", "user@example.com",
		"--token-style", "cloud-classic", "--token-stdin")
	if err == nil {
		t.Fatal("expected an error for an empty stdin token")
	}
	var ae *apperr.Error
	if !errors.As(err, &ae) {
		t.Fatalf("error type = %T, want *apperr.Error", err)
	}
}

func TestAuthLoginFallsBackToFileWhenKeyringUnavailable(t *testing.T) {
	keyring.MockInitWithError(errors.New("no keychain available"))
	t.Cleanup(keyring.MockInit)
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	const secret = "fallback-secret-token"

	out, err := execRoot(t, jiraInfo(), "auth", "login", "--site", "work",
		"--url", "https://example.atlassian.net", "--username", "user@example.com",
		"--token-style", "cloud-classic", "--token", secret)
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if !strings.Contains(out, "Warning") || !strings.Contains(out, "keychain") {
		t.Errorf("expected a keychain-fallback warning, got:\n%s", out)
	}

	cfg, _ := config.Load(configPath(t, dir))
	if ref := cfg.Sites["work"].TokenRef; ref != secrets.BackendFile {
		t.Errorf("TokenRef = %q, want %q", ref, secrets.BackendFile)
	}
	credPath := credentialsFilePath(t, dir)
	st, err := os.Stat(credPath)
	if err != nil {
		t.Fatalf("stat credentials file: %v", err)
	}
	if perm := st.Mode().Perm(); perm != 0o600 {
		t.Errorf("credentials file mode = %o, want 600", perm)
	}
	raw, _ := os.ReadFile(configPath(t, dir))
	if strings.Contains(string(raw), secret) {
		t.Fatal("config file contains the raw token value")
	}
	store, _ := secrets.ForRef(secrets.BackendFile, credPath)
	if got, _ := store.Get("work"); got != secret {
		t.Fatalf("file store Get = %q, want %q", got, secret)
	}
}

func TestAuthStatusReportsStoredKeyringCredential(t *testing.T) {
	keyring.MockInit()
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	const secret = "status-keyring-secret"

	if _, err := execRoot(t, jiraInfo(), "auth", "login", "--site", "work",
		"--url", "https://example.atlassian.net", "--username", "user@example.com",
		"--token-style", "cloud-classic", "--token", secret); err != nil {
		t.Fatalf("login: %v", err)
	}
	out, err := execRoot(t, jiraInfo(), "auth", "status", "--site", "work", "--json=*")
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if strings.Contains(out, secret) {
		t.Fatalf("status output leaked the token value:\n%s", out)
	}
	if !strings.Contains(out, "OS keychain") {
		t.Errorf("status output missing keychain availability:\n%s", out)
	}
}

func TestAuthLoginClearsStoredTokenWhenSwitchingBackends(t *testing.T) {
	keyring.MockInit()
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	// First login stores a token in the keyring.
	if _, err := execRoot(t, jiraInfo(), "auth", "login", "--site", "work",
		"--url", "https://example.atlassian.net", "--username", "user@example.com",
		"--token-style", "cloud-classic", "--token", "first-secret"); err != nil {
		t.Fatalf("first login: %v", err)
	}
	// Re-login with --token-env switches the profile to an env reference.
	if _, err := execRoot(t, jiraInfo(), "auth", "login", "--site", "work",
		"--url", "https://example.atlassian.net", "--username", "user@example.com",
		"--token-style", "cloud-classic", "--token-env", "ATL_TEST_TOKEN"); err != nil {
		t.Fatalf("second login: %v", err)
	}
	// The keyring secret orphaned by the switch must have been removed.
	store, _ := secrets.ForRef(secrets.BackendKeyring, credentialsFilePath(t, dir))
	if _, err := store.Get("work"); err == nil {
		t.Fatal("re-login did not clear the previously stored keyring credential")
	}
}

func TestAuthLogoutClearsStoredToken(t *testing.T) {
	keyring.MockInit()
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	if _, err := execRoot(t, jiraInfo(), "auth", "login", "--site", "work",
		"--url", "https://example.atlassian.net", "--username", "user@example.com",
		"--token-style", "cloud-classic", "--token", "logout-secret"); err != nil {
		t.Fatalf("login: %v", err)
	}
	if _, err := execRoot(t, jiraInfo(), "auth", "logout", "--site", "work"); err != nil {
		t.Fatalf("logout: %v", err)
	}
	store, _ := secrets.ForRef(secrets.BackendKeyring, credentialsFilePath(t, dir))
	if _, err := store.Get("work"); err == nil {
		t.Fatal("logout did not clear the stored keyring credential")
	}
}
