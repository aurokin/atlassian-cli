// Package atljiracmd assembles the command tree for the atl-jira binary.
package atljiracmd

import (
	"github.com/spf13/cobra"

	"github.com/aurokin/atlassian-cli/internal/appinfo"
	"github.com/aurokin/atlassian-cli/internal/cli"
	"github.com/aurokin/atlassian-cli/internal/jiracmd"
)

const short = "atl-jira is a true-to-API command-line interface for Atlassian Jira"

// NewRoot builds the root command for the atl-jira binary together with the
// global flags bound to it. It layers the Jira product commands onto the
// shared root. The build metadata is supplied by the binary's main package.
func NewRoot(version, commit, date string) (*cobra.Command, *cli.GlobalFlags) {
	info := appinfo.New("atl-jira", appinfo.ProductJira, version, commit, date)
	root, g := cli.NewRoot(info, short)
	jiracmd.AddCommands(root, info, g)
	return root, g
}

// Run builds the atl-jira command tree, executes it, and returns the process
// exit code.
func Run(version, commit, date string) int {
	root, g := NewRoot(version, commit, date)
	return cli.Execute(root, g)
}
