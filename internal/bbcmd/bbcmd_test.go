package bbcmd

import (
	"bytes"
	"testing"

	"github.com/aurokin/atlassian-cli/internal/appinfo"
	"github.com/aurokin/atlassian-cli/internal/auth"
	"github.com/aurokin/atlassian-cli/internal/cli"
	"github.com/aurokin/atlassian-cli/internal/config"
)

// bbTestInfo is the appinfo used by the command tests.
func bbTestInfo() appinfo.Info {
	return appinfo.New("atl-bb", appinfo.ProductBitbucket, "test", "", "")
}

// execBB builds a fresh atl-bb root — the shared commands plus the Bitbucket
// product commands — runs it with args, and returns the combined output and
// the execution error.
func execBB(t *testing.T, args ...string) (string, error) {
	t.Helper()
	info := bbTestInfo()
	root, g := cli.NewRoot(info, "atl-bb test")
	AddCommands(root, info, g)
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs(args)
	err := root.Execute()
	return buf.String(), err
}

// stubInferDisabled makes git-checkout inference report "not inferable" for the
// duration of a test, so a test that exercises the missing-target path does not
// accidentally pick up the ambient checkout's git remote.
func stubInferDisabled(t *testing.T) {
	t.Helper()
	orig := inferRepoTarget
	inferRepoTarget = func() (repoTarget, bool) { return repoTarget{}, false }
	t.Cleanup(func() { inferRepoTarget = orig })
}

// stubInfer makes git-checkout inference return a fixed workspace/repo.
func stubInfer(t *testing.T, workspace, repo string) {
	t.Helper()
	orig := inferRepoTarget
	inferRepoTarget = func() (repoTarget, bool) {
		return repoTarget{Workspace: workspace, Repo: repo}, true
	}
	t.Cleanup(func() { inferRepoTarget = orig })
}

// loginBBSite records a cloud-classic Bitbucket site profile named "work"
// pointing at srvURL, arming ATL_API_TOKEN as its token reference. It writes
// the config directly rather than through `auth login`, because auth login
// requires an https URL for the cloud-classic (Basic) style while the test
// server is served over http. No raw token is written to config — the token
// stays in the environment variable the token_ref points at.
func loginBBSite(t *testing.T, srvURL string) {
	t.Helper()
	t.Setenv("ATL_API_TOKEN", "test-token")
	path, err := config.DefaultPath()
	if err != nil {
		t.Fatalf("config path: %v", err)
	}
	cfg := config.New()
	cfg.Sites["work"] = config.SiteProfile{
		Product:    string(appinfo.ProductBitbucket),
		Deployment: "cloud",
		BaseURL:    srvURL,
		Username:   "auro@example.com",
		TokenStyle: string(auth.StyleCloudClassic),
		AuthType:   auth.StyleCloudClassic.AuthType(),
		TokenRef:   "env:ATL_API_TOKEN",
	}
	if err := config.Save(path, cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}
}
