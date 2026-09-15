//go:build integration

// Package integration holds the live, end-to-end test suite for the atl-*
// CLIs. Unlike the hermetic unit tests under internal/ (which talk only to a
// local httptest.Server), these tests drive the real built binaries against a
// real Atlassian tenant, exactly as a user would. They are the project's
// highest-confidence check that authentication, gateway routing, request
// shaping, and output rendering all actually work together end to end.
//
// They are MANUAL-ONLY. The //go:build integration tag keeps them out of
// `go test ./...` and CI; on top of that every test skips unless
// ATL_RUN_INTEGRATION=1 is set, and skips outright when CI is set. Run them
// deliberately, against your own throwaway/sandbox tenant:
//
//	ATL_RUN_INTEGRATION=1 make integration
//	ATL_RUN_INTEGRATION=1 go test -tags=integration ./integration -run Jira -v
//
// # Authentication
//
// By default the suite authenticates with credentials supplied through
// environment variables: it runs `auth login --token-env` into a private
// throwaway config directory, so nothing is stored to your keychain and no
// token ever touches disk. Set ATL_IT_USE_STORED_PROFILES=1 to instead reuse a
// site profile you have already configured (handy for oauth-3lo, whose tokens
// cannot be supplied through an environment variable).
//
// See docs/integration-testing.md for the full environment-variable contract.
package integration

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/aurokin/atlassian-cli/internal/config"
)

// requireIntegration gates every test in this package. Tests run only when the
// developer has explicitly opted in, and never under CI.
func requireIntegration(t *testing.T) {
	t.Helper()
	if os.Getenv("ATL_RUN_INTEGRATION") != "1" {
		t.Skip("set ATL_RUN_INTEGRATION=1 to run the live integration suite")
	}
	if os.Getenv("CI") != "" {
		t.Skip("integration tests are manual-only and never run under CI")
	}
}

// useStoredProfiles reports whether the suite should reuse already-configured
// site profiles instead of synthesizing one from environment-variable
// credentials. Stored mode is required for oauth-3lo sites.
func useStoredProfiles() bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv("ATL_IT_USE_STORED_PROFILES")))
	return v == "1" || v == "true" || v == "yes"
}

// product describes one CLI binary and the environment-variable prefix that
// configures it (for example "JIRA" → ATL_IT_JIRA_*).
type product struct {
	// binary is the command/package name under ./cmd, e.g. "atl-jira".
	binary string
	// envPrefix is the per-product environment-variable infix, e.g. "JIRA".
	envPrefix string
	// tokenStyle is the default token style used in env-credential mode.
	tokenStyle string
	// needsUsername is true when the static token style requires --username.
	needsUsername bool
	// supportsScoped is true for products that can route a scoped API token
	// through the api.atlassian.com gateway via the cloud-scoped style. When the
	// matching ATL_IT_<P>_CLOUD_ID is set, env-credential mode logs in with
	// cloud-scoped instead of tokenStyle. (Atlassian scoped API tokens for Jira
	// and Confluence MUST use the gateway; they do not authenticate against the
	// site URL. Bitbucket scoped tokens use plain cloud-classic Basic auth.)
	supportsScoped bool
}

var (
	jiraProduct = product{binary: "atl-jira", envPrefix: "JIRA", tokenStyle: "cloud-classic", needsUsername: true, supportsScoped: true}
	confProduct = product{binary: "atl-conf", envPrefix: "CONF", tokenStyle: "cloud-classic", needsUsername: true, supportsScoped: true}
	bbProduct   = product{binary: "atl-bb", envPrefix: "BB", tokenStyle: "cloud-classic", needsUsername: true}
)

func (p product) env(suffix string) string {
	return os.Getenv("ATL_IT_" + p.envPrefix + "_" + suffix)
}

// session is a ready-to-use, authenticated test context for one product: a
// built binary plus the config directory and site name to target it with.
type session struct {
	t          *testing.T
	binaryPath string
	// configHome is the value to export as XDG_CONFIG_HOME for child commands.
	// Empty means "use the real config dir" (stored-profile mode).
	configHome string
	site       string
	target     map[string]string
}

