//go:build integration

package integration

import (
	"bytes"
	"context"
	"fmt"
	"mime/multipart"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/aurokin/atlassian-cli/internal/appinfo"
	"github.com/aurokin/atlassian-cli/internal/cli"
)

type bbCommit struct {
	Hash    string `json:"hash"`
	Message string `json:"message"`
}
type bbRef struct {
	Name   string   `json:"name"`
	Target bbCommit `json:"target"`
}
type bbPR struct {
	ID    int    `json:"id"`
	State string `json:"state"`
	Title string `json:"title"`
}

// ownedBBRepo never uses ATL_IT_BB_REPO for writes. The exact private repository
// created here is the only deletion target, including on failed assertions.
func ownedBBRepo(t *testing.T, s *session) string {
	t.Helper()
	ws := bbWorkspace(t)
	var workspace struct {
		Slug string `json:"slug"`
	}
	s.mustJSON(&workspace, "workspace", "view", ws)
	if !strings.EqualFold(workspace.Slug, ws) {
		t.Fatalf("unexpected workspace %q", workspace.Slug)
	}
	target := ws + "/atl-cli-e2e-" + strconv.FormatInt(time.Now().UnixNano(), 36)
	var created struct {
		FullName  string `json:"full_name"`
		IsPrivate bool   `json:"is_private"`
	}
	createResult := s.mustRun("repo", "create", target, "--private", "--description", "Disposable Atlassian CLI live acceptance fixture", "--json")
	t.Logf("owned repository: %s", target)
	s.cleanupOwned("repository", target, func() bool {
		res := s.run("repo", "delete", target, "--yes", "--json")
		if res.err != nil {
			t.Errorf("cleanup failed for owned repository %s: %s %s", target, res.stdout, res.stderr)
			return false
		}
		var deleted struct {
			Resource string `json:"resource"`
			ID       string `json:"id"`
			Deleted  bool   `json:"deleted"`
		}
		if err := jsonUnmarshal(res.stdout, &deleted); err != nil || !deleted.Deleted || deleted.Resource != "repository" || !strings.EqualFold(deleted.ID, target) {
			t.Errorf("invalid repository deletion result: %s", res.stdout)
			return false
		}
		return bbRequireMissing(t, s, "repo", "view", target)
	})
	if err := jsonUnmarshal(createResult.stdout, &created); err != nil {
		t.Fatalf("decode created repository: %v", err)
	}
	if !created.IsPrivate || !strings.EqualFold(created.FullName, target) {
		t.Fatalf("wrong repository creation result: %+v", created)
	}
	var viewed struct {
		FullName  string `json:"full_name"`
		IsPrivate bool   `json:"is_private"`
	}
	s.mustJSON(&viewed, "repo", "view", target)
	if !viewed.IsPrivate || !strings.EqualFold(viewed.FullName, target) {
		t.Fatalf("repository readback differs: %+v", viewed)
	}
	return target
}

func bbRequireMissing(t *testing.T, s *session, args ...string) bool {
	t.Helper()
	res := s.run(append(args, "--json")...)
	if res.err == nil || !strings.Contains(res.stderr, "not_found_or_not_visible") {
		t.Errorf("expected missing resource after deletion for %v: err=%v stdout=%s stderr=%s", args, res.err, res.stdout, res.stderr)
		return false
	}
	return true
}

