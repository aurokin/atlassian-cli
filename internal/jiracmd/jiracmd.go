// Package jiracmd assembles the Jira-specific command tree for the atl-jira
// binary: project, issue, search, and status. The shared commands (auth, api,
// resolve, browse, version) are built by internal/cli; AddCommands layers the
// Jira product commands on top of that root.
package jiracmd

import (
	"io"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/aurokin/atlassian-cli/internal/appinfo"
	"github.com/aurokin/atlassian-cli/internal/cli"
	"github.com/aurokin/atlassian-cli/internal/jira"
)

// AddCommands registers the Jira product commands on the atl-jira root.
func AddCommands(root *cobra.Command, info appinfo.Info, g *cli.GlobalFlags) {
	root.AddCommand(newProjectCommand(info, g))
}

// jiraClient builds a typed Jira client for the profile named by --site.
func jiraClient(info appinfo.Info, g *cli.GlobalFlags) (*jira.Client, error) {
	c, err := cli.SiteClient(info, g)
	if err != nil {
		return nil, err
	}
	return jira.New(c), nil
}

// tabWriter returns a tabwriter for aligned, column-separated list output.
func tabWriter(w io.Writer) *tabwriter.Writer {
	return tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
}
