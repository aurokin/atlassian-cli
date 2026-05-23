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
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
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
}

// newSession builds (once) the product's binary, then either reuses a stored
// profile or provisions a throwaway one from environment-variable credentials.
// It calls t.Skip when the required configuration for this product is absent,
// so a developer can exercise just one product without configuring the others.
func newSession(t *testing.T, p product) *session {
	t.Helper()
	requireIntegration(t)

	bin := buildBinary(t, p.binary)

	if useStoredProfiles() {
		site := p.env("SITE")
		if site == "" {
			t.Skipf("set ATL_IT_%s_SITE to run %s integration tests against a stored profile", p.envPrefix, p.binary)
		}
		return &session{t: t, binaryPath: bin, configHome: "", site: site}
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
		t.Skipf("set ATL_IT_%s_BASE_URL, ATL_IT_%s_TOKEN%s (or ATL_IT_USE_STORED_PROFILES=1) to run %s integration tests",
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
	return s
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
	full = append(full, "--site", s.site)

	cmd := exec.Command(s.binaryPath, full...)
	cmd.Env = os.Environ()
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

// mustJSON runs a command with --json and decodes stdout into v.
func (s *session) mustJSON(v any, args ...string) {
	s.t.Helper()
	res := s.mustRun(append(args, "--json")...)
	if err := json.Unmarshal([]byte(res.stdout), v); err != nil {
		s.t.Fatalf("decode JSON from %v: %v\nstdout:\n%s", args, err, res.stdout)
	}
}

// skipIfScopeOrPermission skips (rather than fails) when a command failed only
// because the credential lacks the scope or permission for that endpoint —
// that is a tenant/app configuration gap, not a CLI defect. Anything else is a
// real failure and is returned for the caller to assert on.
func (s *session) skipIfScopeOrPermission(res cmdResult, op string) {
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
			s.t.Skipf("%s requires a scope/permission this credential lacks; skipping:\n%s", op, msg)
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
	cmd := exec.Command("go", "build", "-o", out, "./cmd/"+name)
	cmd.Dir = repoRoot(t)
	if combined, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build %s: %v\n%s", name, err, combined)
	}
	builtBins[name] = out
	return out
}
