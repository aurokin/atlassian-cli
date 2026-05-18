package jiracmd

import (
	"github.com/spf13/cobra"

	"github.com/aurokin/atlassian-cli/internal/appinfo"
	"github.com/aurokin/atlassian-cli/internal/cli"
	"github.com/aurokin/atlassian-cli/internal/jira"
)

// newSearchCommand builds the "search" command group.
func newSearchCommand(info appinfo.Info, g *cli.GlobalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "search",
		Short: "Search Jira with JQL",
	}
	cmd.AddCommand(newSearchIssuesCommand(info, g))
	return cmd
}

func newSearchIssuesCommand(info appinfo.Info, g *cli.GlobalFlags) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:   "issues <JQL>",
		Short: "Search issues with a raw JQL query",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			jc, err := jiraClient(info, g)
			if err != nil {
				return err
			}
			raw, err := jc.SearchIssues(cmd.Context(), args[0], limit)
			if err != nil {
				return err
			}
			if g.JSON != "" || g.JQ != "" {
				return cli.Render(cmd, g, raw)
			}
			page, err := jira.Decode[jira.IssuePage](raw)
			if err != nil {
				return err
			}
			writeIssueList(cmd.OutOrStdout(), page.Issues)
			return nil
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 0, "maximum number of issues to return")
	return cmd
}
