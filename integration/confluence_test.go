//go:build integration

package integration

import (
	"strings"
	"testing"
	"time"
)

// confSession builds an authenticated atl-conf session, skipping when
// Confluence is not configured for this run.
func confSession(t *testing.T) *session { return newSession(t, confProduct) }

// confSpace returns the fixture space key, skipping when it is unset.
func confSpace(t *testing.T) string {
	t.Helper()
	key := confProduct.env("SPACE")
	if key == "" {
		t.Skip("set ATL_IT_CONF_SPACE to a space key to run space-scoped Confluence tests")
	}
	return key
}

// firstPageInSpace returns the id of any page in the fixture space, skipping
// when the space has no pages to operate on.
func firstPageInSpace(t *testing.T, s *session, spaceKey string) string {
	t.Helper()
	// `space view <key>` yields the numeric space id needed by the v2 page list.
	var space struct {
		ID string `json:"id"`
	}
	s.mustJSON(&space, "space", "view", spaceKey)
	if space.ID == "" {
		t.Skipf("space %q has no resolvable id", spaceKey)
	}
	var pages struct {
		Results []struct {
			ID string `json:"id"`
		} `json:"results"`
	}
	s.mustJSON(&pages, "page", "list", "--space", spaceKey, "--limit", "5")
	if len(pages.Results) == 0 {
		t.Skipf("space %q has no pages to exercise page/label reads against", spaceKey)
	}
	return pages.Results[0].ID
}

func TestConfStatus(t *testing.T) {
	s := confSession(t)
	var status struct {
		AccountID string `json:"accountId"`
		Type      string `json:"type"`
	}
	s.mustJSON(&status, "status")
	if status.AccountID == "" {
		t.Fatal("status returned an empty accountId")
	}
}

