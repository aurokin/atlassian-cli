//go:build integration

package integration

import (
	"errors"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/aurokin/atlassian-cli/internal/config"
)

// TestBitbucketIndependentReviewer selects a second, stored Bitbucket identity
// explicitly. Before any fixture mutation, both accounts must authenticate and
// differ. The reviewer must be a workspace member without inherited access to
// new private repositories (not an admin/default-access group member). The test
// proves denied access before the explicit grant. The owner token needs
// admin:repository:bitbucket and write:permission:bitbucket to grant access only
// to the newly-created private repository. Repository cleanup removes that grant.
// https://developer.atlassian.com/cloud/bitbucket/rest/api-group-repositories/
func TestBitbucketIndependentReviewer(t *testing.T) {
	requireIntegration(t)
	if os.Getenv("ATL_IT_BB_REVIEWER") != "1" {
		t.Skip("set ATL_IT_BB_REVIEWER=1 to select independent live PR approval")
	}
	site := strings.TrimSpace(os.Getenv("ATL_IT_BB_REVIEWER_SITE"))
	if site == "" {
		t.Fatal("selected reviewer acceptance requires ATL_IT_BB_REVIEWER_SITE")
	}
	if !useStoredProfiles() {
		t.Fatal("independent reviewer acceptance requires stored-profile mode")
	}
	owner := bbSession(t)
	if site == owner.site {
		t.Fatal("reviewer profile must differ from the owner profile")
	}
	path, err := config.DefaultPath()
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	profile, ok := cfg.Sites[site]
	if !ok || profile.Product != "bitbucket" {
		t.Fatal("reviewer profile is absent or is not Bitbucket")
	}
	if strings.TrimRight(profile.BaseURL, "/") != strings.TrimRight(owner.target["base_url"], "/") {
		t.Fatal("reviewer and owner must target the same Bitbucket API")
	}
	reviewer := &session{t: t, binaryPath: owner.binaryPath, site: site}
	type identity struct {
		UUID string `json:"uuid"`
	}
	var authorID, reviewerID identity
	owner.mustJSON(&authorID, "status")
	reviewer.mustJSON(&reviewerID, "status")
	if authorID.UUID == "" || reviewerID.UUID == "" {
		t.Fatal("both owner and reviewer must return authenticated UUIDs")
	}
	if authorID.UUID == reviewerID.UUID {
		t.Fatal("reviewer and owner profiles resolve to the same account; independent approval requires distinct identities")
	}
	// Do not reuse owner.preflight for the reviewer: the owner's optional expected
	// account setting must not force the second identity to equal the author.
	t.Logf("independent reviewer preflight owner=%s reviewer=%s reviewer_site=%s", authorID.UUID, reviewerID.UUID, site)
	target := ownedBBRepo(t, owner)
	var repository struct {
		FullName string `json:"full_name"`
		Private  bool   `json:"is_private"`
	}
	owner.mustJSON(&repository, "repo", "view", target)
	if !strings.EqualFold(repository.FullName, target) || !repository.Private {
		t.Fatalf("owner could not verify exact private fixture before denial check: %+v", repository)
	}
	denied := reviewer.run("repo", "view", target, "--json")
	if denied.err == nil {
		t.Fatal("reviewer already has inherited access; use a workspace member without initial access to new private repositories")
	}
	var exit *exec.ExitError
	var envelope struct {
		Error  string `json:"error"`
		Status int    `json:"status"`
	}
	if denied.stdout != "" || !errors.As(denied.err, &exit) || jsonUnmarshal(denied.stderr, &envelope) != nil {
		t.Fatalf("restricted reviewer denial did not preserve process/error contract: err=%v stdout=%s stderr=%s", denied.err, denied.stdout, denied.stderr)
	}
	forbidden := exit.ExitCode() == 5 && envelope.Status == 403 && envelope.Error == "forbidden"
	concealed := exit.ExitCode() == 6 && envelope.Status == 404 && envelope.Error == "not_found_or_not_visible"
	if !forbidden && !concealed {
		t.Fatalf("restricted reviewer must receive 403/exit5 or concealed 404/exit6, got exit=%d stderr=%s", exit.ExitCode(), denied.stderr)
	}
	t.Logf("verified independent restricted reviewer denied private repository: status=%d exit=%d", envelope.Status, exit.ExitCode())
	permissionsPath := "/repositories/" + target + "/permissions-config/users/" + url.PathEscape(reviewerID.UUID)
	var permission struct {
		Permission string   `json:"permission"`
		User       identity `json:"user"`
	}
	owner.mustJSON(&permission, "api", permissionsPath, "--method", "PUT", "--data", `{"permission":"write"}`)
	if permission.Permission != "write" || permission.User.UUID != reviewerID.UUID {
		t.Fatalf("owned repository permission grant differs: %+v", permission)
	}
	owner.mustJSON(&permission, "api", permissionsPath)
	if permission.Permission != "write" || permission.User.UUID != reviewerID.UUID {
		t.Fatalf("owned repository permission readback differs: %+v", permission)
	}
	reviewer.mustJSON(&repository, "repo", "view", target)
	if !strings.EqualFold(repository.FullName, target) || !repository.Private {
		t.Fatalf("reviewer cannot positively identify owned private repository: %+v", repository)
	}
	main := seedBBCommit(t, owner, target, "main", "", "README.md", "independent approval fixture\n")
	seedBBCommit(t, owner, target, "review", main, "review.txt", "review this change\n")
	var created bbPR
	owner.mustJSON(&created, "pr", "create", "--repo", target, "--source", "review", "--destination", "main", "--title", "CLI E2E independent reviewer")
	if created.ID == 0 || created.State != "OPEN" {
		t.Fatalf("owned PR was not created open: %+v", created)
	}
	id := strconv.Itoa(created.ID)
	type participant struct {
		User     identity `json:"user"`
		Approved bool     `json:"approved"`
	}
	type pullRequest struct {
		ID           int           `json:"id"`
		State        string        `json:"state"`
		Author       identity      `json:"author"`
		Participants []participant `json:"participants"`
	}
	var readback pullRequest
	reviewer.mustJSON(&readback, "pr", "view", id, "--repo", target)
	if readback.ID != created.ID || readback.Author.UUID != authorID.UUID || readback.State != "OPEN" {
		t.Fatalf("reviewer PR readback identity differs: %+v", readback)
	}
	for _, p := range readback.Participants {
		if p.User.UUID == reviewerID.UUID && p.Approved {
			t.Fatal("new owned PR unexpectedly already approved by reviewer")
		}
	}
	var approved participant
	reviewer.mustJSON(&approved, "pr", "approve", id, "--repo", target)
	if !approved.Approved || approved.User.UUID != reviewerID.UUID {
		t.Fatalf("approval attributed to wrong identity: %+v", approved)
	}
	waitApproval := func(want bool) {
		t.Helper()
		deadline := time.Now().Add(30 * time.Second)
		for time.Now().Before(deadline) {
			owner.mustJSON(&readback, "pr", "view", id, "--repo", target, "--timeout", "10s")
			if readback.ID != created.ID || readback.Author.UUID != authorID.UUID || readback.State != "OPEN" {
				t.Fatalf("approval mutation changed PR identity/state: %+v", readback)
			}
			count := 0
			for _, p := range readback.Participants {
				if p.User.UUID == reviewerID.UUID && p.Approved {
					count++
				}
			}
			if (want && count == 1) || (!want && count == 0) {
				return
			}
			if count > 1 {
				t.Fatal("reviewer approval duplicated in readback")
			}
			time.Sleep(time.Second)
		}
		t.Fatalf("reviewer approval presence did not become %t in owner readback", want)
	}
	waitApproval(true)
	var removed struct {
		ID     int    `json:"id"`
		Action string `json:"action"`
		Done   bool   `json:"done"`
	}
	reviewer.mustJSON(&removed, "pr", "unapprove", id, "--repo", target)
	if removed.ID != created.ID || removed.Action != "unapprove" || !removed.Done {
		t.Fatalf("unapproval result differs: %+v", removed)
	}
	waitApproval(false)
}
