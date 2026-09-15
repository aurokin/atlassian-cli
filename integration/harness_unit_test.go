//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/aurokin/atlassian-cli/internal/config"
)

func TestHarnessStoredEnvironmentCredential(t *testing.T) {
	t.Setenv("ATL_API_TOKEN", "dummy-harness-token")
	t.Setenv("ATL_SITE", "unrelated-profile")
	t.Setenv("ATL_TIMEOUT", "invalid-ambient-timeout")
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("ATL_IT_JIRA_EXPECTED_ACCOUNT_ID", "harness-account")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		username, token, ok := r.BasicAuth()
		if !ok || username != "fixture@example.invalid" || token != "dummy-harness-token" {
			t.Error("stored environment credential did not reach the server")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"accountId":"harness-account","displayName":"Fixture"}`))
	}))
	defer srv.Close()
	cfg := config.New()
	cfg.Sites["fixture"] = config.SiteProfile{
		Product: "jira", Deployment: "cloud", BaseURL: srv.URL,
		APIBaseURL: srv.URL + "/rest/api/3", Username: "fixture@example.invalid",
		TokenStyle: "cloud-classic", AuthType: "basic", TokenRef: "env:ATL_API_TOKEN",
	}
	path, err := config.DefaultPath()
	if err != nil {
		t.Fatal(err)
	}
	if err := config.Save(path, cfg); err != nil {
		t.Fatal(err)
	}
	s := &session{t: t, binaryPath: buildBinary(t, "atl-jira"), site: "fixture"}
	s.preflight(jiraProduct)
	if s.target["account_id"] != "harness-account" {
		t.Fatal("stored-profile preflight did not verify the fixture identity")
	}
}

// Expected test failures run in a child test process so they cannot turn the
// parent suite red or access configured live credentials.
func TestHarnessStrictFailures(t *testing.T) {
	for _, tc := range []struct{ mode, message string }{
		{"missing-profile", "set ATL_IT_JIRA_SITE"},
		{"wrong-product", "missing or wrong product"},
		{"scope-denial", "selected capability requires missing scope/permission"},
		{"cleanup-failure", "cleanup incomplete"},
	} {
		t.Run(tc.mode, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			ledger := filepath.Join(t.TempDir(), "ledger.jsonl")
			cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestHarnessChild$")
			cmd.Env = append(os.Environ(), "ATL_HARNESS_CHILD="+tc.mode, "XDG_CONFIG_HOME="+t.TempDir(), "ATL_RUN_INTEGRATION=1", "ATL_IT_USE_STORED_PROFILES=1", "ATL_IT_JIRA_SITE=", "CI=", "ATL_HARNESS_LEDGER="+ledger)
			boundProcess(cmd)
			out, err := cmd.CombinedOutput()
			if err == nil || !strings.Contains(string(out), tc.message) {
				t.Fatalf("wanted failure %q: %v\n%s", tc.message, err, out)
			}
			if tc.mode == "cleanup-failure" {
				data, err := os.ReadFile(ledger)
				if err != nil {
					t.Fatal(err)
				}
				if !strings.Contains(string(data), `"pending"`) || strings.Contains(string(data), `"deleted"`) {
					t.Fatalf("failed cleanup lost pending recovery record: %s", data)
				}
			}
		})
	}
}

func TestHarnessChild(t *testing.T) {
	switch os.Getenv("ATL_HARNESS_CHILD") {
	case "missing-profile":
		newSession(t, jiraProduct)
	case "wrong-product":
		dir := filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "atlassian-cli")
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(`{"version":1,"sites":{"wrong":{"product":"bitbucket"}}}`), 0600); err != nil {
			t.Fatal(err)
		}
		(&session{t: t, site: "wrong"}).preflight(jiraProduct)
	case "scope-denial":
		(&session{t: t}).failIfScopeOrPermission(cmdResult{err: errors.New("exit 5"), stderr: "forbidden"}, "fixture write")
	case "cleanup-failure":
		cleanupLedger = os.Getenv("ATL_HARNESS_LEDGER")
		if err := os.WriteFile(cleanupLedger, nil, 0600); err != nil {
			t.Fatal(err)
		}
		(&session{t: t, site: "fixture", binaryPath: "atl-jira"}).cleanupOwned("jira-issue", "OWNED-1", func() bool { return false })
	case "sleep":
		time.Sleep(time.Minute)
	}
}

func TestHarnessProcessDeadline(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestHarnessChild$")
	cmd.Env = append(os.Environ(), "ATL_HARNESS_CHILD=sleep")
	boundProcess(cmd)
	cmd.WaitDelay = time.Second
	start := time.Now()
	err := cmd.Run()
	if err == nil || ctx.Err() != context.DeadlineExceeded {
		t.Fatalf("expected deadline termination: %v / %v", err, ctx.Err())
	}
	if elapsed := time.Since(start); elapsed > 3*time.Second {
		t.Fatalf("deadline was not bounded: %v", elapsed)
	}
}

func TestHarnessCleanupLedger(t *testing.T) {
	previous := cleanupLedger
	cleanupLedger = filepath.Join(t.TempDir(), "ledger.jsonl")
	defer func() { cleanupLedger = previous }()
	if err := os.WriteFile(cleanupLedger, nil, 0600); err != nil {
		t.Fatal(err)
	}
	called := false
	t.Run("owned", func(t *testing.T) {
		s := &session{t: t, site: "fixture", binaryPath: "atl-jira", target: map[string]string{"base_url": "https://example.invalid", "cloud_id": "cloud"}}
		s.cleanupOwned("jira-issue", "OWNED-1", func() bool { called = true; return true })
		data, err := os.ReadFile(cleanupLedger)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(data), `"deleted"`) {
			t.Fatal("marked deleted before cleanup")
		}
	})
	if !called {
		t.Fatal("cleanup did not run")
	}
	data, err := os.ReadFile(cleanupLedger)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 {
		t.Fatalf("ledger entries: %d", len(lines))
	}
	for i, want := range []string{"pending", "deleted"} {
		var row struct {
			State, ID, Site string
			Target          map[string]string
		}
		if err := json.Unmarshal([]byte(lines[i]), &row); err != nil {
			t.Fatal(err)
		}
		if row.State != want || row.ID != "OWNED-1" || row.Site != "fixture" || row.Target["cloud_id"] != "cloud" {
			t.Fatalf("incorrect recovery record: %+v", row)
		}
	}
}

func TestHarnessSourceDigest(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "untracked.go")
	if err := os.WriteFile(path, []byte("package fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	names := []byte("untracked.go\x00credentials.json\x00.tmp-reauth.go\x00")
	before, err := goSourceDigest(dir, names)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("package changed"), 0600); err != nil {
		t.Fatal(err)
	}
	after, err := goSourceDigest(dir, names)
	if err != nil {
		t.Fatal(err)
	}
	if before == after {
		t.Fatal("untracked source edit did not change digest")
	}
	// Local non-source files must not influence the reported Go source identity.
	for _, name := range []string{"credentials.json", ".tmp-reauth.go"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("excluded local fixture"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	excluded, err := goSourceDigest(dir, names)
	if err != nil || excluded != after {
		t.Fatalf("excluded local files affected digest: %v", err)
	}
	onlySource, err := goSourceDigest(dir, []byte("untracked.go\x00"))
	if err != nil || onlySource != after {
		t.Fatalf("excluded files affected digest: %v", err)
	}
}
