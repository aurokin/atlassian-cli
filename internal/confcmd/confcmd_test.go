package confcmd

import (
	"bytes"
	"testing"

	"github.com/aurokin/atlassian-cli/internal/appinfo"
	"github.com/aurokin/atlassian-cli/internal/cli"
)

// confTestInfo is the appinfo used by the command tests.
func confTestInfo() appinfo.Info {
	return appinfo.New("atl-conf", appinfo.ProductConfluence, "test", "", "")
}

// execConf builds a fresh atl-conf root — the shared commands plus the
// Confluence product commands — runs it with args, and returns the combined
// output and the execution error.
func execConf(t *testing.T, args ...string) (string, error) {
	t.Helper()
	info := confTestInfo()
	root, g := cli.NewRoot(info, "atl-conf test")
	AddCommands(root, info, g)
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs(args)
	err := root.Execute()
	return buf.String(), err
}

// loginConfSite records a data-center site profile named "work" pointing at
// srvURL, arming ATL_API_TOKEN as its token reference.
func loginConfSite(t *testing.T, srvURL string) {
	t.Helper()
	t.Setenv("ATL_API_TOKEN", "test-token")
	if _, err := execConf(t, "auth", "login", "--site", "work",
		"--url", srvURL, "--token-style", "data-center-pat", "--token-env", "ATL_API_TOKEN"); err != nil {
		t.Fatalf("login: %v", err)
	}
}