// Source upload has no CLI write command. This setup uses the production
// authenticated client and official multipart source endpoint; assertions below
// exercise only the built CLI. Credentials stay inside the existing provider.
func seedBBCommit(t *testing.T, s *session, target, branch, parent, path, content string) string {
	t.Helper()
	if s.configHome != "" {
		t.Setenv("XDG_CONFIG_HOME", s.configHome)
	}
	client, err := cli.SiteClient(appinfo.Info{Binary: "atl-bb", Product: appinfo.ProductBitbucket}, &cli.GlobalFlags{Site: s.site})
	if err != nil {
		t.Fatal(err)
	}
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	fields := map[string]string{"branch": branch, "message": "CLI E2E fixture " + branch}
	if parent != "" {
		fields["parents"] = parent
	}
	for name, value := range fields {
		if err := writer.WriteField(name, value); err != nil {
			t.Fatal(err)
		}
	}
	file, err := writer.CreateFormFile(path, path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = file.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
	if err = writer.Close(); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	_, err = client.DoUpload(ctx, "POST", "repositories/"+target+"/src", writer.FormDataContentType(), &body)
	if err != nil {
		t.Fatalf("source fixture setup %s: %v", branch, err)
	}
	var commit bbCommit
	s.mustJSON(&commit, "commit", "view", branch, "--repo", target)
	if commit.Hash == "" {
		t.Fatal("seeded commit has no hash")
	}
	return commit.Hash
}

func TestBitbucketOwnedRefsAndContent(t *testing.T) {
	s := bbSession(t)
	target := ownedBBRepo(t, s)
	ws, slug, _ := strings.Cut(target, "/")
	var repos struct {
		Values []struct {
			FullName string `json:"full_name"`
		} `json:"values"`
	}
	s.mustJSON(&repos, "search", "repos", `slug="`+slug+`"`, "--workspace", ws, "--all", "--limit", "1")
	if len(repos.Values) != 1 || !strings.EqualFold(repos.Values[0].FullName, target) {
		t.Fatalf("repo search failed known membership: %+v", repos)
	}
	var repo struct {
		Project struct {
			Key string `json:"key"`
		} `json:"project"`
	}
	s.mustJSON(&repo, "repo", "view", target)
	if repo.Project.Key == "" {
		t.Fatal("owned repository missing project")
	}
	var project struct {
		Key string `json:"key"`
	}
	s.mustJSON(&project, "project", "view", repo.Project.Key, "--workspace", ws)
	if project.Key != repo.Project.Key {
		t.Fatalf("project view differs: %+v", project)
	}
	var projects struct {
		Values []struct {
			Key string `json:"key"`
		} `json:"values"`
	}
	s.mustJSON(&projects, "project", "list", "--workspace", ws, "--all", "--limit", "1")
	found := false
	for _, entry := range projects.Values {
		if entry.Key == repo.Project.Key {
			found = true
		}
	}
	if !found {
		t.Fatalf("project list missing %s", repo.Project.Key)
	}
	const content = "CLI fixture\n\x00\x01\xff\n"
	first := seedBBCommit(t, s, target, "main", "", "fixture.bin", content)
	second := seedBBCommit(t, s, target, "main", first, "second.txt", "second file\n")
	var history struct {
		Values []bbCommit `json:"values"`
	}
	s.mustJSON(&history, "commit", "list", "--repo", target, "--all", "--limit", "1")
	if len(history.Values) != 2 || history.Values[0].Hash != second || history.Values[1].Hash != first {
		t.Fatalf("unexpected paginated commit history: %+v", history)
	}
	var source struct {
		Values []struct {
			Path string `json:"path"`
		} `json:"values"`
	}
	s.mustJSON(&source, "src", "--repo", target, "--all", "--limit", "1")
	paths := map[string]bool{}
	for _, entry := range source.Values {
		paths[entry.Path] = true
	}
	if len(source.Values) != 2 || !paths["fixture.bin"] || !paths["second.txt"] {
		t.Fatalf("source membership differs: %+v", source)
	}
	if result := s.mustRun("file", "fixture.bin", "--repo", target); result.stdout != content {
		t.Fatalf("file bytes differ: got %x want %x", result.stdout, content)
	}
	for _, kind := range []string{"branch", "tag"} {
		t.Run(kind, func(t *testing.T) {
			child := *s
			child.t = t
			names := []string{"e2e-one", "e2e-two"}
			for _, name := range names {
				var ref bbRef
				child.mustJSON(&ref, kind, "create", "--repo", target, "--name", name, "--target", first)
				child.mustJSON(&ref, kind, "view", name, "--repo", target)
				if ref.Name != name || ref.Target.Hash != first {
					t.Fatalf("unexpected ref: %+v", ref)
				}
			}
			var refs struct {
				Values []bbRef `json:"values"`
			}
			child.mustJSON(&refs, kind, "list", "--repo", target, "--all", "--limit", "1")
			expected := len(names)
			if kind == "branch" {
				expected++
			}
			if len(refs.Values) != expected {
				t.Fatalf("unexpected %s count: got %d want %d", kind, len(refs.Values), expected)
			}
			seen := map[string]int{}
			for _, ref := range refs.Values {
				seen[ref.Name]++
			}
			for _, name := range names {
				if seen[name] != 1 {
					t.Fatalf("%s membership incorrect: %+v", kind, refs)
				}
			}
			for _, name := range names {
				child.mustRun(kind, "delete", name, "--repo", target, "--yes")
				bbRequireMissing(t, &child, kind, "view", name, "--repo", target)
			}
		})
	}
}

func TestBitbucketOwnedPullRequests(t *testing.T) {
	s := bbSession(t)
	target := ownedBBRepo(t, s)
	base := seedBBCommit(t, s, target, "main", "", "README.md", "base\n")
	ids := make([]int, 0, 3)
	hashes := make([]string, 0, 3)
	for i := 0; i < 3; i++ {
		branch := fmt.Sprintf("change-%d", i)
		hashes = append(hashes, seedBBCommit(t, s, target, branch, base, branch+".txt", branch+"\n"))
		var pr bbPR
		s.mustJSON(&pr, "pr", "create", "--repo", target, "--source", branch, "--destination", "main", "--title", "CLI E2E "+branch)
		if pr.ID == 0 || pr.State != "OPEN" {
			t.Fatalf("unexpected PR creation: %+v", pr)
		}
		ids = append(ids, pr.ID)
	}
	for _, args := range [][]string{{"--all"}, {"--all", "--limit", "1"}} {
		var page struct {
			Values []bbPR `json:"values"`
		}
		s.mustJSON(&page, append([]string{"pr", "list", "--repo", target}, args...)...)
		seen := map[int]int{}
		for _, pr := range page.Values {
			seen[pr.ID]++
		}
		if len(page.Values) != len(ids) {
			t.Fatalf("PR pagination %v returned %d, want %d", args, len(page.Values), len(ids))
		}
		for _, id := range ids {
			if seen[id] != 1 {
				t.Fatalf("PR %d missing/duplicated: %+v", id, page)
			}
		}
	}

	var searched struct {
		Values []bbPR `json:"values"`
	}
	s.mustJSON(&searched, "search", "prs", `state="OPEN" AND title ~ "CLI E2E"`, "--repo", target, "--all", "--limit", "1")
	matches := map[int]int{}
	for _, pr := range searched.Values {
		matches[pr.ID]++
	}
	if len(searched.Values) != len(ids) {
		t.Fatalf("PR search returned %d, want %d", len(searched.Values), len(ids))
	}
	for _, id := range ids {
		if matches[id] != 1 {
			t.Fatalf("PR search missing/duplicated %d: %+v", id, searched)
		}
	}
	for i, id := range ids {
		var pr bbPR
		s.mustJSON(&pr, "pr", "view", strconv.Itoa(id), "--repo", target)
		if pr.ID != id || pr.State != "OPEN" || pr.Title != fmt.Sprintf("CLI E2E change-%d", i) {
			t.Fatalf("PR readback differs: %+v", pr)
		}
	}
	id := strconv.Itoa(ids[0])
	diff := s.mustRun("pr", "diff", id, "--repo", target)
	if !strings.Contains(diff.stdout, "+change-0") {
		t.Fatalf("PR diff lacks source change: %s", diff.stdout)
	}
	type comment struct {
		ID      int `json:"id"`
		Content struct {
			Raw string `json:"raw"`
		} `json:"content"`
	}
	comments := map[int]string{}
	for i := 0; i < 2; i++ {
		body := fmt.Sprintf("CLI E2E comment %d", i)
		var created comment
		s.mustJSON(&created, "pr", "comments", "add", id, "--repo", target, "--body", body)
		if created.ID == 0 {
			t.Fatal("comment missing ID")
		}
		comments[created.ID] = body
	}
	var page struct {
		Values []comment `json:"values"`
	}
	s.mustJSON(&page, "pr", "comments", "list", id, "--repo", target, "--all", "--limit", "1")
	if len(page.Values) != len(comments) {
		t.Fatalf("comment pagination count: got %d want %d", len(page.Values), len(comments))
	}
	for _, c := range page.Values {
		if body, ok := comments[c.ID]; ok {
			if c.Content.Raw != body {
				t.Fatalf("comment body differs: %+v", c)
			}
			delete(comments, c.ID)
		}
	}
	if len(comments) != 0 {
		t.Fatalf("comments missing: %+v", comments)
	}
	s.mustRun("pr", "decline", strconv.Itoa(ids[1]), "--repo", target)
	var declined bbPR
	s.mustJSON(&declined, "pr", "view", strconv.Itoa(ids[1]), "--repo", target)
	if declined.State != "DECLINED" {
		t.Fatalf("decline did not persist: %+v", declined)
	}
	var main bbCommit
	s.mustJSON(&main, "commit", "view", "main", "--repo", target)
	if main.Hash != base {
		t.Fatalf("decline changed main: %+v", main)
	}
	s.mustRun("pr", "merge", id, "--repo", target, "--strategy", "fast-forward")
	var merged bbPR
	s.mustJSON(&merged, "pr", "view", id, "--repo", target)
	if merged.State != "MERGED" {
		t.Fatalf("merge did not persist: %+v", merged)
	}
	s.mustJSON(&main, "commit", "view", "main", "--repo", target)
	if main.Hash != hashes[0] {
		t.Fatalf("main did not advance to source: got %s want %s", main.Hash, hashes[0])
	}
}
