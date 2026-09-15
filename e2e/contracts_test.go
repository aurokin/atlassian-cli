// Package e2e exercises the shipped entry points in isolated subprocesses.
// Local HTTP fixtures prove process contracts, not Atlassian API compatibility.
package e2e

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aurokin/atlassian-cli/internal/config"
)

var binaries map[string]string

const dummyToken = "e2e-only-never-a-real-credential"

func TestMain(m *testing.M) {
	os.Exit(runTests(m))
}

func runTests(m *testing.M) int {
	dir, err := os.MkdirTemp("", "atl-contract-binaries-")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer os.RemoveAll(dir)
	binaries = make(map[string]string)
	for _, product := range []string{"jira", "conf", "bb"} {
		name := "atl-" + product
		path := filepath.Join(dir, name)
		if runtime.GOOS == "windows" {
			path += ".exe"
		}
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		cmd := exec.CommandContext(ctx, "go", "build", "-o", path, "./cmd/"+name)
		cmd.Dir = ".."
		out, err := cmd.CombinedOutput()
		cancel()
		if err != nil {
			fmt.Fprintf(os.Stderr, "build %s: %v\n%s", name, err, out)
			return 1
		}
		binaries[product] = path
	}
	return m.Run()
}

type process struct {
	home string
	env  []string
}
type result struct {
	stdout, stderr string
	exit           int
}

func isolated(t *testing.T) process {
	t.Helper()
	home := t.TempDir()
	// Whitelist platform plumbing; no ambient ATL_*, proxies, Git config,
	// credential variables, or user config may influence a fixture process.
	env := []string{"HOME=" + home, "USERPROFILE=" + home, "XDG_CONFIG_HOME=" + home,
		"ATL_E2E_TOKEN=" + dummyToken, "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=" + os.DevNull}
	for _, key := range []string{"PATH", "SystemRoot", "WINDIR", "TEMP", "TMP", "TMPDIR"} {
		if value, ok := os.LookupEnv(key); ok {
			env = append(env, key+"="+value)
		}
	}
	return process{home, env}
}

func (p process) run(t *testing.T, product string, args ...string) result {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, binaries[product], args...)
	cmd.Dir, cmd.Env = p.home, p.env
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	if ctx.Err() != nil {
		t.Fatalf("%s %v exceeded process deadline", product, args)
	}
	code := 0
	if err != nil {
		if e, ok := err.(*exec.ExitError); ok {
			code = e.ExitCode()
		} else {
			t.Fatal(err)
		}
	}
	r := result{stdout.String(), stderr.String(), code}
	for _, secret := range []string{dummyToken, base64.StdEncoding.EncodeToString([]byte("fixture@example.com:" + dummyToken))} {
		if strings.Contains(r.stdout+r.stderr, secret) {
			t.Fatal("credential leaked to process output")
		}
	}
	return r
}

func (p process) profile(t *testing.T, product, url string) {
	t.Helper()
	productName := map[string]string{"jira": "jira", "conf": "confluence", "bb": "bitbucket"}[product]
	cfg := config.New()
	cfg.DefaultSite = "fixture"
	cfg.Sites["fixture"] = config.SiteProfile{Product: productName, Deployment: "data-center", BaseURL: url,
		TokenStyle: "data-center-pat", AuthType: "pat-bearer", TokenRef: "env:ATL_E2E_TOKEN"}
	if err := config.Save(filepath.Join(p.home, "atlassian-cli", "config.json"), cfg); err != nil {
		t.Fatal(err)
	}
}

func success(t *testing.T, r result) {
	t.Helper()
	if r.exit != 0 || r.stderr != "" {
		t.Fatalf("exit=%d stdout=%s stderr=%s", r.exit, r.stdout, r.stderr)
	}
}

func failure(t *testing.T, r result, exit int, code string) {
	t.Helper()
	if r.exit != exit || r.stdout != "" {
		t.Fatalf("exit=%d want=%d stdout=%s stderr=%s", r.exit, exit, r.stdout, r.stderr)
	}
	var envelope struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal([]byte(r.stderr), &envelope); err != nil || envelope.Error != code {
		t.Fatalf("want structured %s, got stderr=%s", code, r.stderr)
	}
}

