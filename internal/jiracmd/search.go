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
	var (
		limit int
		all   bool
	)
	cmd := &cobra.Command{
		Use:   "issues <jql>",
		Short: "Search issues with a raw JQL query",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			jc, err := jiraClient(info, g)
			if err != nil {
				return err
			}
			search := jc.SearchIssues
			if all {
				search = jc.SearchIssuesAll
			}
			raw, err := search(cmd.Context(), args[0], limit)
			if err != nil {
				return err
			}
			if g.WantsStructured() {
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
	cli.AddPaginationFlags(cmd, &limit, &all, "issues")
	return cmd
}
