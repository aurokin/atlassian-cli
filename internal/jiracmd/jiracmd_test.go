package jiracmd

import (
	"bytes"
	"testing"

	"github.com/aurokin/atlassian-cli/internal/appinfo"
	"github.com/aurokin/atlassian-cli/internal/cli"
)

// jiraTestInfo is the appinfo used by the command tests.
func jiraTestInfo() appinfo.Info {
	return appinfo.New("atl-jira", appinfo.ProductJira, "test", "", "")
}

// execJira builds a fresh atl-jira root — the shared commands plus the Jira
// product commands — runs it with args, and returns the combined output and
// the execution error.
func execJira(t *testing.T, args ...string) (string, error) {
	t.Helper()
	info := jiraTestInfo()
	root, g := cli.NewRoot(info, "atl-jira test")
	AddCommands(root, info, g)
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs(args)
	err := root.Execute()
	return buf.String(), err
}

// loginJiraSite records a data-center site profile named "work" pointing at
// srvURL, arming ATL_API_TOKEN as its token reference.
func loginJiraSite(t *testing.T, srvURL string) {
	t.Helper()
	t.Setenv("ATL_API_TOKEN", "test-token")
	if _, err := execJira(t, "auth", "login", "--site", "work",
		"--url", srvURL, "--token-style", "data-center-pat", "--token-env", "ATL_API_TOKEN"); err != nil {
		t.Fatalf("login: %v", err)
	}
}