func TestConfSpaceList(t *testing.T) {
	s := confSession(t)
	var spaces struct {
		Results []struct {
			Key string `json:"key"`
		} `json:"results"`
	}
	s.mustJSON(&spaces, "space", "list")
	if len(spaces.Results) == 0 {
		t.Fatal("space list returned no spaces")
	}
	if want := confProduct.env("SPACE"); want != "" {
		found := false
		for _, sp := range spaces.Results {
			if sp.Key == want {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("fixture space %q not present in space list", want)
		}
	}
}

func TestConfSpaceView(t *testing.T) {
	s := confSession(t)
	key := confSpace(t)
	var space struct {
		Key string `json:"key"`
		ID  string `json:"id"`
	}
	s.mustJSON(&space, "space", "view", key)
	if space.Key != key {
		t.Fatalf("space view returned key %q, want %q", space.Key, key)
	}
	if space.ID == "" {
		t.Fatal("space view returned an empty id")
	}
}

func TestConfSearchCQL(t *testing.T) {
	s := confSession(t)
	// A bounded CQL search should succeed and return a well-formed result set
	// (possibly empty).
	var result struct {
		Results []any `json:"results"`
	}
	s.mustJSON(&result, "search", "cql", "type = page ORDER BY created DESC", "--limit", "5")
}

func TestConfPageListAndView(t *testing.T) {
	s := confSession(t)
	key := confSpace(t)
	pageID := firstPageInSpace(t, s, key)
	var page struct {
		ID string `json:"id"`
	}
	s.mustJSON(&page, "page", "view", pageID)
	if page.ID != pageID {
		t.Fatalf("page view returned id %q, want %q", page.ID, pageID)
	}
}

// TestConfLabelLifecycle adds a uniquely-named label to an existing page,
// confirms it lists, then removes it — a fully reversible write that exercises
// the v1 content-label path.
func TestConfLabelLifecycle(t *testing.T) {
	s := confSession(t)
	key := confSpace(t)
	pageID := firstPageInSpace(t, s, key)

	label := "atl-cli-it-" + time.Now().UTC().Format("20060102-150405")

	addRes := s.run("page", "label", "add", pageID, label)
	s.skipIfScopeOrPermission(addRes, "page label add")
	if addRes.err != nil {
		t.Fatalf("page label add failed: %v\nstdout:\n%s\nstderr:\n%s", addRes.err, addRes.stdout, addRes.stderr)
	}

	// Safety net: remove the label even if a later assertion aborts the test
	// before the explicit removal below. The happy path removes it itself, so a
	// "not found" here just means it is already gone — not a failure.
	t.Cleanup(func() {
		res := s.run("page", "label", "remove", pageID, label)
		if res.err != nil && !strings.Contains(res.stdout+res.stderr, "No label found") {
			t.Logf("cleanup: failed to remove label %q from page %s (remove it manually): %v\n%s",
				label, pageID, res.err, res.stdout+res.stderr)
		}
	})

	listRes := s.mustRun("page", "label", "list", pageID, "--json")
	if !strings.Contains(listRes.stdout, label) {
		t.Fatalf("label %q not found in label list:\n%s", label, listRes.stdout)
	}

	removeRes := s.mustWrite("page label remove", "page", "label", "remove", pageID, label)
	if !strings.Contains(removeRes.stdout, "removed label "+label) {
		t.Fatalf("page label remove output unexpected: %q", removeRes.stdout)
	}
}

// TestConfPageLifecycle creates a throwaway page, edits it, comments on it, and
// deletes it (via the raw api command, since there is no `page delete` verb).
// On a tenant whose app lacks the v2 write:page:confluence scope this skips at
// the create step.
func TestConfPageLifecycle(t *testing.T) {
	s := confSession(t)
	space := confSpace(t)

	stamp := time.Now().UTC().Format("20060102-150405")
	title := "atl-cli integration page " + stamp

	createRes := s.run("page", "create",
		"--space", space,
		"--title", title,
		"--body", "<p>Created by the atl-cli integration suite. Safe to delete.</p>",
		"--body-format", "storage",
		"--json")
	s.skipIfScopeOrPermission(createRes, "page create")
	if createRes.err != nil {
		t.Fatalf("page create failed: %v\nstdout:\n%s\nstderr:\n%s", createRes.err, createRes.stdout, createRes.stderr)
	}

	var pageID string
	t.Cleanup(func() {
		if pageID == "" {
			t.Logf("cleanup: page was created but its id could not be parsed; delete it manually.\ncreate output:\n%s", createRes.stdout)
			return
		}
		res := s.run("api", "/pages/"+pageID, "--method", "DELETE")
		if res.err != nil {
			t.Logf("cleanup: failed to delete page %s (delete it manually): %v\n%s", pageID, res.err, res.stdout+res.stderr)
			return
		}
		t.Logf("cleanup: deleted page %s", pageID)
	})

	var created struct {
		ID string `json:"id"`
	}
	if err := jsonUnmarshal(createRes.stdout, &created); err != nil || created.ID == "" {
		t.Fatalf("could not parse created page id: %v\nstdout:\n%s", err, createRes.stdout)
	}
	pageID = created.ID
	t.Logf("created page %s", pageID)

	// View it back.
	var viewed struct {
		ID string `json:"id"`
	}
	s.mustJSON(&viewed, "page", "view", pageID)
	if viewed.ID != pageID {
		t.Fatalf("page view returned id %q, want %q", viewed.ID, pageID)
	}

	// Edit the title.
	editRes := s.mustWrite("page edit", "page", "edit", pageID, "--title", title+" (edited)")
	if !strings.Contains(editRes.stdout, "updated page "+pageID) {
		t.Fatalf("page edit output unexpected: %q", editRes.stdout)
	}

	// Comment lifecycle: create → delete (scope-permitting).
	commentRes := s.run("page", "comment", "create", pageID, "--body", "integration comment "+stamp, "--json")
	s.skipIfScopeOrPermission(commentRes, "page comment create")
	if commentRes.err != nil {
		t.Fatalf("page comment create failed: %v\nstdout:\n%s\nstderr:\n%s", commentRes.err, commentRes.stdout, commentRes.stderr)
	}
	var comment struct {
		ID string `json:"id"`
	}
	if err := jsonUnmarshal(commentRes.stdout, &comment); err != nil || comment.ID == "" {
		t.Fatalf("could not parse created comment id: %v\nstdout:\n%s", err, commentRes.stdout)
	}
	s.mustWrite("page comment delete", "page", "comment", "delete", comment.ID)
}
