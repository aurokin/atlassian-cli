// Package atlbbcmd assembles the command tree for the atl-bb binary.
package atlbbcmd

import (
	"github.com/spf13/cobra"

	"github.com/aurokin/atlassian-cli/internal/appinfo"
	"github.com/aurokin/atlassian-cli/internal/bbcmd"
	"github.com/aurokin/atlassian-cli/internal/cli"
)

const short = "atl-bb is a true-to-API command-line interface for Atlassian Bitbucket"

// buildInfo describes the atl-bb binary for the given build metadata.
func buildInfo(version, commit, date string) appinfo.Info {
	return appinfo.New("atl-bb", appinfo.ProductBitbucket, version, commit, date)
}

// NewRoot builds the root command for the atl-bb binary together with the
// global flags bound to it. It layers the Bitbucket product commands onto the
// shared root. The build metadata is supplied by the binary's main package.
func NewRoot(version, commit, date string) (*cobra.Command, *cli.GlobalFlags) {
	info := buildInfo(version, commit, date)
	root, g := cli.NewRoot(info, short)
	bbcmd.AddCommands(root, info, g)
	return root, g
}

// Run builds the atl-bb command tree and runs it through the shared entry
// point (alias expansion + execute + extension fallback), returning the
// process exit code.
func Run(version, commit, date string) int {
	root, g := NewRoot(version, commit, date)
	return cli.Run(buildInfo(version, commit, date), root, g)
}
