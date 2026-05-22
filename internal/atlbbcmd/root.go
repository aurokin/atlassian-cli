// Package atlbbcmd assembles the command tree for the atl-bb binary.
package atlbbcmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/aurokin/atlassian-cli/internal/appinfo"
	"github.com/aurokin/atlassian-cli/internal/bbcmd"
	"github.com/aurokin/atlassian-cli/internal/cli"
)

const short = "atl-bb is a true-to-API command-line interface for Atlassian Bitbucket"

// NewRoot builds the root command for the atl-bb binary together with the
// global flags bound to it. It layers the Bitbucket product commands onto the
// shared root. The build metadata is supplied by the binary's main package.
func NewRoot(version, commit, date string) (*cobra.Command, *cli.GlobalFlags) {
	info := appinfo.New("atl-bb", appinfo.ProductBitbucket, version, commit, date)
	root, g := cli.NewRoot(info, short)
	bbcmd.AddCommands(root, info, g)
	return root, g
}

// Run builds the atl-bb command tree, expands any configured command aliases
// against the process arguments, executes the tree, and returns the process
// exit code.
func Run(version, commit, date string) int {
	root, g := NewRoot(version, commit, date)
	args, err := bbcmd.ExpandAliases(os.Args[1:])
	if err != nil {
		// An alias-expansion failure happens before dispatch (a malformed alias
		// in a hand-edited config). Report it plainly and stop.
		fmt.Fprintln(os.Stderr, "Error:", err)
		return 1
	}
	root.SetArgs(args)
	return cli.Execute(root, g)
}
