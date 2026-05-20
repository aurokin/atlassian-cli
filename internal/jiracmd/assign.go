package jiracmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/aurokin/atlassian-cli/internal/appinfo"
	"github.com/aurokin/atlassian-cli/internal/cli"
)

func newIssueAssignCommand(info appinfo.Info, g *cli.GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "assign <issue> <account-id|->",
		Short: "Assign an issue, or pass - to unassign",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			jc, err := jiraClient(info, g)
			if err != nil {
				return err
			}
			var accountID *string
			if args[1] != "-" {
				v := args[1]
				accountID = &v
			}
			if err := jc.AssignIssue(cmd.Context(), args[0], accountID); err != nil {
				return err
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
