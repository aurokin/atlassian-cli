package jiracmd

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/aurokin/atlassian-cli/internal/apperr"
	"github.com/aurokin/atlassian-cli/internal/appinfo"
	"github.com/aurokin/atlassian-cli/internal/cli"
	"github.com/aurokin/atlassian-cli/internal/jira"
)

// newIssueCommand builds the "issue" command group.
func newIssueCommand(info appinfo.Info, g *cli.GlobalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "issue",
		Short: "List and view Jira issues",
	}
	cmd.AddCommand(
		newIssueListCommand(info, g),
		newIssueViewCommand(info, g),
		newCommentCommand(info, g),
	)
	return cmd
}

func newIssueListCommand(info appinfo.Info, g *cli.GlobalFlags) *cobra.Command {
	var (
		project  string
		status   string
		assignee string
		limit    int
	)
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List issues in a project",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if project == "" {
				return apperr.InvalidInput("a project key is required; pass --project")
			}
			jc, err := jiraClient(info, g)
			if err != nil {
				return err
			}
			raw, err := jc.SearchIssues(cmd.Context(), buildIssueListJQL(project, status, assignee), limit)
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
	f := cmd.Flags()
	f.StringVar(&project, "project", "", "project key (required)")
	f.StringVar(&status, "status", "", "filter by status name")
	f.StringVar(&assignee, "assignee", "", "filter by assignee account id, or currentUser()")
	f.IntVar(&limit, "limit", 0, "maximum number of issues to return")
	return cmd
}

func newIssueViewCommand(info appinfo.Info, g *cli.GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "view <issue>",
		Short: "View a single Jira issue",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			jc, err := jiraClient(info, g)
			if err != nil {
				return err
			}
			raw, err := jc.GetIssue(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			if g.JSON != "" || g.JQ != "" {
				return cli.Render(cmd, g, raw)
			}
			iss, err := jira.Decode[jira.Issue](raw)
			if err != nil {
				return err
			}
			writeIssue(cmd.OutOrStdout(), iss)
			return nil
		},
	}
}

// buildIssueListJQL turns the issue-list filter flags into a JQL query. The
// project clause is always present; status and assignee are added when set.
// The literal currentUser() is passed through unquoted as a JQL function.
func buildIssueListJQL(project, status, assignee string) string {
	clauses := []string{"project = " + jqlQuote(project)}
	if status != "" {
		clauses = append(clauses, "status = "+jqlQuote(status))
	}
	if assignee == "currentUser()" {
		clauses = append(clauses, "assignee = currentUser()")
	} else if assignee != "" {
		clauses = append(clauses, "assignee = "+jqlQuote(assignee))
	}
	return strings.Join(clauses, " AND ") + " ORDER BY created DESC"
}

// jqlQuote wraps a JQL string value in double quotes, escaping the characters
// that are significant inside a quoted JQL string.
func jqlQuote(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		if r == '"' || r == '\\' {
			b.WriteByte('\\')
		}
		b.WriteRune(r)
	}
	b.WriteByte('"')
	return b.String()
}

// namedOr returns a NamedField's name, or fallback when it is absent.
func namedOr(n *jira.NamedField, fallback string) string {
	if n != nil && n.Name != "" {
		return n.Name
	}
	return fallback
}

// writeIssueList prints issues as aligned key/status/summary rows.
func writeIssueList(w io.Writer, issues []jira.Issue) {
	if len(issues) == 0 {
		fmt.Fprintln(w, "No issues found.")
		return
	}
	tw := tabWriter(w)
	for _, iss := range issues {
		fmt.Fprintf(tw, "%s\t%s\t%s\n", iss.Key, namedOr(iss.Fields.Status, "-"), iss.Fields.Summary)
	}
	_ = tw.Flush()
}

// writeIssue prints a single issue as aligned label/value lines.
func writeIssue(w io.Writer, iss jira.Issue) {
	f := iss.Fields
	fmt.Fprintf(w, "%-9s %s\n", "key:", iss.Key)
	fmt.Fprintf(w, "%-9s %s\n", "summary:", f.Summary)
	if f.Status != nil {
		fmt.Fprintf(w, "%-9s %s\n", "status:", f.Status.Name)
	}
	if f.IssueType != nil {
		fmt.Fprintf(w, "%-9s %s\n", "type:", f.IssueType.Name)
	}
	if f.Priority != nil {
		fmt.Fprintf(w, "%-9s %s\n", "priority:", f.Priority.Name)
	}
	if f.Assignee != nil && f.Assignee.DisplayName != "" {
		fmt.Fprintf(w, "%-9s %s\n", "assignee:", f.Assignee.DisplayName)
	}
	if f.Reporter != nil && f.Reporter.DisplayName != "" {
		fmt.Fprintf(w, "%-9s %s\n", "reporter:", f.Reporter.DisplayName)
	}
	if f.Created != "" {
		fmt.Fprintf(w, "%-9s %s\n", "created:", f.Created)
	}
	if f.Updated != "" {
		fmt.Fprintf(w, "%-9s %s\n", "updated:", f.Updated)
	}
}
