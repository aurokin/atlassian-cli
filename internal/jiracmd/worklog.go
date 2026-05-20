package jiracmd

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/aurokin/atlassian-cli/internal/apperr"
	"github.com/aurokin/atlassian-cli/internal/appinfo"
	"github.com/aurokin/atlassian-cli/internal/cli"
	"github.com/aurokin/atlassian-cli/internal/jira"
	"github.com/aurokin/atlassian-cli/internal/output"
)

// newWorklogCommand builds the "issue worklog" command group.
func newWorklogCommand(info appinfo.Info, g *cli.GlobalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "worklog",
		Short: "List and add worklog entries on an issue",
	}
	cmd.AddCommand(
		newWorklogListCommand(info, g),
		newWorklogAddCommand(info, g),
	)
	return cmd
}

func newWorklogListCommand(info appinfo.Info, g *cli.GlobalFlags) *cobra.Command {
	var (
		limit int
		all   bool
	)
	cmd := &cobra.Command{
		Use:   "list <issue>",
		Short: "List worklog entries on an issue",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			jc, err := jiraClient(info, g)
			if err != nil {
				return err
			}
			list := jc.ListWorklogs
			if all {
				list = jc.ListWorklogsAll
			}
			raw, err := list(cmd.Context(), args[0], limit)
			if err != nil {
				return err
			}
			if g.JSON != "" || g.JQ != "" {
				return cli.Render(cmd, g, raw)
			}
			page, err := jira.Decode[jira.WorklogPage](raw)
			if err != nil {
				return err
			}
			writeWorklogList(cmd.OutOrStdout(), page.Worklogs)
			return nil
		},
	}
	f := cmd.Flags()
	f.IntVar(&limit, "limit", 0, "maximum number of worklogs to return")
	f.BoolVar(&all, "all", false, "follow pagination and return every page (--limit sets the page size)")
	return cmd
}

func newWorklogAddCommand(info appinfo.Info, g *cli.GlobalFlags) *cobra.Command {
	var (
		timeSpent string
		comment   string
	)
	cmd := &cobra.Command{
		Use:   "add <issue>",
		Short: "Log work against an issue",
		Long: "Adds a worklog entry to an issue. --time is sent verbatim as Jira's\n" +
			"timeSpent (duration strings like \"3h 30m\" or seconds with units);\n" +
			"the CLI does not parse or convert the value. --comment, when given,\n" +
			"is plain text wrapped as an ADF document.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if timeSpent == "" {
				return apperr.InvalidInput("issue worklog add requires --time")
			}
			jc, err := jiraClient(info, g)
			if err != nil {
				return err
			}
			var commentADF []byte
			if comment != "" {
				commentADF = jira.DocOf(comment)
			}
			raw, err := jc.AddWorklog(cmd.Context(), args[0], timeSpent, commentADF)
			if err != nil {
				return err
			}
			if g.JSON != "" || g.JQ != "" {
				return cli.Render(cmd, g, raw)
			}
			wl, err := jira.Decode[jira.Worklog](raw)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "logged %s on %s (worklog %s)\n",
				wl.TimeSpent, args[0], wl.ID)
			return nil
		},
	}
	f := cmd.Flags()
	f.StringVar(&timeSpent, "time", "", "duration to log, e.g. 3h 30m (required)")
	f.StringVar(&comment, "comment", "",
		"worklog comment; plain text is wrapped as ADF")
	return cmd
}

// writeWorklogList prints worklog entries as aligned
// id/author/time-spent/started rows.
func writeWorklogList(w io.Writer, worklogs []jira.Worklog) {
	if len(worklogs) == 0 {
		fmt.Fprintln(w, "No worklog entries.")
		return
	}
	tw := output.TabWriter(w)
	for _, wl := range worklogs {
		author := ""
		if wl.Author != nil {
			author = wl.Author.DisplayName
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", wl.ID, author, wl.TimeSpent, wl.Started)
	}
	_ = tw.Flush()
}
