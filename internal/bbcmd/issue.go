package bbcmd

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/aurokin/atlassian-cli/internal/apperr"
	"github.com/aurokin/atlassian-cli/internal/appinfo"
	"github.com/aurokin/atlassian-cli/internal/bitbucket"
	"github.com/aurokin/atlassian-cli/internal/cli"
	"github.com/aurokin/atlassian-cli/internal/output"
)

// normalizeIssueState trims a --state value and maps the synthetic "ALL"
// (case-insensitive) to the empty filter the client treats as "every state".
// Bitbucket issue states are lower-case, so the value is otherwise passed
// through verbatim.
func normalizeIssueState(state string) string {
	s := strings.TrimSpace(state)
	if strings.EqualFold(s, "ALL") {
		return ""
	}
	return s
}

// newIssueCommand builds the "issue" command group.
func newIssueCommand(info appinfo.Info, g *cli.GlobalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "issue",
		Aliases: []string{"issues"},
		Short:   "List, view, and create Bitbucket issues",
	}
	cmd.AddCommand(
		newIssueListCommand(info, g),
		newIssueViewCommand(info, g),
		newIssueCreateCommand(info, g),
	)
	return cmd
}

func newIssueListCommand(info appinfo.Info, g *cli.GlobalFlags) *cobra.Command {
	var (
		repoFlag      string
		workspaceFlag string
		state         string
		limit         int
		all           bool
	)
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List a repository's issues",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			target, err := resolveRepoTarget(nil, repoFlag, workspaceFlag)
			if err != nil {
				return err
			}
			bc, err := bbClient(info, g)
			if err != nil {
				return err
			}
			st := normalizeIssueState(state)
			list := bc.ListIssues
			if all {
				list = bc.ListIssuesAll
			}
			raw, err := list(cmd.Context(), target.Workspace, target.Repo, st, limit)
			if err != nil {
				return err
			}
			if g.WantsStructured() {
				return cli.Render(cmd, g, raw)
			}
			page, err := bitbucket.Decode[bitbucket.IssuePage](raw)
			if err != nil {
				return err
			}
			writeIssueList(cmd.OutOrStdout(), page.Values)
			return nil
		},
	}
	addRepoFlags(cmd, &repoFlag, &workspaceFlag)
	f := cmd.Flags()
	f.StringVar(&state, "state", "", "filter by issue state (e.g. new, open, resolved, closed) or ALL")
	cli.AddPaginationFlags(cmd, &limit, &all, "issues")
	return cmd
}

func newIssueViewCommand(info appinfo.Info, g *cli.GlobalFlags) *cobra.Command {
	var (
		repoFlag      string
		workspaceFlag string
	)
	cmd := &cobra.Command{
		Use:   "view <id>",
		Short: "View a single issue",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseIssueID(args[0])
			if err != nil {
				return err
			}
			target, err := resolveRepoTarget(nil, repoFlag, workspaceFlag)
			if err != nil {
				return err
			}
			bc, err := bbClient(info, g)
			if err != nil {
				return err
			}
			raw, err := bc.GetIssue(cmd.Context(), target.Workspace, target.Repo, id)
			if err != nil {
				return err
			}
			if g.WantsStructured() {
				return cli.Render(cmd, g, raw)
			}
			issue, err := bitbucket.Decode[bitbucket.Issue](raw)
			if err != nil {
				return err
			}
			writeIssue(cmd.OutOrStdout(), issue)
			return nil
		},
	}
	addRepoFlags(cmd, &repoFlag, &workspaceFlag)
	return cmd
}

func newIssueCreateCommand(info appinfo.Info, g *cli.GlobalFlags) *cobra.Command {
	var (
		repoFlag      string
		workspaceFlag string
		opts          bitbucket.CreateIssueOptions
	)
	cmd := &cobra.Command{
		Use:   "create",
		Short: "File a new issue",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if strings.TrimSpace(opts.Title) == "" {
				return apperr.InvalidInput("a title is required; pass --title")
			}
			target, err := resolveRepoTarget(nil, repoFlag, workspaceFlag)
			if err != nil {
				return err
			}
			bc, err := bbClient(info, g)
			if err != nil {
				return err
			}
			raw, err := bc.CreateIssue(cmd.Context(), target.Workspace, target.Repo, opts)
			if err != nil {
				return err
			}
			if g.WantsStructured() {
				return cli.Render(cmd, g, raw)
			}
			issue, err := bitbucket.Decode[bitbucket.Issue](raw)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "created issue #%d: %s\n", issue.ID, issue.Title)
			return nil
		},
	}
	addRepoFlags(cmd, &repoFlag, &workspaceFlag)
	f := cmd.Flags()
	f.StringVar(&opts.Title, "title", "", "issue title (required)")
	f.StringVar(&opts.Body, "body", "", "issue description (raw markup)")
	f.StringVar(&opts.Kind, "kind", "", "issue kind: bug, enhancement, proposal, or task")
	f.StringVar(&opts.Priority, "priority", "", "issue priority: trivial, minor, major, critical, or blocker")
	return cmd
}

// parseIssueID parses a positive issue id.
func parseIssueID(s string) (int, error) {
	id, err := parsePositiveInt(s)
	if err != nil {
		return 0, apperr.InvalidInput(fmt.Sprintf("invalid issue id %q; expected a positive integer", s))
	}
	return id, nil
}

// writeIssueList prints issues as aligned id/state/kind/title rows.
func writeIssueList(w io.Writer, issues []bitbucket.Issue) {
	if len(issues) == 0 {
		fmt.Fprintln(w, "No issues found.")
		return
	}
	tw := output.TabWriter(w)
	for _, is := range issues {
		fmt.Fprintf(tw, "#%d\t%s\t%s\t%s\n", is.ID, is.State, is.Kind, is.Title)
	}
	_ = tw.Flush()
}

// writeIssue prints a single issue as aligned label/value lines.
func writeIssue(w io.Writer, is bitbucket.Issue) {
	fmt.Fprintf(w, "%-10s #%d\n", "id:", is.ID)
	fmt.Fprintf(w, "%-10s %s\n", "title:", is.Title)
	if is.State != "" {
		fmt.Fprintf(w, "%-10s %s\n", "state:", is.State)
	}
	if is.Kind != "" {
		fmt.Fprintf(w, "%-10s %s\n", "kind:", is.Kind)
	}
	if is.Priority != "" {
		fmt.Fprintf(w, "%-10s %s\n", "priority:", is.Priority)
	}
	if reporter := accountLabel(is.Reporter); reporter != "" {
		fmt.Fprintf(w, "%-10s %s\n", "reporter:", reporter)
	}
	if assignee := accountLabel(is.Assignee); assignee != "" {
		fmt.Fprintf(w, "%-10s %s\n", "assignee:", assignee)
	}
	if is.CreatedOn != "" {
		fmt.Fprintf(w, "%-10s %s\n", "created:", is.CreatedOn)
	}
	if is.Content != nil && is.Content.Raw != "" {
		fmt.Fprintf(w, "%-10s %s\n", "body:", is.Content.Raw)
	}
}
