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

func confSession(t *testing.T) *session { return newSession(t, confProduct) }
func confSpace(t *testing.T) string {
	t.Helper()
	key := confProduct.env("SPACE")
	if key == "" {
		t.Fatal("set ATL_IT_CONF_SPACE to the dedicated fixture space key")
	}
	return key
}

type confContent struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Status   string `json:"status"`
	ParentID string `json:"parentId"`
	Version  struct {
		Number int `json:"number"`
	} `json:"version"`
	Body struct {
		Storage struct {
			Value string `json:"value"`
		} `json:"storage"`
		ADF struct {
			Value string `json:"value"`
		} `json:"atlas_doc_format"`
	} `json:"body"`
}
type confIDs struct {
	Results []struct {
		ID   string `json:"id"`
		Type string `json:"type"`
		Name string `json:"name"`
	} `json:"results"`
}

func confTitle() string { return fmt.Sprintf("atl-cli-e2e-%d", time.Now().UnixNano()) }

// Only resources created by this test are eligible for cleanup. Children are
// registered after parents so testing's LIFO cleanup removes them first.
func confCreate(t *testing.T, s *session, kind, body, format string) confContent {
	t.Helper()
	var c confContent
	s.mustJSON(&c, kind, "create", "--space", confSpace(t), "--title", confTitle(), "--body", body, "--body-format", format)
	if c.ID == "" {
		t.Fatal("creation returned no ID; reconcile the named fixture before rerunning")
	}
	t.Logf("owned %s %s", kind, c.ID)
	s.cleanupOwned(kind, c.ID, func() bool { return confDelete(t, s, kind, c.ID) })
	return c
}
func confDelete(t *testing.T, s *session, kind, id string) bool {
	t.Helper()
	path := "/" + kind + "s/" + id
	res := s.run("api", path+"?status=current,trashed", "--json")
	if res.err != nil {
		if strings.Contains(res.stderr, "not_found_or_not_visible") {
			return true
		}
		t.Errorf("cleanup inspect %s %s: %s %s", kind, id, res.stdout, res.stderr)
		return false
	}
	var c confContent
	if err := json.Unmarshal([]byte(res.stdout), &c); err != nil {
		t.Errorf("cleanup inspect: %v", err)
		return false
	}
	if c.Status != "trashed" {
		res = s.run("api", path, "--method", "DELETE")
		if res.err != nil {
			t.Errorf("cleanup trash %s: %s %s", id, res.stdout, res.stderr)
			return false
		}
	}
	res = s.run("api", path+"?purge=true", "--method", "DELETE")
	if res.err != nil {
		t.Errorf("cleanup purge %s: %s %s", id, res.stdout, res.stderr)
		return false
	}
	res = s.run("api", path+"?status=current,trashed", "--json")
	if res.err == nil || !strings.Contains(res.stderr, "not_found_or_not_visible") {
		t.Errorf("purged %s still visible or verification failed: %s %s", id, res.stdout, res.stderr)
		return false
	}
	return true
}
func confHas(t *testing.T, list confIDs, id string) {
	t.Helper()
	n := 0
	for _, v := range list.Results {
		if v.ID == id {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("expected ID %s exactly once, got %d in %+v", id, n, list)
	}
}

func TestConfStatus(t *testing.T) {
	s := confSession(t)
	var status struct {
		AccountID string `json:"accountId"`
	}
	s.mustJSON(&status, "status")
	if status.AccountID == "" {
		t.Fatal("missing accountId")
	}
}
func TestConfSpaceList(t *testing.T) {
	s := confSession(t)
	var spaces struct {
		Results []struct {
			Key string `json:"key"`
		} `json:"results"`
	}
	s.mustJSON(&spaces, "space", "list", "--all", "--limit", "1")
	for _, v := range spaces.Results {
		if v.Key == confSpace(t) {
			return
		}
	}
	t.Fatal("fixture space missing")
}
func TestConfSpaceView(t *testing.T) {
	s := confSession(t)
	var space struct {
		ID  string `json:"id"`
		Key string `json:"key"`
	}
	s.mustJSON(&space, "space", "view", confSpace(t))
	if space.ID == "" || space.Key != confSpace(t) {
		t.Fatal("wrong space", space)
	}
}

// Confluence Cloud search propagates asynchronously. A controlled tenant probe
// took over five minutes; eight minutes bounds readiness within the matrix's
// twenty-minute cell budget for two search cases. See CONFCLOUD-80582.
func confWaitForSearch(t *testing.T, s *session, id string, args ...string) {
	t.Helper()
	started := time.Now()
	deadline := started.Add(8 * time.Minute)
	queryArgs := append([]string{}, args...)
	queryArgs = append(queryArgs, "--timeout", "10s")
	for attempt := 1; ; attempt++ {
		var list struct {
			Results []struct {
				Content struct {
					ID string `json:"id"`
				} `json:"content"`
			} `json:"results"`
		}
		// Command/auth/HTTP failures remain fatal; only empty search results wait
		// for indexing. Resource creation is never retried.
		s.mustJSON(&list, queryArgs...)
		elapsed := time.Since(started).Round(time.Second)
		if len(list.Results) > 0 {
			if len(list.Results) != 1 || list.Results[0].Content.ID != id {
				t.Fatalf("search returned wrong fixture %s: %+v", id, list)
			}
			t.Logf("search indexed owned page %s after %s (%d read attempts)", id, elapsed, attempt)
			return
		}
		if !time.Now().Before(deadline) {
			t.Fatalf("owned page %s not searchable after %s (%d read attempts)", id, elapsed, attempt)
		}
		t.Logf("waiting for search index: page=%s elapsed=%s attempt=%d", id, elapsed, attempt)
		time.Sleep(min(10*time.Second, time.Until(deadline)))
	}
}

func TestConfSearchCQL(t *testing.T) {
	s := confSession(t)
	p := confCreate(t, s, "page", "<p>search fixture</p>", "storage")
	confWaitForSearch(t, s, p.ID, "search", "cql", fmt.Sprintf("id = %s", p.ID), "--all", "--limit", "1")
}
func TestConfPageListAndView(t *testing.T) {
	s := confSession(t)
	p := confCreate(t, s, "page", "<p>list fixture</p>", "storage")
	var list confIDs
	s.mustJSON(&list, "page", "list", "--space", confSpace(t), "--all", "--limit", "1")
	confHas(t, list, p.ID)
	var viewed confContent
	s.mustJSON(&viewed, "page", "view", p.ID)
	if viewed.ID != p.ID || viewed.Title != p.Title {
		t.Fatal("page identity mismatch", viewed)
	}
}
func TestConfLabelLifecycle(t *testing.T) {
	s := confSession(t)
	p := confCreate(t, s, "page", "<p>labels</p>", "storage")
	labels := []string{"e2e-label-a", "e2e-label-b", "e2e-label-c"}
	for _, label := range labels {
		s.mustRun("page", "label", "add", p.ID, label, "--json")
	}
	var list confIDs
	s.mustJSON(&list, "page", "label", "list", p.ID, "--all", "--limit", "1")
	for _, label := range labels {
		found := false
		for _, v := range list.Results {
			if v.Name == label {
				found = true
			}
		}
		if !found {
			t.Fatalf("missing label %s", label)
		}
	}
	var removed struct {
		Page    string `json:"page"`
		Label   string `json:"label"`
		Removed bool   `json:"removed"`
	}
	s.mustJSON(&removed, "page", "label", "remove", p.ID, labels[0])
	if removed.Page != p.ID || removed.Label != labels[0] || !removed.Removed {
		t.Fatal("bad removal result", removed)
	}
	s.mustJSON(&list, "page", "label", "list", p.ID, "--all", "--limit", "1")
	for _, label := range labels[1:] {
		found := false
		for _, v := range list.Results {
			if v.Name == label {
				found = true
			}
		}
		if !found {
			t.Fatalf("unrelated label %s removed", label)
		}
	}
	for _, v := range list.Results {
		if v.Name == labels[0] {
			t.Fatal("removed label still listed")
		}
	}
}
func TestConfPageLifecycle(t *testing.T) {
	s := confSession(t)
	p := confCreate(t, s, "page", "<p>Body must survive title edit</p>", "storage")
	var before, after confContent
	s.mustJSON(&before, "page", "view", p.ID)
	if !strings.Contains(before.Body.Storage.Value, "Body must survive title edit") {
		t.Fatal("created page body missing")
	}
	s.mustRun("page", "edit", p.ID, "--title", p.Title+" edited", "--json")
	s.mustJSON(&after, "page", "view", p.ID)
	if after.Title != p.Title+" edited" || after.Version.Number != before.Version.Number+1 || after.Body.Storage.Value != before.Body.Storage.Value {
		t.Fatal("title edit failed to preserve body/version", after)
	}
	var versions struct {
		Results []struct {
			Number int `json:"number"`
		} `json:"results"`
	}
	s.mustJSON(&versions, "page", "versions", p.ID, "--all", "--limit", "1")
	if len(versions.Results) != 2 || versions.Results[0].Number == versions.Results[1].Number {
		t.Fatal("version pagination did not retrieve both versions")
	}
	var deletion struct {
		ID      string `json:"id"`
		Deleted bool   `json:"deleted"`
		Purged  bool   `json:"purged"`
	}
	s.mustJSON(&deletion, "page", "delete", p.ID)
	if deletion.ID != p.ID || !deletion.Deleted || deletion.Purged {
		t.Fatal("bad trash result", deletion)
	}
	s.mustJSON(&after, "api", "/pages/"+p.ID+"?status=trashed")
	if after.Status != "trashed" {
		t.Fatal("page not trashed", after)
	}
	s.mustJSON(&deletion, "page", "delete", p.ID, "--purge", "--yes")
	if !deletion.Purged || !deletion.Deleted || deletion.ID != p.ID {
		t.Fatal("bad purge result", deletion)
	}
}
func TestConfPageADFTitleEdit(t *testing.T) {
	s := confSession(t)
	adf := `{"version":1,"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"E2E ADF content survives"}]}]}`
	p := confCreate(t, s, "page", adf, "atlas_doc_format")
	var before, after confContent
	s.mustJSON(&before, "api", "/pages/"+p.ID+"?body-format=atlas_doc_format")
	s.mustRun("page", "edit", p.ID, "--title", p.Title+" edited", "--json")
	s.mustJSON(&after, "api", "/pages/"+p.ID+"?body-format=atlas_doc_format")
	if !strings.Contains(before.Body.ADF.Value, "E2E ADF content survives") {
		t.Fatal("created ADF body missing")
	}
	var oldBody, newBody any
	if json.Unmarshal([]byte(before.Body.ADF.Value), &oldBody) != nil || json.Unmarshal([]byte(after.Body.ADF.Value), &newBody) != nil {
		t.Fatal("invalid ADF response")
	}
	a, _ := json.Marshal(oldBody)
	b, _ := json.Marshal(newBody)
	if !bytes.Equal(a, b) || after.Title != p.Title+" edited" || after.Version.Number != before.Version.Number+1 {
		t.Fatal("ADF body/title/version changed unexpectedly")
	}
}
func TestConfComments(t *testing.T) {
	s := confSession(t)
	p := confCreate(t, s, "page", "<p>comments</p>", "storage")
	ids := make([]string, 0, 3)
	for i := 0; i < 3; i++ {
		var c confContent
		s.mustJSON(&c, "page", "comment", "create", p.ID, "--body", fmt.Sprintf("<p>comment %d</p>", i), "--body-format", "storage")
		if c.ID == "" {
			t.Fatal("missing comment ID")
		}
		ids = append(ids, c.ID)
	}
	var list confIDs
	s.mustJSON(&list, "page", "comment", "list", p.ID, "--all", "--limit", "1")
	for _, id := range ids {
		confHas(t, list, id)
	}
	var before, after confContent
	s.mustJSON(&before, "page", "comment", "view", ids[0])
	s.mustRun("page", "comment", "edit", ids[0], "--body", "<p>edited comment</p>", "--body-format", "storage", "--json")
	s.mustJSON(&after, "page", "comment", "view", ids[0])
	if !strings.Contains(after.Body.Storage.Value, "edited comment") || after.Version.Number != before.Version.Number+1 {
		t.Fatal("comment edit not reflected", after)
	}
	var deleted struct {
		ID      string `json:"id"`
		Deleted bool   `json:"deleted"`
	}
	s.mustJSON(&deleted, "page", "comment", "delete", ids[0], "--yes")
	if deleted.ID != ids[0] || !deleted.Deleted {
		t.Fatal("bad comment deletion result", deleted)
	}
	res := s.run("page", "comment", "view", ids[0], "--json")
	if res.err == nil || !strings.Contains(res.stderr, "not_found_or_not_visible") {
		t.Fatal("deleted comment remains visible", res)
	}
}
func TestConfChildren(t *testing.T) {
	s := confSession(t)
	p := confCreate(t, s, "page", "<p>parent</p>", "storage")
	var space struct {
		ID string `json:"id"`
	}
	s.mustJSON(&space, "space", "view", confSpace(t))
	ids := make([]string, 0, 3)
	for i := 0; i < 3; i++ {
		body, _ := json.Marshal(map[string]any{"spaceId": space.ID, "parentId": p.ID, "status": "current", "title": confTitle(), "body": map[string]string{"representation": "storage", "value": "<p>child</p>"}})
		var child confContent
		s.mustJSON(&child, "api", "/pages", "--method", "POST", "--data", string(body))
		if child.ID == "" {
			t.Fatal("missing child ID")
		}
		s.cleanupOwned("page", child.ID, func() bool { return confDelete(t, s, "page", child.ID) })
		ids = append(ids, child.ID)
		var ancestors confIDs
		s.mustJSON(&ancestors, "page", "ancestors", child.ID, "--all", "--limit", "1")
		confHas(t, ancestors, p.ID)
	}
	// A grandchild makes ancestor pagination cross a boundary and distinguishes
	// direct children from all descendants.
	body, _ := json.Marshal(map[string]any{"spaceId": space.ID, "parentId": ids[0], "status": "current", "title": confTitle(), "body": map[string]string{"representation": "storage", "value": "<p>grandchild</p>"}})
	var grandchild confContent
	s.mustJSON(&grandchild, "api", "/pages", "--method", "POST", "--data", string(body))
	if grandchild.ID == "" {
		t.Fatal("missing grandchild ID")
	}
	s.cleanupOwned("page", grandchild.ID, func() bool { return confDelete(t, s, "page", grandchild.ID) })
	var ancestors confIDs
	s.mustJSON(&ancestors, "page", "ancestors", grandchild.ID, "--all", "--limit", "1")
	confHas(t, ancestors, p.ID)
	confHas(t, ancestors, ids[0])

	var children confIDs
	s.mustJSON(&children, "page", "children", p.ID, "--all", "--limit", "1")
	for _, id := range ids {
		confHas(t, children, id)
	}
	if len(children.Results) != len(ids) {
		t.Fatalf("direct children included unexpected descendants: %+v", children)
	}
	for _, v := range children.Results {
		if v.Type != "page" || v.ID == grandchild.ID {
			t.Fatalf("wrong direct-child type: %+v", v)
		}
	}
}
func TestConfAttachmentRoundTrip(t *testing.T) {
	s := confSession(t)
	p := confCreate(t, s, "page", "<p>attachment</p>", "storage")
	data := []byte{'a', 0, 255, 1, '\n', 'z'}
	file := filepath.Join(t.TempDir(), "e2e.bin")
	if err := os.WriteFile(file, data, 0600); err != nil {
		t.Fatal(err)
	}
	var uploaded confIDs
	s.mustJSON(&uploaded, "attachment", "upload", p.ID, "--file", file)
	if len(uploaded.Results) != 1 || uploaded.Results[0].ID == "" {
		t.Fatal("attachment upload returned no ID", uploaded)
	}
	id := uploaded.Results[0].ID
	var list confIDs
	s.mustJSON(&list, "attachment", "list", p.ID, "--all", "--limit", "1")
	confHas(t, list, id)
	res := s.mustRun("attachment", "download", id, "--out", "-")
	if !bytes.Equal([]byte(res.stdout), data) {
		t.Fatal("stdout attachment bytes differ")
	}
	out := filepath.Join(t.TempDir(), "download.bin")
	s.mustRun("attachment", "download", id, "--out", out)
	actual, err := os.ReadFile(out)
	if err != nil || !bytes.Equal(actual, data) {
		t.Fatal("download file differs", err)
	}
}
func TestConfBlogpostLifecycle(t *testing.T) {
	s := confSession(t)
	p := confCreate(t, s, "blogpost", "<p>blog body</p>", "storage")
	var before, after confContent
	s.mustJSON(&before, "blogpost", "view", p.ID)
	if !strings.Contains(before.Body.Storage.Value, "blog body") {
		t.Fatal("created blogpost body missing")
	}
	s.mustRun("blogpost", "edit", p.ID, "--title", p.Title+" edited", "--json")
	s.mustJSON(&after, "blogpost", "view", p.ID)
	if after.Title != p.Title+" edited" || after.Body.Storage.Value != before.Body.Storage.Value || after.Version.Number != before.Version.Number+1 {
		t.Fatal("blogpost edit mismatch", after)
	}
	var list confIDs
	s.mustJSON(&list, "blogpost", "list", "--space", confSpace(t), "--all", "--limit", "1")
	confHas(t, list, p.ID)
}

func TestConfSearchText(t *testing.T) {
	s := confSession(t)
	marker := fmt.Sprintf("atle2esearch%d", time.Now().UnixNano())
	p := confCreate(t, s, "page", "<p>"+marker+"</p>", "storage")
	confWaitForSearch(t, s, p.ID, "search", "text", marker, "--space", confSpace(t), "--type", "page", "--all", "--limit", "1")
}
