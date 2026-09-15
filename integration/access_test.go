//go:build integration

package integration

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/aurokin/atlassian-cli/internal/config"
)

// TestRestrictedAccess checks credential-scope restrictions for the same user,
// not second-user permissions. Run explicitly with two stored profiles:
// ATL_IT_ACCESS_SITE is read-only and ATL_IT_ACCESS_OWNER_SITE owns fixtures.
// The ordinary product fixture/expected-account environment variables apply.
func TestRestrictedAccess(t *testing.T) {
	requireIntegration(t)
	restrictedSite := strings.TrimSpace(os.Getenv("ATL_IT_ACCESS_SITE"))
	ownerSite := strings.TrimSpace(os.Getenv("ATL_IT_ACCESS_OWNER_SITE"))
	if !useStoredProfiles() || restrictedSite == "" || ownerSite == "" || restrictedSite == ownerSite {
		t.Fatal("restricted access requires stored mode and distinct ATL_IT_ACCESS_SITE / ATL_IT_ACCESS_OWNER_SITE profiles")
	}
	path, err := config.DefaultPath()
	if err != nil {
		t.Fatal("locate profile configuration")
	}
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatal("load profile configuration")
	}
	restrictedProfile, restrictedOK := cfg.Sites[restrictedSite]
	ownerProfile, ownerOK := cfg.Sites[ownerSite]
	if !restrictedOK || !ownerOK || restrictedProfile.Product != ownerProfile.Product {
		t.Fatal("restricted and owner profiles must exist and target the same product")
	}
	if strings.TrimRight(restrictedProfile.BaseURL, "/") != strings.TrimRight(ownerProfile.BaseURL, "/") ||
		(restrictedProfile.CloudID != "" && ownerProfile.CloudID != "" && restrictedProfile.CloudID != ownerProfile.CloudID) {
		t.Fatal("restricted and owner profiles target different sites")
	}
	var product product
	switch restrictedProfile.Product {
	case "jira":
		product = jiraProduct
	case "confluence":
		product = confProduct
	case "bitbucket":
		product = bbProduct
	default:
		t.Fatal("unsupported restricted-access product")
	}
	style := "cloud-scoped"
	if product.envPrefix == "BB" {
		style = "cloud-classic"
	}
	if restrictedProfile.TokenStyle != style {
		t.Fatalf("restricted profile must use %s token transport", style)
	}
	binary := buildBinary(t, product.binary)
	owner := &session{t: t, binaryPath: binary, site: ownerSite}
	restricted := &session{t: t, binaryPath: binary, site: restrictedSite}
	owner.preflight(product)
	restricted.preflight(product)
	if owner.target["account_id"] != restricted.target["account_id"] {
		t.Fatal("scope test requires the same authenticated account for owner and restricted credentials")
	}
	t.Logf("restricted credential scope test: product=%s owner=%s restricted=%s same_account=true; not a second-user permission test", product.envPrefix, ownerSite, restrictedSite)
	switch product.envPrefix {
	case "JIRA":
		restrictedJiraEdit(t, owner, restricted)
	case "CONF":
		restrictedConfluenceEdit(t, owner, restricted)
	case "BB":
		restrictedBitbucketEdit(t, owner, restricted)
	}
}

func expectScopeDenied(t *testing.T, product string, result cmdResult) {
	t.Helper()
	wantExit, wantStatus, wantError := 5, 403, "forbidden"
	switch product {
	case "jira", "confluence":
		// Observed scoped-gateway contract: a valid read-only credential is
		// rejected with 401 and this specific scope message, not generic 403.
		wantExit, wantStatus, wantError = 4, 401, "unauthorized"
	case "bitbucket":
		// Bitbucket's Basic-auth endpoint reports missing write scope as 403.
	default:
		t.Fatal("unsupported product for scope-denial assertion")
	}
	var exit *exec.ExitError
	var envelope struct {
		Error   string `json:"error"`
		Status  int    `json:"status"`
		Message string `json:"message"`
		Product string `json:"product"`
	}
	if !errors.As(result.err, &exit) || exit.ExitCode() != wantExit {
		t.Errorf("restricted mutation must exit %d; got %v", wantExit, result.err)
	}
	if err := json.Unmarshal([]byte(result.stderr), &envelope); err != nil || envelope.Error != wantError || envelope.Status != wantStatus || envelope.Product != product {
		t.Errorf("restricted mutation must emit %s/%d for %s on stderr: %s", wantError, wantStatus, product, result.stderr)
	}
	if wantStatus == 401 && envelope.Message != "Unauthorized; scope does not match" {
		t.Errorf("401 must specifically identify scope mismatch, not an invalid/expired credential: %s", result.stderr)
	}
	t.Logf("scope denial: product=%s status=%d category=%s", product, envelope.Status, envelope.Error)
	// Do not abort here: owner read-back must run even if the supposedly
	// restricted token unexpectedly succeeded. Owned fixtures still clean up.
}

