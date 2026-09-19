package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aurokin/atlassian-cli/internal/config"
)

func sharedConfig(t *testing.T, p process) config.Config {
	t.Helper()
	cfg, err := config.Load(filepath.Join(p.home, "atlassian-cli", "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

func TestSharedAuthLifecycleAndTargetPrecedence(t *testing.T) {
	for _, product := range []string{"jira", "conf", "bb"} {
		t.Run(product, func(t *testing.T) {
			p := isolated(t)
			var first, second atomic.Int32
			server := func(marker string, requests *atomic.Int32) *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					requests.Add(1)
					if r.Header.Get("Authorization") != "Bearer "+dummyToken {
						t.Error("incorrect fixture authentication")
					}
					_, _ = fmt.Fprintf(w, `{"target":%q}`, marker)
				}))
			}
			a, b := server("alpha", &first), server("beta", &second)
			defer a.Close()
			defer b.Close()
			for name, url := range map[string]string{"alpha": a.URL, "beta": b.URL} {
				success(t, p.run(t, product, "auth", "login", "--site", name, "--url", url, "--token-style", "data-center-pat", "--token-env", "ATL_E2E_TOKEN", "--no-prompt"))
			}
			var statuses struct {
				Sites []struct{ Site, TokenStatus string } `json:"sites"`
			}
			r := p.run(t, product, "auth", "status", "--json")
			success(t, r)
			if err := json.Unmarshal([]byte(r.stdout), &statuses); err != nil || len(statuses.Sites) != 2 || statuses.Sites[0].Site != "alpha" || statuses.Sites[1].Site != "beta" {
				t.Fatalf("unexpected offline profiles: %s", r.stdout)
			}
			r = p.run(t, product, "auth", "status", "--site", "alpha", "--jq", ".token_status")
			success(t, r)
			if !strings.Contains(r.stdout, "environment") {
				t.Fatalf("expected environment token availability: %s", r.stdout)
			}
			if first.Load()+second.Load() != 0 {
				t.Fatal("offline auth commands reached network")
			}
			missingToken := p
			missingToken.env = append(append([]string{}, p.env...), "ATL_E2E_TOKEN=")
			failure(t, missingToken.run(t, product, "api", "/fixture", "--site", "alpha", "--json"), 1, "token_unavailable")
			if first.Load()+second.Load() != 0 {
				t.Fatal("missing credential reached network")
			}
			success(t, p.run(t, product, "auth", "default", "alpha"))
			r = p.run(t, product, "auth", "default", "--json")
			success(t, r)
			if !strings.Contains(r.stdout, `"default_site": "alpha"`) {
				t.Fatalf("default not persisted: %s", r.stdout)
			}
			assertTarget := func(proc process, want string, args ...string) {
				t.Helper()
				r := proc.run(t, product, append([]string{"api", "/fixture", "--jq", ".target"}, args...)...)
				success(t, r)
				if strings.TrimSpace(r.stdout) != fmt.Sprintf("%q", want) {
					t.Fatalf("wrong selected target: %s", r.stdout)
				}
			}
			assertTarget(p, "alpha")
			withEnv := p
			withEnv.env = append(append([]string{}, p.env...), "ATL_SITE=beta")
			assertTarget(withEnv, "beta")
			assertTarget(withEnv, "alpha", "--site", "alpha")
			if first.Load() != 2 || second.Load() != 1 {
				t.Fatalf("unexpected routing alpha=%d beta=%d", first.Load(), second.Load())
			}
			success(t, p.run(t, product, "auth", "default", "--clear"))
			failure(t, p.run(t, product, "api", "/fixture", "--json"), 8, "invalid_input")
			success(t, p.run(t, product, "auth", "default", "alpha"))
			success(t, p.run(t, product, "auth", "logout", "--site", "alpha"))
			cfg := sharedConfig(t, p)
			if len(cfg.Sites) != 1 || cfg.Sites["beta"].BaseURL != b.URL || cfg.DefaultSite != "" {
				t.Fatalf("logout removed wrong state: %+v", cfg)
			}
			failure(t, p.run(t, product, "auth", "status", "--site", "alpha", "--json"), 1, "site_not_configured")
			failure(t, p.run(t, product, "auth", "default", "alpha", "--json"), 1, "site_not_configured")
			assertTarget(p, "beta", "--site", "beta")
		})
	}
}

