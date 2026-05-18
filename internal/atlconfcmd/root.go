// Package atlconfcmd assembles the command tree for the atl-conf binary.
package atlconfcmd

import (
	"github.com/spf13/cobra"

	"github.com/aurokin/atlassian-cli/internal/appinfo"
	"github.com/aurokin/atlassian-cli/internal/cli"
)

const short = "atl-conf is a true-to-API command-line interface for Atlassian Confluence"

// NewRoot builds the root command for the atl-conf binary together with the
// global flags bound to it. The build metadata is supplied by the binary's
// main package.
func NewRoot(version, commit, date string) (*cobra.Command, *cli.GlobalFlags) {
	info := appinfo.New("atl-conf", appinfo.ProductConfluence, version, commit, date)
	return cli.NewRoot(info, short)
}

// Run builds the atl-conf command tree, executes it, and returns the process
// exit code.
func Run(version, commit, date string) int {
	root, g := NewRoot(version, commit, date)
	return cli.Execute(root, g)
}