func TestDiscovery(t *testing.T) {
	for _, product := range []string{"jira", "conf", "bb"} {
		t.Run(product, func(t *testing.T) {
			p := isolated(t)
			r := p.run(t, product, "version", "--json")
			success(t, r)
			var version struct {
				Binary string `json:"binary"`
			}
			if err := json.Unmarshal([]byte(r.stdout), &version); err != nil || version.Binary != "atl-"+product {
				t.Fatalf("wrong identity: %s", r.stdout)
			}
			r = p.run(t, product, "--help")
			success(t, r)
			if !strings.Contains(r.stdout, "--no-prompt") {
				t.Fatal("help lacks agent flag")
			}
			r = p.run(t, product, "completion", "bash")
			success(t, r)
			if !strings.Contains(r.stdout, "__start_atl-") {
				t.Fatal("missing Bash completion entry point")
			}
			r = p.run(t, product, "version", "--e2e-unknown-flag")
			if r.exit != 1 || r.stdout != "" || !strings.Contains(r.stderr, "unknown flag") {
				t.Fatalf("unknown flag: %+v", r)
			}
		})
	}
}

func TestHTTPErrorExits(t *testing.T) {
	cases := []struct {
		status, exit int
		code         string
	}{{400, 1, "http_error"}, {401, 4, "unauthorized"}, {403, 5, "forbidden"}, {404, 6, "not_found_or_not_visible"}, {410, 1, "gone"}, {429, 7, "rate_limited"}, {500, 1, "http_error"}}
	for _, product := range []string{"jira", "conf", "bb"} {
		t.Run(product, func(t *testing.T) {
			for _, tc := range cases {
				t.Run(strconv.Itoa(tc.status), func(t *testing.T) {
					srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						if r.Header.Get("Authorization") != "Bearer "+dummyToken {
							t.Error("request missing expected fixture authentication")
						}
						w.WriteHeader(tc.status)
						_, _ = w.Write([]byte(`{"message":"fixture failure"}`))
					}))
					defer srv.Close()
					p := isolated(t)
					p.profile(t, product, srv.URL)
					failure(t, p.run(t, product, "status", "--json"), tc.exit, tc.code)
				})
			}
		})
	}
}

func TestTimeoutAndTrace(t *testing.T) {
	for _, product := range []string{"jira", "conf", "bb"} {
		t.Run(product, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/slow" {
					<-r.Context().Done()
					return
				}
				_, _ = w.Write([]byte(`{"id":"fixture"}`))
			}))
			defer srv.Close()
			p := isolated(t)
			p.profile(t, product, srv.URL)
			failure(t, p.run(t, product, "api", "/slow", "--timeout", "30ms", "--json"), 9, "timeout")
			r := p.run(t, product, "api", "/fixture", "--trace", "--json")
			if r.exit != 0 || !json.Valid([]byte(r.stdout)) || !strings.Contains(r.stderr, "[trace] > GET") || !strings.Contains(r.stderr, "[trace] < 200") {
				t.Fatalf("trace contract: %+v", r)
			}
		})
	}
}

func TestValidationBeforeAuthentication(t *testing.T) {
	cases := []struct {
		product string
		args    []string
	}{
		{"jira", []string{"issue", "comment", "delete", "KAN-1", "10"}},
		{"conf", []string{"page", "delete", "10", "--purge"}},
		{"conf", []string{"page", "comment", "delete", "10"}},
		{"bb", []string{"repo", "delete", "acme/fixture"}},
		{"jira", []string{"issue", "create"}},
	}
	for _, tc := range cases {
		t.Run(tc.product+"/"+strings.Join(tc.args, "_"), func(t *testing.T) {
			p := isolated(t)
			args := append(append([]string{}, tc.args...), "--json", "--no-prompt")
			failure(t, p.run(t, tc.product, args...), 8, "invalid_input")
			var requests atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { requests.Add(1); w.WriteHeader(500) }))
			defer srv.Close()
			p.profile(t, tc.product, srv.URL)
			failure(t, p.run(t, tc.product, args...), 8, "invalid_input")
			if requests.Load() != 0 {
				t.Fatal("invalid input reached network")
			}
		})
	}
}

