package jiracmd

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/aurokin/atlassian-cli/internal/appinfo"
	"github.com/aurokin/atlassian-cli/internal/cli"
	"github.com/aurokin/atlassian-cli/internal/jira"
	"github.com/aurokin/atlassian-cli/internal/output"
)

func newIssueWatchCommand(info appinfo.Info, g *cli.GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "watch <issue>",
		Short: "Add yourself as a watcher of an issue",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			jc, err := jiraClient(info, g)
			if err != nil {
				return err
			}
			// An empty accountID makes Jira add the authenticated user.
			if err := jc.AddWatcher(cmd.Context(), args[0], ""); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "watching %s\n", args[0])
			return nil
		},
	}
}

func newIssueUnwatchCommand(info appinfo.Info, g *cli.GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "unwatch <issue>",
		Short: "Remove yourself as a watcher of an issue",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			jc, err := jiraClient(info, g)
			if err != nil {
				return err
			}
			// DELETE /watchers requires an explicit accountId; look up the
			// calling user via /myself.
			rawMe, err := jc.Myself(cmd.Context())
			if err != nil {
				return err
			}
			me, err := jira.Decode[jira.User](rawMe)
			if err != nil {
				return err
			}
			if err := jc.RemoveWatcher(cmd.Context(), args[0], me.AccountID); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "no longer watching %s\n", args[0])
			return nil
		},
	}
}

func newIssueWatchersCommand(info appinfo.Info, g *cli.GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "watchers <issue>",
		Short: "List the watchers of an issue",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			jc, err := jiraClient(info, g)
			if err != nil {
				return err
			}
			raw, err := jc.ListWatchers(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			if g.JSON != "" || g.JQ != "" {
				return cli.Render(cmd, g, raw)
			}
			ws, err := jira.Decode[jira.Watchers](raw)
			if err != nil {
				return err
			}
			writeWatchers(cmd.OutOrStdout(), ws.Watchers)
			return nil
		},
	}
}

// writeWatchers prints watchers as aligned account-id/display-name rows.
func writeWatchers(w io.Writer, users []jira.User) {
	if len(users) == 0 {
		fmt.Fprintln(w, "No watchers.")
		return
	}
	tw := output.TabWriter(w)
	for _, u := range users {
		fmt.Fprintf(tw, "%s\t%s\n", u.AccountID, u.DisplayName)
	}
	_ = tw.Flush()
}