func TestSharedAliasLifecycleAndExpansion(t *testing.T) {
	for _, product := range []string{"jira", "conf", "bb"} {
		t.Run(product, func(t *testing.T) {
			p := isolated(t)
			success(t, p.run(t, product, "alias", "set", "identity", "version --jq '.binary + \" with spaces\"'"))
			r := p.run(t, product, "identity")
			success(t, r)
			if strings.TrimSpace(r.stdout) != fmt.Sprintf(`"atl-%s with spaces"`, product) {
				t.Fatalf("quoted expansion changed: %s", r.stdout)
			}
			success(t, p.run(t, product, "alias", "set", "identity", "version"))
			r = p.run(t, product, "identity", "--jq", ".binary")
			success(t, r)
			if strings.TrimSpace(r.stdout) != fmt.Sprintf(`"atl-%s"`, product) {
				t.Fatalf("appended arguments lost: %s", r.stdout)
			}
			r = p.run(t, product, "alias", "list", "--json")
			success(t, r)
			var aliases map[string]string
			if err := json.Unmarshal([]byte(r.stdout), &aliases); err != nil || len(aliases) != 1 || aliases["identity"] != "version" {
				t.Fatalf("wrong aliases: %s", r.stdout)
			}
			failure(t, p.run(t, product, "alias", "set", "broken", `version "unterminated`, "--json"), 8, "invalid_input")
			if _, ok := sharedConfig(t, p).Aliases["broken"]; ok {
				t.Fatal("invalid alias persisted")
			}
			success(t, p.run(t, product, "alias", "delete", "identity"))
			failure(t, p.run(t, product, "alias", "delete", "identity", "--json"), 6, "not_found_or_not_visible")
			r = p.run(t, product, "alias", "list", "--json")
			success(t, r)
			if strings.TrimSpace(r.stdout) != "{}" {
				t.Fatalf("alias survived delete: %s", r.stdout)
			}
			success(t, p.run(t, product, "alias", "set", "cycle-one", "cycle-two"))
			success(t, p.run(t, product, "alias", "set", "cycle-two", "cycle-one"))
			// Empty PATH also prevents extension fallback from finding ambient tools.
			cycle := p
			cycle.env = append(append([]string{}, p.env...), "PATH="+t.TempDir())
			r = cycle.run(t, product, "cycle-one")
			if r.exit != 1 || r.stdout != "" || !strings.Contains(r.stderr, "unknown command") {
				t.Fatalf("cycle did not terminate cleanly: %+v", r)
			}
		})
	}
}

func TestSharedResolveBrowseAndHelp(t *testing.T) {
	cases := []struct{ product, input, kind, id, key, url string }{
		{"jira", "https://fixture.invalid/browse/KAN-7", "jira_issue", "", "KAN-7", "https://fixture.invalid/browse/KAN-7"},
		{"conf", "https://fixture.invalid/wiki/spaces/TEAM/pages/123/Example", "confluence_page", "123", "TEAM", "https://fixture.invalid/wiki/spaces/TEAM/pages/123"},
		{"bb", "https://bitbucket.org/acme/fixture/pull-requests/7", "bitbucket_pull_request", "7", "acme/fixture", "https://bitbucket.org/acme/fixture/pull-requests/7"},
	}
	for _, tc := range cases {
		t.Run(tc.product, func(t *testing.T) {
			p := isolated(t)
			// A PATH with no browser launcher makes accidental open() fail instead
			// of touching the user's browser; successful JSON proves suppression.
			p.env = append(p.env, "PATH="+t.TempDir())
			r := p.run(t, tc.product, "resolve", tc.input, "--json")
			success(t, r)
			var resource struct{ Kind, ID, Key, Input string }
			if err := json.Unmarshal([]byte(r.stdout), &resource); err != nil || resource.Kind != tc.kind || resource.ID != tc.id || resource.Key != tc.key || resource.Input != tc.input {
				t.Fatalf("wrong resolved resource: %s", r.stdout)
			}
			for _, flag := range []string{"--no-browser", "--no-prompt"} {
				r = p.run(t, tc.product, "browse", tc.input, flag, "--json")
				success(t, r)
				var target struct{ URL string }
				if err := json.Unmarshal([]byte(r.stdout), &target); err != nil || target.URL != tc.url {
					t.Fatalf("wrong browser URL: %s", r.stdout)
				}
			}
			failure(t, p.run(t, tc.product, "resolve", "not a resource", "--json"), 1, "unresolved")
			bare := map[string]string{"jira": "KAN-7", "conf": "123", "bb": "acme/fixture"}[tc.product]
			failure(t, p.run(t, tc.product, "browse", bare, "--no-browser", "--json"), 8, "invalid_input")
			var requests atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				requests.Add(1)
				w.WriteHeader(http.StatusInternalServerError)
			}))
			defer srv.Close()
			p.profile(t, tc.product, srv.URL)
			path := map[string]string{"jira": "/browse/KAN-7", "conf": "/wiki/pages/viewpage.action?pageId=123", "bb": "/acme/fixture"}[tc.product]
			r = p.run(t, tc.product, "browse", bare, "--no-browser")
			success(t, r)
			if strings.TrimSpace(r.stdout) != srv.URL+path || requests.Load() != 0 {
				t.Fatalf("bare browse failed offline targeting: %s requests=%d", r.stdout, requests.Load())
			}
			r = p.run(t, tc.product, "help", "auth", "login")
			success(t, r)
			for _, flag := range []string{"--token-stdin", "--token-env", "--no-prompt"} {
				if !strings.Contains(r.stdout, flag) {
					t.Fatalf("explicit help missing %s", flag)
				}
			}
		})
	}
}

