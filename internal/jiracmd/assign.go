package jiracmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/aurokin/atlassian-cli/internal/appinfo"
	"github.com/aurokin/atlassian-cli/internal/cli"
)

func newIssueAssignCommand(info appinfo.Info, g *cli.GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "assign <issue> <account-id|email|@me|->",
		Short: "Assign an issue, or pass - to unassign",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			jc, err := jiraClient(info, g)
			if err != nil {
				return err
			}
			var accountID *string
			if args[1] != "-" {
				v, err := resolveAccountID(cmd.Context(), jc, args[1])
				if err != nil {
					return err
				}
				accountID = &v
			}
			if err := jc.AssignIssue(cmd.Context(), args[0], accountID); err != nil {
				return err
			}
			if g.WantsStructured() {
				return cli.Render(cmd, g, assignResult{
					Issue: args[0], Assignee: accountID, Assigned: accountID != nil})
			}
			if accountID == nil {
				fmt.Fprintf(cmd.OutOrStdout(), "unassigned %s\n", args[0])
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "assigned %s to %s\n", args[0], *accountID)
			}
			return nil
		},
	}
}

// assignResult is the synthesized outcome of an assignment, whose API call
// returns no body, so --json has a stable object to render. Assignee is null
// when the issue was unassigned.
type assignResult struct {
	Issue    string  `json:"issue"`
	Assignee *string `json:"assignee"`
	Assigned bool    `json:"assigned"`
}