func restrictedJiraEdit(t *testing.T, owner, restricted *session) {
	t.Helper()
	initial := "atl-cli restricted-scope " + time.Now().UTC().Format("20060102T150405.000000000")
	var created struct {
		Key string `json:"key"`
	}
	owner.mustJSON(&created, "issue", "create", "--project", jiraProject(t), "--type", jiraIssueType(), "--summary", initial)
	if created.Key == "" {
		t.Fatal("scope fixture creation returned no issue key")
	}
	cleanupJiraIssue(owner, created.Key)
	baseline := initial + " owner-control"
	owner.mustRun("issue", "edit", created.Key, "--summary", baseline, "--json")
	type issue struct {
		Key    string `json:"key"`
		Fields struct {
			Summary string `json:"summary"`
		} `json:"fields"`
	}
	var readable issue
	restricted.mustJSON(&readable, "issue", "view", created.Key, "--fields", "summary")
	if readable.Key != created.Key || readable.Fields.Summary != baseline {
		t.Fatal("restricted credential cannot read the exact owned issue")
	}
	expectScopeDenied(t, "jira", restricted.run("issue", "edit", created.Key, "--summary", initial+" forbidden-change", "--json"))
	var after issue
	owner.mustJSON(&after, "issue", "view", created.Key, "--fields", "summary")
	if after.Key != created.Key || after.Fields.Summary != baseline {
		t.Fatal("restricted mutation changed the owned issue")
	}
}

func restrictedConfluenceEdit(t *testing.T, owner, restricted *session) {
	t.Helper()
	created := confCreate(t, owner, "page", "<p>restricted scope fixture body</p>", "storage")
	baseline := created.Title + " owner-control"
	owner.mustRun("page", "edit", created.ID, "--title", baseline, "--json")
	var before confContent
	restricted.mustJSON(&before, "page", "view", created.ID)
	if before.ID != created.ID || before.Title != baseline || !strings.Contains(before.Body.Storage.Value, "restricted scope fixture body") {
		t.Fatal("restricted credential cannot read the exact owned page")
	}
	expectScopeDenied(t, "confluence", restricted.run("page", "edit", created.ID, "--title", created.Title+" forbidden-change", "--json"))
	var after confContent
	owner.mustJSON(&after, "page", "view", created.ID)
	if after.ID != created.ID || after.Title != baseline || after.Version.Number != before.Version.Number || after.Body.Storage.Value != before.Body.Storage.Value {
		t.Fatal("restricted mutation changed the owned page")
	}
}

func restrictedBitbucketEdit(t *testing.T, owner, restricted *session) {
	t.Helper()
	target := ownedBBRepo(t, owner)
	// No repo edit verb is shipped; the raw API is the supported write escape
	// hatch. A positive owner update proves this same endpoint/payload is valid.
	path := "/repositories/" + target
	owner.mustRun("api", path, "--method", "PUT", "--data", `{"description":"scope-owner-control"}`, "--json")
	type repository struct {
		FullName    string `json:"full_name"`
		Description string `json:"description"`
		Private     bool   `json:"is_private"`
	}
	var before repository
	restricted.mustJSON(&before, "repo", "view", target)
	if !strings.EqualFold(before.FullName, target) || before.Description != "scope-owner-control" || !before.Private {
		t.Fatal("restricted credential cannot read the exact owned repository")
	}
	expectScopeDenied(t, "bitbucket", restricted.run("api", path, "--method", "PUT", "--data", `{"description":"scope-forbidden-change"}`, "--json"))
	var after repository
	owner.mustJSON(&after, "repo", "view", target)
	if after.FullName != before.FullName || after.Description != before.Description || !after.Private {
		t.Fatal("restricted mutation changed the owned repository")
	}
}
