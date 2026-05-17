// Package atljiracmd assembles the command tree for the atl-jira binary.
package atljiracmd

import (
	"github.com/spf13/cobra"

	"github.com/aurokin/atlassian-cli/internal/appinfo"
	"github.com/aurokin/atlassian-cli/internal/cli"
)

const short = "atl-jira is a true-to-API command-line interface for Atlassian Jira"

// NewRoot builds the root command for the atl-jira binary. The build metadata
// is supplied by the binary's main package.
func NewRoot(version, commit, date string) *cobra.Command {
	info := appinfo.New("atl-jira", appinfo.ProductJira, version, commit, date)
	root, _ := cli.NewRoot(info, short)
	return root
}
