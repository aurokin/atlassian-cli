//go:build integration

package integration

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

// jiraSession builds an authenticated atl-jira session, skipping when Jira is
// not configured for this run.
func jiraSession(t *testing.T) *session { return newSession(t, jiraProduct) }

// jiraProject returns the fixture project key, skipping when it is unset. Every
// project-scoped test needs a real project to target.
func jiraProject(t *testing.T) string {
	t.Helper()
	key := jiraProduct.env("PROJECT")
	if key == "" {
		t.Skip("set ATL_IT_JIRA_PROJECT to a project key to run project-scoped Jira tests")
	}
	return key
}

// jiraIssueType returns the issue type to create, defaulting to Task.
func jiraIssueType() string {
	if v := jiraProduct.env("ISSUE_TYPE"); v != "" {
		return v
	}
	return "Task"
}

// mustWrite runs a mutating command: a scope/permission failure skips the test
// (a tenant/app gap, not a CLI defect); any other failure is fatal.
func (s *session) mustWrite(op string, args ...string) cmdResult {
	s.t.Helper()
	res := s.run(args...)
	s.skipIfScopeOrPermission(res, op)
	if res.err != nil {
		s.t.Fatalf("%s failed: %v\nstdout:\n%s\nstderr:\n%s", op, res.err, res.stdout, res.stderr)
	}
	return res
}

func TestJiraStatus(t *testing.T) {
	s := jiraSession(t)
	var status struct {
		AccountID    string `json:"accountId"`
		EmailAddress string `json:"emailAddress"`
		Active       bool   `json:"active"`
	}
	s.mustJSON(&status, "status")
	if status.AccountID == "" {
		t.Fatal("status returned an empty accountId")
	}
	if !status.Active {
		t.Fatal("status reported an inactive account")
	}
}

