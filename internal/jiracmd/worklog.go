package jiracmd

import (
	"fmt"
	"io"
	"time"

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
		since string
	)
	cmd := &cobra.Command{
		Use:   "list <issue>",
		Short: "List worklog entries on an issue",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			startedAfter, err := parseSince(since)
			if err != nil {
				return err
			}
			jc, err := jiraClient(info, g)
			if err != nil {
				return err
			}
			list := jc.ListWorklogs
			if all {
				list = jc.ListWorklogsAll
			}
			raw, err := list(cmd.Context(), args[0], startedAfter, limit)
			if err != nil {
				return err
			}
			return cli.RenderDecoded(cmd, g, raw, jira.Decode[jira.WorklogPage],
				func(w io.Writer, page jira.WorklogPage) {
					writeWorklogList(w, page.Worklogs)
				})
		},
	}
	cmd.Flags().StringVar(&since, "since", "",
		"only worklogs started on/after this time (date YYYY-MM-DD or RFC3339)")
	cli.AddPaginationFlags(cmd, &limit, &all, "worklogs")
	return cmd
}

// parseSince converts the --since flag to a Unix epoch-millisecond value for
// the worklog API's startedAfter parameter. It accepts a date (YYYY-MM-DD,
// interpreted as midnight UTC) or a full RFC3339 timestamp. An empty value
// yields 0 (unset).
func parseSince(since string) (int64, error) {
	if since == "" {
		return 0, nil
	}
	if t, err := time.Parse(time.RFC3339, since); err == nil {
		return t.UnixMilli(), nil
	}
	if t, err := time.Parse("2006-01-02", since); err == nil {
		return t.UnixMilli(), nil
	}
	return 0, apperr.InvalidInput(
		fmt.Sprintf("invalid --since %q; expected a date (YYYY-MM-DD) or RFC3339 timestamp", since))
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
			return cli.RenderDecoded(cmd, g, raw, jira.Decode[jira.Worklog],
				func(w io.Writer, wl jira.Worklog) {
					fmt.Fprintf(w, "logged %s on %s (worklog %s)\n",
						wl.TimeSpent, args[0], wl.ID)
				})
		},
	}
	f := cmd.Flags()
	f.StringVar(&timeSpent, "time", "", "duration to log, e.g. 3h 30m (required)")
	f.StringVar(&comment, "comment", "",
		"worklog comment; plain text is wrapped as ADF")
	return cmd
}

// writeWorklogList prints each worklog as a label/value block followed by its
// rendered comment, separated by blank lines.
func writeWorklogList(w io.Writer, worklogs []jira.Worklog) {
	if len(worklogs) == 0 {
		fmt.Fprintln(w, "No worklogs found.")
		return
	}
	for i, wl := range worklogs {
		if i > 0 {
			fmt.Fprintln(w)
		}
		writeWorklog(w, wl)
	}
}

// writeWorklog prints a single worklog as label/value lines followed by its
// plain-text comment, if any.
func writeWorklog(w io.Writer, wl jira.Worklog) {
	author := "-"
	if wl.Author != nil && wl.Author.DisplayName != "" {
		author = wl.Author.DisplayName
	}
	lw := output.NewLabelWriter(w)
	lw.Add("id", wl.ID)
	lw.Add("author", author)
	lw.Add("time spent", wl.TimeSpent)
	lw.AddIf("started", wl.Started)
	_ = lw.Flush()
	if text := jira.TextOf(wl.Comment); text != "" {
		fmt.Fprintf(w, "\n%s\n", text)
	}
}
