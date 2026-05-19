package cli

import (
	"errors"
	"testing"

	"github.com/zalando/go-keyring"

	"github.com/aurokin/atlassian-cli/internal/apperr"
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
	_, err := SiteClient(jiraInfo(), &GlobalFlags{})
	if err == nil {
		t.Fatal("SiteClient with no --site returned no error")
	}
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Code != apperr.CodeInvalidInput {
		t.Fatalf("error = %v, want an invalid_input *apperr.Error", err)
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
