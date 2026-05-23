//go:build integration

package integration

import (
	"strings"
	"testing"
	"time"
)

// bbSession builds an authenticated atl-bb session, skipping when Bitbucket is
// not configured for this run.
func bbSession(t *testing.T) *session { return newSession(t, bbProduct) }

// bbWorkspace returns the fixture workspace slug, skipping when unset.
func bbWorkspace(t *testing.T) string {
	t.Helper()
	ws := bbProduct.env("WORKSPACE")
	if ws == "" {
		t.Skip("set ATL_IT_BB_WORKSPACE to a workspace slug to run workspace-scoped Bitbucket tests")
	}
	return ws
}

// bbRepo returns the fixture repository target as <workspace>/<repo>, skipping
// when either piece is unset. Repo-scoped tests need a real repository with at
// least one commit.
func bbRepo(t *testing.T) (workspace, repo, target string) {
	t.Helper()
	workspace = bbWorkspace(t)
	repo = bbProduct.env("REPO")
	if repo == "" {
		t.Skip("set ATL_IT_BB_REPO to a repository slug to run repo-scoped Bitbucket tests")
	}
	return workspace, repo, workspace + "/" + repo
}

// headCommit returns the hash of the most recent commit on the repo's default
// history, used as the target for branch/tag creation.
func headCommit(t *testing.T, s *session, target string) string {
	t.Helper()
	var commits struct {
		Values []struct {
			Hash string `json:"hash"`
		} `json:"values"`
	}
	s.mustJSON(&commits, "commit", "list", "--repo", target, "--limit", "1")
	if len(commits.Values) == 0 || commits.Values[0].Hash == "" {
		t.Skipf("repo %q has no commits to anchor branch/tag creation", target)
	}
	return commits.Values[0].Hash
}

func TestBitbucketStatus(t *testing.T) {
	s := bbSession(t)
	var status struct {
		Username  string `json:"username"`
		AccountID string `json:"account_id"`
	}
	s.mustJSON(&status, "status")
	if status.Username == "" && status.AccountID == "" {
		t.Fatalf("status returned neither username nor account_id:\n%+v", status)
	}
}

// TestBitbucketWorkspaceView exercises a single-workspace lookup, which is the
// supported way to read workspace data.
//
// NOTE: there is deliberately no TestBitbucketWorkspaceList. Bitbucket removed
// the cross-workspace user-enumeration endpoint (GET /2.0/workspaces) on
// 2026-04-14 (changelog CHANGE-3022), and there is no API-token replacement for
// listing the workspaces an account belongs to, so the `atl-bb workspace list`
// command was removed; workspaces are addressed by slug via `workspace view`.
func TestBitbucketWorkspaceView(t *testing.T) {
	s := bbSession(t)
	ws := bbWorkspace(t)
	var workspace struct {
		Slug string `json:"slug"`
	}
	s.mustJSON(&workspace, "workspace", "view", ws)
	if workspace.Slug != ws {
		t.Fatalf("workspace view returned slug %q, want %q", workspace.Slug, ws)
	}
}

func TestBitbucketRepoList(t *testing.T) {
	s := bbSession(t)
	ws := bbWorkspace(t)
	var repos struct {
		Values []struct {
			FullName string `json:"full_name"`
		} `json:"values"`
	}
	s.mustJSON(&repos, "repo", "list", "--workspace", ws)
	// A brand-new workspace may have no repos; just assert the call shaped output.
	for _, r := range repos.Values {
		if r.FullName == "" {
			t.Fatalf("repo list returned an entry with no full_name:\n%+v", repos)
		}
	}
}

func TestBitbucketRepoView(t *testing.T) {
	s := bbSession(t)
	_, _, target := bbRepo(t)
	var repo struct {
		FullName string `json:"full_name"`
	}
	s.mustJSON(&repo, "repo", "view", "--repo", target)
	if !strings.EqualFold(repo.FullName, target) {
		t.Fatalf("repo view returned full_name %q, want %q", repo.FullName, target)
	}
}

func TestBitbucketCommitList(t *testing.T) {
	s := bbSession(t)
	_, _, target := bbRepo(t)
	var commits struct {
		Values []struct {
			Hash string `json:"hash"`
		} `json:"values"`
	}
	s.mustJSON(&commits, "commit", "list", "--repo", target, "--limit", "5")
	// The repo may be empty; if there are commits, each must carry a hash.
	for _, c := range commits.Values {
		if c.Hash == "" {
			t.Fatal("commit list returned a commit with no hash")
		}
	}
}

