// Package atlconfcmd assembles the command tree for the atl-conf binary.
package atlconfcmd

import (
	"github.com/spf13/cobra"

	"github.com/aurokin/atlassian-cli/internal/appinfo"
	"github.com/aurokin/atlassian-cli/internal/cli"
	"github.com/aurokin/atlassian-cli/internal/confcmd"
)

const short = "atl-conf is a true-to-API command-line interface for Atlassian Confluence"

// buildInfo describes the atl-conf binary for the given build metadata.
func buildInfo(version, commit, date string) appinfo.Info {
	return appinfo.New("atl-conf", appinfo.ProductConfluence, version, commit, date)
}

// NewRoot builds the root command for the atl-conf binary together with the
// global flags bound to it. It layers the Confluence product commands onto the
// shared root. The build metadata is supplied by the binary's main package.
func NewRoot(version, commit, date string) (*cobra.Command, *cli.GlobalFlags) {
	info := buildInfo(version, commit, date)
	root, g := cli.NewRoot(info, short)
	confcmd.AddCommands(root, info, g)
	return root, g
}

// Run builds the atl-conf command tree and runs it through the shared entry
// point (alias expansion + execute + extension fallback), returning the
// process exit code.
func Run(version, commit, date string) int {
	root, g := NewRoot(version, commit, date)
	return cli.Run(buildInfo(version, commit, date), root, g)
}