func sharedStdin(t *testing.T, p process, product, input string, protectKeychain bool, args ...string) result {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	command := binaries[product]
	if protectKeychain && runtime.GOOS == "darwin" {
		// go-keyring calls this absolute executable, so PATH isolation alone
		// would still write the real keychain. Sandbox denial forces fallback.
		args = append([]string{"-p", `(version 1) (allow default) (deny process-exec (literal "/usr/bin/security"))`, command}, args...)
		command = "/usr/bin/sandbox-exec"
	}
	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Dir = p.home
	cmd.Env = p.env
	cmd.Stdin = strings.NewReader(input)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if ctx.Err() != nil {
		t.Fatal("stdin process exceeded deadline")
	}
	code := 0
	if err != nil {
		if exit, ok := err.(*exec.ExitError); ok {
			code = exit.ExitCode()
		} else {
			t.Fatal(err)
		}
	}
	r := result{stdout.String(), stderr.String(), code}
	if strings.Contains(r.stdout+r.stderr, dummyToken) {
		t.Fatal("stdin credential leaked")
	}
	return r
}

func TestSharedLoginStdinValidation(t *testing.T) {
	for _, product := range []string{"jira", "conf", "bb"} {
		t.Run(product, func(t *testing.T) {
			p := isolated(t)
			args := []string{"auth", "login", "--site", "fixture", "--url", "http://127.0.0.1:1", "--token-style", "data-center-pat", "--token-stdin", "--json", "--no-prompt"}
			failure(t, sharedStdin(t, p, product, " \n", false, args...), 8, "invalid_input")
			failure(t, sharedStdin(t, p, product, dummyToken, false, append(args, "--token-env", "ATL_E2E_TOKEN")...), 8, "invalid_input")
			if len(sharedConfig(t, p).Sites) != 0 {
				t.Fatal("invalid stdin login persisted profile")
			}
		})
	}
}

func TestSharedLoginStdinFileFallback(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("no isolated native keychain denial mechanism on this platform")
	}
	for _, product := range []string{"jira", "conf", "bb"} {
		t.Run(product, func(t *testing.T) {
			p := isolated(t)
			p.env = append(p.env, "DBUS_SESSION_BUS_ADDRESS=unix:path="+filepath.Join(p.home, "absent-dbus"))
			var requests atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				if r.Header.Get("Authorization") != "Bearer "+dummyToken {
					t.Error("stored stdin token not used")
				}
				_, _ = w.Write([]byte(`{"ok":true}`))
			}))
			defer srv.Close()
			r := sharedStdin(t, p, product, " \n"+dummyToken+"\n", true, "auth", "login", "--site", "stdin-fixture", "--url", srv.URL, "--token-style", "data-center-pat", "--token-stdin", "--no-prompt")
			if r.exit != 0 || !strings.Contains(r.stderr, "Warning:") {
				t.Fatalf("expected explicit fallback warning: %+v", r)
			}
			cfg := sharedConfig(t, p)
			if cfg.Sites["stdin-fixture"].TokenRef != "file" {
				t.Fatal("stdin credential did not use isolated file backend")
			}
			path := filepath.Join(p.home, "atlassian-cli", "credentials.json")
			info, err := os.Stat(path)
			if err != nil || info.Mode().Perm() != 0600 {
				t.Fatalf("credential file protection: %v", err)
			}
			// Make the successful request in a separate process with no token env.
			var withoutToken []string
			for _, entry := range p.env {
				if !strings.HasPrefix(entry, "ATL_E2E_TOKEN=") {
					withoutToken = append(withoutToken, entry)
				}
			}
			p.env = withoutToken
			r = p.run(t, product, "api", "/fixture", "--site", "stdin-fixture", "--json")
			success(t, r)
			if requests.Load() != 1 {
				t.Fatal("stored credential request missing")
			}
			r = p.run(t, product, "auth", "status", "--site", "stdin-fixture", "--jq", ".token_status")
			success(t, r)
			if !strings.Contains(r.stdout, "local credentials file") {
				t.Fatalf("wrong token status: %s", r.stdout)
			}
			success(t, p.run(t, product, "auth", "logout", "--site", "stdin-fixture"))
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if bytes.Contains(data, []byte(dummyToken)) {
				t.Fatal("logout retained stored token")
			}
			if len(sharedConfig(t, p).Sites) != 0 {
				t.Fatal("logout retained stdin profile")
			}
		})
	}
}