func TestPullRequestPagination(t *testing.T) {
	for _, args := range [][]string{{"pr", "list"}, {"search", "prs", `title ~ "fixture"`}} {
		t.Run(strings.Join(args[:2], "_"), func(t *testing.T) {
			var requests atomic.Int32
			var serverURL string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				if r.URL.Path != "/repositories/acme/fixture/pullrequests" {
					t.Errorf("unexpected path %s", r.URL.Path)
				}
				if r.URL.Query().Get("page") == "2" {
					_, _ = w.Write([]byte(`{"values":[{"id":2,"title":"second"}]}`))
					return
				}
				if r.URL.Query().Get("pagelen") != "50" {
					t.Errorf("default PR page size=%s, want 50", r.URL.RawQuery)
				}
				if args[0] == "search" && r.URL.Query().Get("q") != args[2] {
					t.Errorf("search query lost: %s", r.URL.RawQuery)
				}
				_, _ = fmt.Fprintf(w, `{"values":[{"id":1,"title":"first"}],"next":%q}`, serverURL+r.URL.Path+"?page=2")
			}))
			defer srv.Close()
			serverURL = srv.URL
			p := isolated(t)
			p.profile(t, "bb", srv.URL)
			command := append(append([]string{}, args...), "--repo", "acme/fixture", "--all", "--jq", ".values | map(.id)")
			r := p.run(t, "bb", command...)
			success(t, r)
			var ids []int
			if err := json.Unmarshal([]byte(r.stdout), &ids); err != nil || len(ids) != 2 || ids[0] != 1 || ids[1] != 2 || requests.Load() != 2 {
				t.Fatalf("incomplete/duplicate pagination: %s requests=%d", r.stdout, requests.Load())
			}
		})
	}
}

