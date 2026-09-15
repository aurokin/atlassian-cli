//go:build integration

package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// jiraSession builds an authenticated atl-jira session for the selected run.
func jiraSession(t *testing.T) *session { return newSession(t, jiraProduct) }

// jiraProject requires an explicit fixture project key. Every
// project-scoped test needs a real project to target.
func jiraProject(t *testing.T) string {
	t.Helper()
	key := jiraProduct.env("PROJECT")
	if key == "" {
		t.Fatal("set ATL_IT_JIRA_PROJECT to a project key to run project-scoped Jira tests")
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

func TestJiraFields(t *testing.T) {
	s := jiraSession(t)
	var fields []struct {
		ID string `json:"id"`
	}
	s.mustJSON(&fields, "field", "list")
	for _, field := range fields {
		if field.ID == "summary" {
			return
		}
	}
	t.Fatal("field list omitted the summary field")
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

	stamp := time.Now().UTC().Format("20060102-150405.000000000")
	summary := "atl-cli integration issue " + stamp

	// Create a uniquely owned issue; every selected write must succeed.
	createRes := s.run("issue", "create",
		"--project", project,
		"--type", jiraIssueType(),
		"--summary", summary,
		"--description", "Created by the atl-cli integration suite. Safe to delete.",
		"--json")
	s.failIfScopeOrPermission(createRes, "issue create")
	if createRes.err != nil {
		t.Fatalf("issue create failed: %v\nstdout:\n%s\nstderr:\n%s", createRes.err, createRes.stdout, createRes.stderr)
	}

	var key string
	var created struct {
		Key string `json:"key"`
	}
	if err := jsonUnmarshal(createRes.stdout, &created); err != nil || created.Key == "" {
		t.Fatalf("could not parse created issue key: %v\nstdout:\n%s", err, createRes.stdout)
	}
	key = created.Key
	cleanupJiraIssue(s, key)
	t.Logf("created issue %s", key)

	// View it back.
	var viewed struct {
		Key    string `json:"key"`
		Fields struct {
			Summary  string `json:"summary"`
			Assignee *struct {
				AccountID string `json:"accountId"`
			} `json:"assignee"`
			Status struct {
				ID string `json:"id"`
			} `json:"status"`
		} `json:"fields"`
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

	s.mustJSON(&viewed, "issue", "view", key, "--fields", "summary")
	if viewed.Fields.Summary != summary+" (edited)" {
		t.Fatalf("edited summary not persisted: %q", viewed.Fields.Summary)
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

	var comments struct {
		Comments []struct {
			ID   string          `json:"id"`
			Body json.RawMessage `json:"body"`
		} `json:"comments"`
	}
	s.mustJSON(&comments, "issue", "comment", "list", key, "--limit", "1", "--all")
	if len(comments.Comments) != 1 || comments.Comments[0].ID != comment.ID {
		t.Fatalf("created comment absent: %+v", comments)
	}
	editedBody := "integration comment edited " + stamp
	s.mustWrite("issue comment edit", "issue", "comment", "edit", key, comment.ID, "--body", editedBody)
	var readComment struct {
		ID   string          `json:"id"`
		Body json.RawMessage `json:"body"`
	}
	s.mustJSON(&readComment, "issue", "comment", "view", key, comment.ID)
	if readComment.ID != comment.ID || !strings.Contains(string(readComment.Body), editedBody) {
		t.Fatalf("comment edit not persisted: %+v", readComment)
	}

	delComment := s.mustWrite("issue comment delete", "issue", "comment", "delete", key, comment.ID, "--yes", "--json")
	var deleted struct {
		Deleted bool `json:"deleted"`
	}
	if err := jsonUnmarshal(delComment.stdout, &deleted); err != nil || !deleted.Deleted {
		t.Fatalf("comment delete did not report success: %v\nstdout:\n%s", err, delComment.stdout)
	}

	s.assertMissing("issue", "comment", "view", key, comment.ID)

	// Assign to self, then exercise watch/watchers/unwatch.
	assignRes := s.mustWrite("issue assign", "issue", "assign", key, me.AccountID)
	if !strings.Contains(assignRes.stdout, "assigned "+key) {
		t.Fatalf("issue assign output unexpected: %q", assignRes.stdout)
	}

	s.mustJSON(&viewed, "issue", "view", key, "--fields", "assignee")
	if viewed.Fields.Assignee == nil || viewed.Fields.Assignee.AccountID != me.AccountID {
		t.Fatal("assignment not persisted")
	}
	s.mustWrite("unassign", "issue", "assign", key, "-")
	viewed.Fields.Assignee = nil
	s.mustJSON(&viewed, "issue", "view", key, "--fields", "assignee")
	if viewed.Fields.Assignee != nil {
		t.Fatal("unassignment not persisted")
	}
	s.mustWrite("assign self", "issue", "assign", key, "@me")

	watchRes := s.mustWrite("issue watch", "issue", "watch", key)
	if !strings.Contains(watchRes.stdout, "watching "+key) {
		t.Fatalf("issue watch output unexpected: %q", watchRes.stdout)
	}
	var watchers struct {
		IsWatching bool `json:"isWatching"`
	}
	s.mustJSON(&watchers, "issue", "watchers", key)
	if !watchers.IsWatching {
		t.Fatal("watch not persisted")
	}
	unwatchRes := s.mustWrite("issue unwatch", "issue", "unwatch", key)
	if !strings.Contains(unwatchRes.stdout, "no longer watching "+key) {
		t.Fatalf("issue unwatch output unexpected: %q", unwatchRes.stdout)
	}

	s.mustJSON(&watchers, "issue", "watchers", key)
	if watchers.IsWatching {
		t.Fatal("unwatch not persisted")
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

	// Apply an advertised transition and independently verify its target status.
	var transitions struct {
		Transitions []struct {
			ID string `json:"id"`
			To struct {
				ID string `json:"id"`
			} `json:"to"`
		} `json:"transitions"`
	}
	s.mustJSON(&transitions, "issue", "transition", key)
	if len(transitions.Transitions) == 0 {
		t.Fatal("fixture has no available transitions")
	}
	transition := transitions.Transitions[0]
	s.mustWrite("transition", "issue", "transition", key, "--to", transition.ID, "--json")
	s.mustJSON(&viewed, "issue", "view", key, "--fields", "status")
	if viewed.Fields.Status.ID != transition.To.ID {
		t.Fatalf("transition target not persisted: %+v", viewed.Fields.Status)
	}

	t.Run("attachment_roundtrip", func(t *testing.T) {
		child := *s
		child.t = t
		data := []byte{0, 1, 2, 3, 255, 254, '\n', 'a', 't', 'l'}
		source := filepath.Join(t.TempDir(), "integration-binary.bin")
		if err := os.WriteFile(source, data, 0600); err != nil {
			t.Fatal(err)
		}
		var attachments []struct {
			ID       string `json:"id"`
			Filename string `json:"filename"`
			Size     int    `json:"size"`
		}
		child.mustJSON(&attachments, "issue", "attachment", "add", key, "--file", source)
		if len(attachments) != 1 || attachments[0].ID == "" {
			t.Fatalf("invalid upload response: %+v", attachments)
		}
		id := attachments[0].ID
		child.mustJSON(&attachments, "issue", "attachment", "list", key)
		if len(attachments) != 1 || attachments[0].ID != id || attachments[0].Filename != filepath.Base(source) || attachments[0].Size != len(data) {
			t.Fatalf("attachment metadata mismatch: %+v", attachments)
		}
		out := filepath.Join(t.TempDir(), "download.bin")
		child.mustRun("issue", "attachment", "download", id, "--out", out)
		actual, err := os.ReadFile(out)
		if err != nil || !bytes.Equal(actual, data) {
			t.Fatalf("download bytes mismatch: %v", err)
		}
		stdout := child.mustRun("issue", "attachment", "download", id, "--out", "-")
		if !bytes.Equal([]byte(stdout.stdout), data) {
			t.Fatal("stdout download bytes mismatch")
		}
	})
}

// Forced one-item pages prove Jira's cursor traversal contains each owned ID
// exactly once. Search indexing is eventually consistent, so retry reads only.
func TestJiraOwnedPagination(t *testing.T) {
	s := jiraSession(t)
	project := jiraProject(t)
	keys := make([]string, 0, 3)
	for i := 0; i < 3; i++ {
		var issue struct {
			Key string `json:"key"`
		}
		s.mustJSON(&issue, "issue", "create", "--project", project, "--type", jiraIssueType(), "--summary", fmt.Sprintf("atl-cli pagination %d %d", time.Now().UnixNano(), i))
		if issue.Key == "" {
			t.Fatal("create omitted owned issue key; inspect creation response")
		}
		key := issue.Key
		cleanupJiraIssue(s, key)
		keys = append(keys, key)
	}
	query := "key in (" + strings.Join(keys, ",") + ") ORDER BY key ASC"
	deadline := time.Now().Add(time.Minute)
	for attempt := 1; ; attempt++ {
		var result struct {
			Issues []struct {
				Key string `json:"key"`
			} `json:"issues"`
		}
		s.mustJSON(&result, "search", "issues", query, "--limit", "1", "--all")
		counts := map[string]int{}
		for _, issue := range result.Issues {
			counts[issue.Key]++
		}
		complete := len(result.Issues) == len(keys)
		for _, key := range keys {
			complete = complete && counts[key] == 1
		}
		if complete {
			t.Logf("known issue pagination complete after %d read attempts", attempt)
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("pagination never returned exact owned keys %v: %+v", keys, counts)
		}
		time.Sleep(2 * time.Second)
	}
	var listed struct {
		Issues []struct {
			Key string `json:"key"`
		} `json:"issues"`
	}
	s.mustJSON(&listed, "issue", "list", "--project", project, "--limit", "1", "--all")
	counts := map[string]int{}
	for _, issue := range listed.Issues {
		counts[issue.Key]++
	}
	for _, key := range keys {
		if counts[key] != 1 {
			t.Fatalf("issue list returned owned key %s %d times", key, counts[key])
		}
	}
	var linkTypes struct {
		IssueLinkTypes []struct {
			Name string `json:"name"`
		} `json:"issueLinkTypes"`
	}
	s.mustJSON(&linkTypes, "issue", "link", "types")
	if len(linkTypes.IssueLinkTypes) == 0 {
		t.Fatal("fixture has no issue link types")
	}
	linkType := linkTypes.IssueLinkTypes[0].Name
	s.mustWrite("issue link", "issue", "link", keys[0], keys[1], "--type", linkType, "--json")
	var linked struct {
		Fields struct {
			Links []struct {
				Type struct {
					Name string `json:"name"`
				} `json:"type"`
				OutwardIssue struct {
					Key string `json:"key"`
				} `json:"outwardIssue"`
			} `json:"issuelinks"`
		} `json:"fields"`
	}
	s.mustJSON(&linked, "issue", "view", keys[0], "--fields", "issuelinks")
	if len(linked.Fields.Links) != 1 || linked.Fields.Links[0].Type.Name != linkType || linked.Fields.Links[0].OutwardIssue.Key != keys[1] {
		t.Fatalf("created issue link not persisted: %+v", linked)
	}

	// Comments have offset pagination and do not depend on the search index.
	ids := map[string]bool{}
	for i := 0; i < 3; i++ {
		var comment struct {
			ID string `json:"id"`
		}
		s.mustJSON(&comment, "issue", "comment", "create", keys[0], "--body", fmt.Sprintf("pagination comment %d", i))
		if comment.ID == "" {
			t.Fatal("missing comment ID")
		}
		ids[comment.ID] = true
	}
	var comments struct {
		Comments []struct {
			ID string `json:"id"`
		} `json:"comments"`
	}
	s.mustJSON(&comments, "issue", "comment", "list", keys[0], "--limit", "1", "--all")
	if len(comments.Comments) != len(ids) {
		t.Fatalf("comment pagination count: %d, want %d", len(comments.Comments), len(ids))
	}
	for _, comment := range comments.Comments {
		if !ids[comment.ID] {
			t.Fatalf("unexpected or duplicate comment %s", comment.ID)
		}
		delete(ids, comment.ID)
	}
	if len(ids) != 0 {
		t.Fatalf("missing comments: %v", ids)
	}
}

func cleanupJiraIssue(s *session, key string) {
	s.cleanupOwned("jira-issue", key, func() bool {
		res := s.run("api", "/issue/"+key, "--method", "DELETE")
		if res.err != nil {
			s.t.Errorf("delete owned issue %s: %v\n%s", key, res.err, res.stdout+res.stderr)
			return false
		}
		return s.assertMissing("issue", "view", key)
	})
}