func TestBitbucketPRList(t *testing.T) {
	s := bbSession(t)
	_, _, target := bbRepo(t)
	// Listing pull requests in any state should succeed and shape output.
	s.mustRun("pr", "list", "--repo", target, "--state", "ALL", "--limit", "5", "--json")
}

// TestBitbucketBranchLifecycle creates a uniquely-named branch off the repo's
// head commit, confirms it lists/views, then deletes it — a fully reversible
// write through the real branch create/delete commands.
func TestBitbucketBranchLifecycle(t *testing.T) {
	s := bbSession(t)
	_, _, target := bbRepo(t)
	head := headCommit(t, s, target)

	name := "atl-cli-it/" + time.Now().UTC().Format("20060102-150405")

	createRes := s.run("branch", "create", "--repo", target, "--name", name, "--target", head)
	s.skipIfScopeOrPermission(createRes, "branch create")
	if createRes.err != nil {
		t.Fatalf("branch create failed: %v\nstdout:\n%s\nstderr:\n%s", createRes.err, createRes.stdout, createRes.stderr)
	}

	// The branch now exists on the tenant. Register cleanup before asserting on
	// output so a successful create is never orphaned by a later failure.
	deleted := false
	t.Cleanup(func() {
		if deleted {
			return
		}
		res := s.run("branch", "delete", name, "--repo", target)
		if res.err != nil && !strings.Contains(res.stdout+res.stderr, "not_found") {
			t.Logf("cleanup: failed to delete branch %q in %s (delete it manually): %v\n%s",
				name, target, res.err, res.stdout+res.stderr)
		}
	})

	if !strings.Contains(createRes.stdout, "created branch "+name) {
		t.Fatalf("branch create output unexpected: %q", createRes.stdout)
	}

	var branch struct {
		Name string `json:"name"`
	}
	s.mustJSON(&branch, "branch", "view", name, "--repo", target)
	if branch.Name != name {
		t.Fatalf("branch view returned name %q, want %q", branch.Name, name)
	}

	delRes := s.mustWrite("branch delete", "branch", "delete", name, "--repo", target)
	if !strings.Contains(delRes.stdout, "deleted branch "+name) {
		t.Fatalf("branch delete output unexpected: %q", delRes.stdout)
	}
	deleted = true
}

// TestBitbucketTagLifecycle creates a uniquely-named tag off the repo's head
// commit, confirms it views, then deletes it — reversible through the real tag
// create/delete commands.
func TestBitbucketTagLifecycle(t *testing.T) {
	s := bbSession(t)
	_, _, target := bbRepo(t)
	head := headCommit(t, s, target)

	name := "atl-cli-it-" + time.Now().UTC().Format("20060102-150405")

	createRes := s.run("tag", "create", "--repo", target, "--name", name, "--target", head)
	s.skipIfScopeOrPermission(createRes, "tag create")
	if createRes.err != nil {
		t.Fatalf("tag create failed: %v\nstdout:\n%s\nstderr:\n%s", createRes.err, createRes.stdout, createRes.stderr)
	}

	// The tag now exists on the tenant. Register cleanup before asserting on
	// output so a successful create is never orphaned by a later failure.
	deleted := false
	t.Cleanup(func() {
		if deleted {
			return
		}
		res := s.run("tag", "delete", name, "--repo", target)
		if res.err != nil && !strings.Contains(res.stdout+res.stderr, "not_found") {
			t.Logf("cleanup: failed to delete tag %q in %s (delete it manually): %v\n%s",
				name, target, res.err, res.stdout+res.stderr)
		}
	})

	if !strings.Contains(createRes.stdout, "created tag "+name) {
		t.Fatalf("tag create output unexpected: %q", createRes.stdout)
	}

	var tag struct {
		Name string `json:"name"`
	}
	s.mustJSON(&tag, "tag", "view", name, "--repo", target)
	if tag.Name != name {
		t.Fatalf("tag view returned name %q, want %q", tag.Name, name)
	}

	delRes := s.mustWrite("tag delete", "tag", "delete", name, "--repo", target)
	if !strings.Contains(delRes.stdout, "deleted tag "+name) {
		t.Fatalf("tag delete output unexpected: %q", delRes.stdout)
	}
	deleted = true
}