// newSession builds (once) the product's binary, then either reuses a stored
// profile or provisions a throwaway one from environment-variable credentials.
// Missing configuration fails selected tests; use -run to
// select a subset of products explicitly.
func newSession(t *testing.T, p product) *session {
	t.Helper()
	requireIntegration(t)

	bin := buildBinary(t, p.binary)

	if useStoredProfiles() {
		site := p.env("SITE")
		if site == "" {
			t.Fatalf("set ATL_IT_%s_SITE to run %s integration tests against a stored profile", p.envPrefix, p.binary)
		}
		s := &session{t: t, binaryPath: bin, configHome: "", site: site}
		s.preflight(p)
		return s
	}

	baseURL := p.env("BASE_URL")
	if baseURL == "" && p.envPrefix == "BB" {
		baseURL = "https://api.bitbucket.org/2.0"
	}
	token := p.env("TOKEN")
	username := p.env("USERNAME")
	if username == "" {
		username = p.env("EMAIL")
	}
	if baseURL == "" || token == "" || (p.needsUsername && username == "") {
		t.Fatalf("set ATL_IT_%s_BASE_URL, ATL_IT_%s_TOKEN%s (or ATL_IT_USE_STORED_PROFILES=1) to run %s integration tests",
			p.envPrefix, p.envPrefix,
			map[bool]string{true: " and ATL_IT_" + p.envPrefix + "_USERNAME/EMAIL", false: ""}[p.needsUsername],
			p.binary)
	}

	configHome := t.TempDir()
	tokenVar := "ATL_IT_" + p.envPrefix + "_TOKEN"
	const site = "integration"

	// A scoped API token routes through the api.atlassian.com gateway, which
	// the CLI selects via the cloud-scoped style plus a cloud_id. Fall back to
	// the default style (cloud-classic) for legacy unscoped tokens.
	style := p.tokenStyle
	cloudID := p.env("CLOUD_ID")
	if p.supportsScoped && cloudID != "" {
		style = "cloud-scoped"
	}

	args := []string{
		"auth", "login",
		"--site", site,
		"--url", baseURL,
		"--token-style", style,
		"--token-env", tokenVar,
	}
	if p.needsUsername {
		args = append(args, "--username", username)
	}
	if style == "cloud-scoped" {
		args = append(args, "--cloud-id", cloudID)
	}

	s := &session{t: t, binaryPath: bin, configHome: configHome, site: site}
	res := s.run(args...)
	if res.err != nil {
		t.Fatalf("auth login (%s) failed: %v\nstdout:\n%s\nstderr:\n%s", p.binary, res.err, res.stdout, res.stderr)
	}
	s.preflight(p)
	return s
}

// preflight checks the exact profile and positive account identity before any
// fixture writes. Only nonsecret targeting metadata enters the recovery ledger.
func (s *session) preflight(p product) {
	s.t.Helper()
	path, err := config.DefaultPath()
	if s.configHome != "" {
		path = filepath.Join(s.configHome, "atlassian-cli", "config.json")
	}
	if err != nil {
		s.t.Fatal(err)
	}
	cfg, err := config.Load(path)
	if err != nil {
		s.t.Fatal(err)
	}
	profile, ok := cfg.Sites[s.site]
	if !ok || profile.Product != map[string]string{"JIRA": "jira", "CONF": "confluence", "BB": "bitbucket"}[p.envPrefix] {
		s.t.Fatalf("profile %q missing or wrong product", s.site)
	}
	var identity struct {
		AccountID string `json:"accountId"`
		UUID      string `json:"uuid"`
	}
	s.mustJSON(&identity, "status")
	account := identity.AccountID
	if account == "" {
		account = identity.UUID
	}
	if account == "" {
		s.t.Fatal("preflight status omitted account identity")
	}
	if expected := p.env("EXPECTED_ACCOUNT_ID"); expected != "" && account != expected {
		s.t.Fatalf("preflight identity %q, expected %q", account, expected)
	}
	s.target = map[string]string{"config_path": path, "base_url": profile.BaseURL, "api_base_url": profile.APIBaseURL, "cloud_id": profile.CloudID, "token_style": profile.TokenStyle, "account_id": account}
	s.t.Logf("preflight product=%s site=%s base_url=%s token_style=%s account=%s", profile.Product, s.site, profile.BaseURL, profile.TokenStyle, account)
}

// cmdResult captures one command invocation.
type cmdResult struct {
	stdout string
	stderr string
	err    error // non-nil when the command exited non-zero
}