func TestConfluenceDirectChildren(t *testing.T) {
	var requests atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if r.URL.Path != "/pages/10/direct-children" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"results":[{"id":"11","title":"Child A","type":"page","status":"current"},{"id":"12","title":"Folder B","type":"folder","status":"current"}]}`))
	}))
	defer srv.Close()
	p := isolated(t)
	p.profile(t, "conf", srv.URL)
	r := p.run(t, "conf", "page", "children", "10")
	success(t, r)
	for _, value := range []string{"11", "Child A", "page", "12", "Folder B", "folder"} {
		if !strings.Contains(r.stdout, value) {
			t.Fatalf("missing %q: %s", value, r.stdout)
		}
	}
	r = p.run(t, "conf", "page", "children", "10", "--jq", ".results | map(.type)")
	success(t, r)
	if strings.TrimSpace(r.stdout) != `["page","folder"]` {
		t.Fatalf("child types lost: %s", r.stdout)
	}
	if requests.Load() != 2 {
		t.Fatalf("requests=%d", requests.Load())
	}
}

func TestSynthesizedMutationResults(t *testing.T) {
	cases := []struct {
		name, path, want string
		args             []string
	}{
		{"label", "/rest/api/content/10/label/fixture", `{"page":"10","label":"fixture","removed":true}`, []string{"page", "label", "remove", "10", "fixture"}},
		{"comment", "/footer-comments/20", `{"id":"20","deleted":true}`, []string{"page", "comment", "delete", "20", "--yes"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var requests atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				if r.Method != http.MethodDelete || r.URL.Path != tc.path {
					t.Errorf("got %s %s, want DELETE %s", r.Method, r.URL.Path, tc.path)
				}
				w.WriteHeader(http.StatusNoContent)
			}))
			defer srv.Close()
			p := isolated(t)
			p.profile(t, "conf", srv.URL)
			r := p.run(t, "conf", append(append([]string{}, tc.args...), "--json")...)
			success(t, r)
			var got, want map[string]json.RawMessage
			if err := json.Unmarshal([]byte(r.stdout), &got); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal([]byte(tc.want), &want); err != nil {
				t.Fatal(err)
			}
			if len(got) != len(want) {
				t.Fatalf("unexpected result %s", r.stdout)
			}
			for key, value := range want {
				if string(got[key]) != string(value) {
					t.Fatalf("unexpected result %s", r.stdout)
				}
			}
			if requests.Load() != 1 {
				t.Fatalf("requests=%d", requests.Load())
			}
		})
	}
}

func TestBitbucketGitInference(t *testing.T) {
	for _, remote := range []string{"git@ssh.bitbucket.org:acme/fixture.git", "ssh://git@ssh.bitbucket.org:443/acme/fixture.git", "git@bitbucket.org:acme/fixture.git", "ssh://git@altssh.bitbucket.org:443/acme/fixture.git"} {
		t.Run(remote, func(t *testing.T) {
			p := isolated(t)
			for _, args := range [][]string{{"init", "--quiet"}, {"remote", "add", "origin", remote}} {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				cmd := exec.CommandContext(ctx, "git", args...)
				cmd.Dir = p.home
				cmd.Env = p.env
				out, err := cmd.CombinedOutput()
				cancel()
				if err != nil {
					t.Fatalf("git setup: %v %s", err, out)
				}
			}
			var requests atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				if r.URL.Path != "/repositories/acme/fixture" {
					t.Errorf("wrong inferred target %s", r.URL.Path)
				}
				_, _ = w.Write([]byte(`{"full_name":"acme/fixture"}`))
			}))
			defer srv.Close()
			p.profile(t, "bb", srv.URL)
			r := p.run(t, "bb", "repo", "view", "--jq", ".full_name")
			success(t, r)
			if strings.TrimSpace(r.stdout) != `"acme/fixture"` || requests.Load() != 1 {
				t.Fatalf("inference: %s requests=%d", r.stdout, requests.Load())
			}
		})
	}
}

func TestEnvironmentTokenLoginPersistsOnlyReference(t *testing.T) {
	for _, product := range []string{"jira", "conf", "bb"} {
		t.Run(product, func(t *testing.T) {
			p := isolated(t)
			var requests atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				if r.Header.Get("Authorization") != "Bearer "+dummyToken {
					t.Error("wrong auth")
				}
				_, _ = w.Write([]byte(`{"id":"fixture"}`))
			}))
			defer srv.Close()
			r := p.run(t, product, "auth", "login", "--site", "fixture", "--url", srv.URL,
				"--token-style", "data-center-pat", "--token-env", "ATL_E2E_TOKEN", "--no-prompt")
			success(t, r)
			data, err := os.ReadFile(filepath.Join(p.home, "atlassian-cli", "config.json"))
			if err != nil {
				t.Fatal(err)
			}
			if bytes.Contains(data, []byte(dummyToken)) || !bytes.Contains(data, []byte("env:ATL_E2E_TOKEN")) {
				t.Fatal("login failed to store only credential reference")
			}
			if _, err := os.Stat(filepath.Join(p.home, "atlassian-cli", "credentials.json")); !os.IsNotExist(err) {
				t.Fatal("env login created a credential file")
			}
			r = p.run(t, product, "api", "/fixture", "--site", "fixture", "--json")
			success(t, r)
			if requests.Load() != 1 {
				t.Fatalf("expected one API request, got %d", requests.Load())
			}
		})
	}
}

func TestRawAPIErrorBodyAndOriginBoundary(t *testing.T) {
	for _, product := range []string{"jira", "conf", "bb"} {
		t.Run(product, func(t *testing.T) {
			p := isolated(t)
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusForbidden)
				_, _ = w.Write([]byte(`{"message":"fixture denial"}`))
			}))
			defer srv.Close()
			p.profile(t, product, srv.URL)
			r := p.run(t, product, "api", "/fixture", "--json")
			var body struct {
				Message string `json:"message"`
			}
			if err := json.Unmarshal([]byte(r.stdout), &body); err != nil || body.Message != "fixture denial" {
				t.Fatalf("raw api lost upstream error body: %s", r.stdout)
			}
			failure(t, result{stderr: r.stderr, exit: r.exit}, 5, "forbidden")
			var leakedRequests atomic.Int32
			other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { leakedRequests.Add(1); w.WriteHeader(204) }))
			defer other.Close()
			failure(t, p.run(t, product, "api", other.URL+"/credential-target", "--json"), 1, "untrusted_url")
			if leakedRequests.Load() != 0 {
				t.Fatal("cross-origin request escaped configured site")
			}
		})
	}
}
