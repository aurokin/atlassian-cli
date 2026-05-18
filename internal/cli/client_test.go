package cli

import (
	"errors"
	"testing"

	"github.com/aurokin/atlassian-cli/internal/apperr"
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