func TestJiraProjectList(t *testing.T) {
	s := jiraSession(t)
	// `project list --json` renders Jira's raw paginated response.
	var projects struct {
		Values []struct {
			Key string `json:"key"`
		} `json:"values"`
	}
	s.mustJSON(&projects, "project", "list")
	if len(projects.Values) == 0 {
		t.Fatal("project list returned no projects")
	}
	if want := jiraProduct.env("PROJECT"); want != "" {
		found := false
		for _, p := range projects.Values {
			if p.Key == want {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("fixture project %q not present in project list", want)
		}
	}
}

func TestJiraProjectView(t *testing.T) {
	s := jiraSession(t)
	key := jiraProject(t)
	var project struct {
		Key string `json:"key"`
	}
	s.mustJSON(&project, "project", "view", key)
	if project.Key != key {
		t.Fatalf("project view returned key %q, want %q", project.Key, key)
	}
}

func TestJiraSearch(t *testing.T) {
	s := jiraSession(t)
	key := jiraProject(t)
	// A bounded JQL search should return without error and yield a well-formed
	// issues array (possibly empty for a brand-new project).
	var result struct {
		Issues []struct {
			Key string `json:"key"`
		} `json:"issues"`
	}
	s.mustJSON(&result, "search", "issues", fmt.Sprintf("project = %q ORDER BY created DESC", key), "--limit", "5")
}

func TestJiraIssueList(t *testing.T) {
	s := jiraSession(t)
	key := jiraProject(t)
	res := s.mustRun("issue", "list", "--project", key, "--limit", "5", "--json")
	// Just assert the command produced JSON output; the project may be empty.
	if strings.TrimSpace(res.stdout) == "" {
		t.Fatal("issue list produced no output")
	}
}

// TestJiraIssueLifecycle exercises the reversible write surface end to end: it
// creates a throwaway issue, edits it, comments on it, assigns/watches it, logs
// work, lists its available transitions, and then deletes the issue so the
// tenant is left clean.
func TestJiraIssueLifecycle(t *testing.T) {
	s := jiraSession(t)
	project := jiraProject(t)

	// Whoami, for self-assignment and watcher operations.
	var me struct {
		AccountID string `json:"accountId"`
	}
	s.mustJSON(&me, "status")

	stamp := time.Now().UTC().Format("20060102-150405")
	summary := "atl-cli integration issue " + stamp

	// Create — a scope failure here skips the whole lifecycle.
	createRes := s.run("issue", "create",
		"--project", project,
		"--type", jiraIssueType(),
		"--summary", summary,
		"--description", "Created by the atl-cli integration suite. Safe to delete.",
		"--json")
	s.skipIfScopeOrPermission(createRes, "issue create")
	if createRes.err != nil {
		t.Fatalf("issue create failed: %v\nstdout:\n%s\nstderr:\n%s", createRes.err, createRes.stdout, createRes.stderr)
	}

	// The create call succeeded server-side, so an issue now exists. Register
	// cleanup before parsing the response so the issue is always deleted, even
	// if the body turns out to be unparseable. The closure reads `key` at
	// cleanup time (after it is populated below).
	var key string
	t.Cleanup(func() {
		if key == "" {
			t.Logf("cleanup: issue was created but its key could not be parsed; delete it manually.\ncreate output:\n%s", createRes.stdout)
			return
		}
		res := s.run("api", "/issue/"+key, "--method", "DELETE")
		if res.err != nil {
			t.Logf("cleanup: failed to delete issue %s (delete it manually): %v\n%s", key, res.err, res.stdout+res.stderr)
			return
		}
		t.Logf("cleanup: deleted issue %s", key)
	})

	var created struct {
		Key string `json:"key"`
	}
	if err := jsonUnmarshal(createRes.stdout, &created); err != nil || created.Key == "" {
		t.Fatalf("could not parse created issue key: %v\nstdout:\n%s", err, createRes.stdout)
	}
	key = created.Key
	t.Logf("created issue %s", key)

	// View it back.
	var viewed struct {
		Key string `json:"key"`
	}
	s.mustJSON(&viewed, "issue", "view", key)
	if viewed.Key != key {
		t.Fatalf("issue view returned %q, want %q", viewed.Key, key)
	}

	// Edit the summary.
	editRes := s.mustWrite("issue edit", "issue", "edit", key, "--summary", summary+" (edited)")
	if !strings.Contains(editRes.stdout, "updated "+key) {
		t.Fatalf("issue edit output unexpected: %q", editRes.stdout)
	}

	// Comment lifecycle: create → list → edit → delete.
	commentRes := s.mustWrite("issue comment create",
		"issue", "comment", "create", key, "--body", "integration comment "+stamp, "--json")
	var comment struct {
		ID string `json:"id"`
	}
	if err := jsonUnmarshal(commentRes.stdout, &comment); err != nil || comment.ID == "" {
		t.Fatalf("could not parse created comment id: %v\nstdout:\n%s", err, commentRes.stdout)
	}

	listComments := s.mustRun("issue", "comment", "list", key, "--json")
	if !strings.Contains(listComments.stdout, comment.ID) {
		t.Fatalf("comment %s not found in comment list:\n%s", comment.ID, listComments.stdout)
	}

	s.mustWrite("issue comment edit",
		"issue", "comment", "edit", key, comment.ID, "--body", "integration comment edited "+stamp)

	delComment := s.mustWrite("issue comment delete", "issue", "comment", "delete", key, comment.ID, "--json")
	var deleted struct {
		Deleted bool `json:"deleted"`
	}
	if err := jsonUnmarshal(delComment.stdout, &deleted); err != nil || !deleted.Deleted {
		t.Fatalf("comment delete did not report success: %v\nstdout:\n%s", err, delComment.stdout)
	}

	// Assign to self, then exercise watch/watchers/unwatch.
	assignRes := s.mustWrite("issue assign", "issue", "assign", key, me.AccountID)
	if !strings.Contains(assignRes.stdout, "assigned "+key) {
		t.Fatalf("issue assign output unexpected: %q", assignRes.stdout)
	}

	watchRes := s.mustWrite("issue watch", "issue", "watch", key)
	if !strings.Contains(watchRes.stdout, "watching "+key) {
		t.Fatalf("issue watch output unexpected: %q", watchRes.stdout)
	}
	s.mustRun("issue", "watchers", key, "--json")
	unwatchRes := s.mustWrite("issue unwatch", "issue", "unwatch", key)
	if !strings.Contains(unwatchRes.stdout, "no longer watching "+key) {
		t.Fatalf("issue unwatch output unexpected: %q", unwatchRes.stdout)
	}

	// Log work, then read it back.
	worklogRes := s.mustWrite("issue worklog add", "issue", "worklog", "add", key, "--time", "5m", "--comment", "integration worklog "+stamp, "--json")
	var worklog struct {
		ID string `json:"id"`
	}
	if err := jsonUnmarshal(worklogRes.stdout, &worklog); err != nil || worklog.ID == "" {
		t.Fatalf("could not parse worklog id: %v\nstdout:\n%s", err, worklogRes.stdout)
	}
	worklogList := s.mustRun("issue", "worklog", "list", key, "--json")
	if !strings.Contains(worklogList.stdout, worklog.ID) {
		t.Fatalf("worklog %s not found in worklog list:\n%s", worklog.ID, worklogList.stdout)
	}

	// List available transitions (read), then best-effort apply the first one.
	var transitions struct {
		Transitions []struct {
			Name string `json:"name"`
		} `json:"transitions"`
	}
	s.mustJSON(&transitions, "issue", "transition", key)
	if len(transitions.Transitions) > 0 {
		name := transitions.Transitions[0].Name
		applyRes := s.run("issue", "transition", key, "--to", name, "--json")
		if applyRes.err != nil {
			// Some transitions require screens/fields; that is not a CLI defect.
			t.Logf("transition to %q not applied (likely requires a screen): %s", name, applyRes.stdout+applyRes.stderr)
		} else {
			t.Logf("applied transition %q to %s", name, key)
		}
	}
}