// run executes the product binary with args, returning captured output and any
// exit error. The configured site is appended automatically; the throwaway
// config dir (when in env-credential mode) is exported as XDG_CONFIG_HOME.
func (s *session) run(args ...string) cmdResult {
	s.t.Helper()
	full := append([]string{}, args...)
	full = append(full, "--site", s.site, "--no-prompt")

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, s.binaryPath, full...)
	boundProcess(cmd)
	cmd.WaitDelay = 2 * time.Second
	for _, entry := range os.Environ() {
		name, _, _ := strings.Cut(entry, "=")
		// Isolate CLI selectors, but preserve arbitrary --token-env references
		// such as ATL_API_TOKEN used by existing stored profiles.
		if name == "ATL_SITE" || name == "ATL_TIMEOUT" {
			continue
		}
		if name == "XDG_CONFIG_HOME" && s.configHome != "" {
			continue
		}
		cmd.Env = append(cmd.Env, entry)
	}
	if s.configHome != "" {
		cmd.Env = append(cmd.Env, "XDG_CONFIG_HOME="+s.configHome)
	}
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return cmdResult{stdout: stdout.String(), stderr: stderr.String(), err: err}
}

// mustRun runs a command and fails the test if it exits non-zero.
func (s *session) mustRun(args ...string) cmdResult {
	s.t.Helper()
	res := s.run(args...)
	if res.err != nil {
		s.t.Fatalf("atl %v failed: %v\nstdout:\n%s\nstderr:\n%s", args, res.err, res.stdout, res.stderr)
	}
	return res
}

// mustWrite requires the selected mutation to succeed, including permissions.
func (s *session) mustWrite(op string, args ...string) cmdResult {
	s.t.Helper()
	res := s.run(args...)
	s.failIfScopeOrPermission(res, op)
	if res.err != nil {
		s.t.Fatalf("%s failed: %v\nstdout:\n%s\nstderr:\n%s", op, res.err, res.stdout, res.stderr)
	}
	return res
}

// mustJSON runs a command with --json and decodes stdout into v.
func (s *session) mustJSON(v any, args ...string) {
	s.t.Helper()
	res := s.mustRun(append(args, "--json")...)
	if err := json.Unmarshal([]byte(res.stdout), v); err != nil {
		s.t.Fatalf("decode JSON from %v: %v\nstdout:\n%s", args, err, res.stdout)
	}
}

// failIfScopeOrPermission reports missing permissions as a selected capability failure.
func (s *session) failIfScopeOrPermission(res cmdResult, op string) {
	s.t.Helper()
	if res.err == nil {
		return
	}
	msg := res.stdout + res.stderr
	for _, marker := range []string{
		"scope does not match",
		"forbidden",
		"unauthorized",
		"OAUTH_SCOPE",
		"insufficient",
		"do not have permission",
		"not permitted",
	} {
		if strings.Contains(strings.ToLower(msg), strings.ToLower(marker)) {
			s.t.Fatalf("%s blocked: selected capability requires missing scope/permission:\n%s", op, msg)
		}
	}
}

// jsonUnmarshal decodes a JSON string into v. It is a thin convenience wrapper
// for parsing command stdout captured as a string.
func jsonUnmarshal(s string, v any) error {
	return json.Unmarshal([]byte(s), v)
}

// repoRoot returns the module root (the parent of this integration/ package).
func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot determine integration package path")
	}
	return filepath.Dir(filepath.Dir(file))
}

var (
	buildMu   sync.Mutex
	builtBins = map[string]string{}
	buildOut  string
)

// buildBinary compiles ./cmd/<name> once per test run and returns the path to
// the resulting executable. Subsequent calls for the same binary are cached.
func buildBinary(t *testing.T, name string) string {
	t.Helper()
	buildMu.Lock()
	defer buildMu.Unlock()

	if buildOut == "" {
		// Created lazily under the lock so a failure here is retried by the next
		// caller rather than leaving an empty output dir cached.
		dir, err := os.MkdirTemp("", "atl-integration-bin-")
		if err != nil {
			t.Fatalf("create bin tempdir: %v", err)
		}
		buildOut = dir
	}
	if p, ok := builtBins[name]; ok {
		return p
	}
	out := filepath.Join(buildOut, name)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "build", "-o", out, "./cmd/"+name)
	boundProcess(cmd)
	cmd.WaitDelay = 2 * time.Second
	cmd.Dir = repoRoot(t)
	if combined, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build %s: %v\n%s", name, err, combined)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	gitOutput := func(args ...string) []byte {
		cmd := exec.CommandContext(ctx, "git", args...)
		cmd.Dir = repoRoot(t)
		output, err := cmd.Output()
		if err != nil {
			t.Fatalf("build identity git %v: %v", args, err)
		}
		return output
	}
	t.Logf("build identity: binary=%s sha256=%x git=%s tracked-diff-sha256=%x go=%s platform=%s/%s", name, sha256.Sum256(data), strings.TrimSpace(string(gitOutput("rev-parse", "HEAD"))), sha256.Sum256(gitOutput("diff", "HEAD", "--")), runtime.Version(), runtime.GOOS, runtime.GOARCH)
	sourceHash, err := goSourceDigest(repoRoot(t), gitOutput("ls-files", "--cached", "--others", "--exclude-standard", "-z"))
	if err != nil {
		t.Fatalf("source tree digest: %v", err)
	}
	t.Logf("Go source tree sha256=%x (tracked/untracked .go, go.mod, go.sum; temporary reauth excluded)", sourceHash)
	t.Logf("working tree (includes untracked files):\n%s", gitOutput("status", "--short"))
	builtBins[name] = out
	return out
}

