package cli

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aurokin/atlassian-cli/internal/appinfo"
	"github.com/aurokin/atlassian-cli/internal/apperr"
	"github.com/aurokin/atlassian-cli/internal/config"
)

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
	root, _ := NewRoot(info, "short")
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs(args)
	return func() (string, error) {
		err := root.Execute()
		return buf.String(), err
	}()
}

func configPath(t *testing.T, dir string) string {
	t.Helper()
	return filepath.Join(dir, "atlassian-cli", "config.json")
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