// goSourceDigest includes untracked Go source without opening unrelated local
// credential files. Symlinks are hashed as links, never followed outside the tree.
func goSourceDigest(root string, paths []byte) ([32]byte, error) {
	unique := map[string]bool{}
	for _, path := range strings.Split(string(paths), "\x00") {
		if strings.Contains(path, ".tmp-reauth") || (filepath.Ext(path) != ".go" && path != "go.mod" && path != "go.sum") {
			continue
		}
		unique[path] = true
	}
	names := make([]string, 0, len(unique))
	for name := range unique {
		names = append(names, name)
	}
	sort.Strings(names)
	digest := sha256.New()
	for _, name := range names {
		full := filepath.Join(root, name)
		info, err := os.Lstat(full)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return [32]byte{}, err
		}
		var data []byte
		kind := "file"
		if info.Mode()&os.ModeSymlink != 0 {
			target, err := os.Readlink(full)
			if err != nil {
				return [32]byte{}, err
			}
			kind, data = "symlink", []byte(target)
		} else {
			data, err = os.ReadFile(full)
			if err != nil {
				return [32]byte{}, err
			}
		}
		fmt.Fprintf(digest, "%s\x00%s\x00%d\x00", name, kind, len(data))
		digest.Write(data)
	}
	var result [32]byte
	copy(result[:], digest.Sum(nil))
	return result, nil
}

// cleanupOwned records ownership while allowing product-specific trash/purge
// semantics. The callback must verify removal and report useful diagnostics.
func (s *session) cleanupOwned(kind, id string, cleanup func() bool) {
	s.t.Helper()
	record := func(state string) {
		cleanupMu.Lock()
		defer cleanupMu.Unlock()
		if cleanupLedger == "" {
			f, err := os.CreateTemp("", "atl-integration-cleanup-*.jsonl")
			if err != nil {
				s.t.Fatalf("create cleanup ledger: %v", err)
			}
			cleanupLedger = f.Name()
			if err := f.Close(); err != nil {
				s.t.Fatal(err)
			}
		}
		f, err := os.OpenFile(cleanupLedger, os.O_APPEND|os.O_WRONLY, 0600)
		if err != nil {
			s.t.Errorf("open cleanup ledger: %v", err)
			return
		}
		entry := map[string]any{"test": s.t.Name(), "binary": filepath.Base(s.binaryPath), "site": s.site, "kind": kind, "id": id, "state": state, "target": s.target}
		if err := json.NewEncoder(f).Encode(entry); err != nil {
			s.t.Errorf("write cleanup ledger: %v", err)
		}
		if err := f.Close(); err != nil {
			s.t.Errorf("close cleanup ledger: %v", err)
		}
	}
	s.t.Cleanup(func() {
		if cleanup() {
			record("deleted")
		} else {
			s.t.Errorf("cleanup incomplete for %s %s; see %s", kind, id, cleanupLedger)
		}
	})
	record("pending")
	s.t.Logf("owned %s %s; cleanup ledger %s", kind, id, cleanupLedger)
}

var cleanupMu sync.Mutex
var cleanupLedger string

func (s *session) assertMissing(args ...string) bool {
	s.t.Helper()
	res := s.run(append(args, "--json")...)
	var envelope struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal([]byte(res.stdout+res.stderr), &envelope); err != nil || res.err == nil || envelope.Error != "not_found_or_not_visible" {
		s.t.Errorf("expected missing resource for %v: %v\n%s", args, res.err, res.stdout+res.stderr)
		return false
	}
	return true
}

func TestMain(m *testing.M) {
	code := m.Run()
	if buildOut != "" {
		if err := os.RemoveAll(buildOut); err != nil {
			fmt.Fprintln(os.Stderr, "remove test binaries:", err)
			code = 1
		}
	}
	os.Exit(code)
}
