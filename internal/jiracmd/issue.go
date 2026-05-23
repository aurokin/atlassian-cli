package jiracmd

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/aurokin/atlassian-cli/internal/apperr"
	"github.com/aurokin/atlassian-cli/internal/appinfo"
	"github.com/aurokin/atlassian-cli/internal/cli"
	"github.com/aurokin/atlassian-cli/internal/jira"
	"github.com/aurokin/atlassian-cli/internal/output"
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
		newIssueCreateCommand(info, g),
		newIssueEditCommand(info, g),
		newIssueTransitionCommand(info, g),
		newIssueAssignCommand(info, g),
		newIssueWatchCommand(info, g),
		newIssueUnwatchCommand(info, g),
		newIssueWatchersCommand(info, g),
		newIssueLinkCommand(info, g),
		newWorklogCommand(info, g),
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
		all      bool
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
			search := jc.SearchIssues
			if all {
				search = jc.SearchIssuesAll
			}
			raw, err := search(cmd.Context(), buildIssueListJQL(project, status, assignee), limit)
			if err != nil {
				return err
			}
			return cli.RenderDecoded(cmd, g, raw, jira.Decode[jira.IssuePage],
				func(w io.Writer, page jira.IssuePage) {
					writeIssueList(w, page.Issues)
				})
		},
	}
	f := cmd.Flags()
	f.StringVar(&project, "project", "", "project key (required)")
	f.StringVar(&status, "status", "", "filter by status name")
	f.StringVar(&assignee, "assignee", "", "filter by assignee account id, or currentUser()")
	cli.AddPaginationFlags(cmd, &limit, &all, "issues")
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
			return cli.RenderDecoded(cmd, g, raw, jira.Decode[jira.Issue], writeIssue)
		},
	}
}

func newIssueCreateCommand(info appinfo.Info, g *cli.GlobalFlags) *cobra.Command {
	var (
		project, issueType, summary, description, assignee string
		fieldFlags                                         []string
	)
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a Jira issue",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if project == "" || issueType == "" || summary == "" {
				return apperr.InvalidInput("issue create requires --project, --type, and --summary")
			}
			fields := map[string]any{
				"project":   map[string]string{"key": project},
				"issuetype": map[string]string{"name": issueType},
			}
			if err := applyIssueFields(fields, summary, description, assignee, fieldFlags); err != nil {
				return err
			}
			jc, err := jiraClient(info, g)
			if err != nil {
				return err
			}
			raw, err := jc.CreateIssue(cmd.Context(), fields)
			if err != nil {
				return err
			}
			return cli.RenderDecoded(cmd, g, raw, jira.Decode[jira.Issue],
				func(w io.Writer, iss jira.Issue) {
					fmt.Fprintf(w, "created %s\n", iss.Key)
				})
		},
	}
	f := cmd.Flags()
	f.StringVar(&project, "project", "", "project key (required)")
	f.StringVar(&issueType, "type", "", "issue type name, e.g. Bug or Task (required)")
	f.StringVar(&summary, "summary", "", "issue summary (required)")
	f.StringVar(&description, "description", "", "issue description; plain text is wrapped as ADF")
	f.StringVar(&assignee, "assignee", "", "assignee account id")
	f.StringArrayVar(&fieldFlags, "field", nil,
		"set any field as name=value (repeatable; value parsed as JSON when valid)")
	return cmd
}

func newIssueEditCommand(info appinfo.Info, g *cli.GlobalFlags) *cobra.Command {
	var (
		summary, description, assignee string
		fieldFlags                     []string
	)
	cmd := &cobra.Command{
		Use:   "edit <issue>",
		Short: "Edit a Jira issue",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			fields := map[string]any{}
			if err := applyIssueFields(fields, summary, description, assignee, fieldFlags); err != nil {
				return err
			}
			if len(fields) == 0 {
				return apperr.InvalidInput(
					"issue edit requires at least one change; pass --summary, --description, --assignee, or --field")
			}
			jc, err := jiraClient(info, g)
			if err != nil {
				return err
			}
			if err := jc.EditIssue(cmd.Context(), args[0], fields); err != nil {
				return err
			}
			if g.WantsStructured() {
				return cli.Render(cmd, g, editResult{Key: args[0], Updated: true})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "updated %s\n", args[0])
			return nil
		},
	}
	f := cmd.Flags()
	f.StringVar(&summary, "summary", "", "new summary")
	f.StringVar(&description, "description", "", "new description; plain text is wrapped as ADF")
	f.StringVar(&assignee, "assignee", "", "new assignee account id")
	f.StringArrayVar(&fieldFlags, "field", nil,
		"set any field as name=value (repeatable; value parsed as JSON when valid)")
	return cmd
}

// editResult is the synthesized outcome of an `issue edit`, whose API call
// returns no body, so --json has a stable object to render.
type editResult struct {
	Key     string `json:"key"`
	Updated bool   `json:"updated"`
}

// applyIssueFields adds the typed common-field flags to fields when set, then
// overlays the repeatable --field escape entries (which can override them).
func applyIssueFields(fields map[string]any, summary, description, assignee string, fieldFlags []string) error {
	if summary != "" {
		fields["summary"] = summary
	}
	if description != "" {
		fields["description"] = jira.DocOf(description)
	}
	if assignee != "" {
		fields["assignee"] = map[string]string{"accountId": assignee}
	}
	extra, err := parseFieldFlags(fieldFlags)
	if err != nil {
		return err
	}
	for k, v := range extra {
		fields[k] = v
	}
	return nil
}

// parseFieldFlags turns repeatable --field name=value entries into a field
// map. A value is used as parsed JSON when it is valid JSON, otherwise it is
// kept as a plain string.
func parseFieldFlags(entries []string) (map[string]any, error) {
	out := map[string]any{}
	for _, e := range entries {
		name, value, ok := strings.Cut(e, "=")
		if !ok || name == "" {
			return nil, apperr.InvalidInput(
				fmt.Sprintf("invalid --field %q; expected name=value", e))
		}
		var parsed any
		if json.Unmarshal([]byte(value), &parsed) == nil {
			out[name] = parsed
		} else {
			out[name] = value
		}
	}
	return out, nil
}

func newIssueTransitionCommand(info appinfo.Info, g *cli.GlobalFlags) *cobra.Command {
	var to string
	cmd := &cobra.Command{
		Use:   "transition <issue>",
		Short: "List or apply workflow transitions on a Jira issue",
		Long: "With no --to flag, lists the transitions available from the issue's\n" +
			"current status. With --to, applies the matching transition.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			jc, err := jiraClient(info, g)
			if err != nil {
				return err
			}
			raw, err := jc.GetTransitions(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			if !cmd.Flags().Changed("to") {
				return cli.RenderDecoded(cmd, g, raw, jira.Decode[jira.TransitionList],
					func(w io.Writer, list jira.TransitionList) {
						writeTransitionList(w, list.Transitions)
					})
			}
			list, err := jira.Decode[jira.TransitionList](raw)
			if err != nil {
				return err
			}
			tr, err := resolveTransition(args[0], list.Transitions, to)
			if err != nil {
				return err
			}
			if err := jc.DoTransition(cmd.Context(), args[0], tr.ID); err != nil {
				return err
			}
			if g.WantsStructured() {
				return cli.Render(cmd, g, transitionResult{Key: args[0], Transition: tr.Name})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "transitioned %s to %q\n", args[0], tr.Name)
			return nil
		},
	}
	cmd.Flags().StringVar(&to, "to", "", "transition to apply, by name or id")
	return cmd
}

// transitionResult is the synthesized outcome of an applied transition, whose
// API call returns no body, so --json has a stable object to render.
type transitionResult struct {
	Key        string `json:"key"`
	Transition string `json:"transition"`
}

// resolveTransition finds the single transition matching to by id or
// (case-insensitive) name, reporting a structured error when none or several
// match.
func resolveTransition(issue string, transitions []jira.Transition, to string) (jira.Transition, error) {
	if len(transitions) == 0 {
		return jira.Transition{}, apperr.InvalidInput(
			"issue " + issue + " has no available transitions")
	}
	var matches []jira.Transition
	for _, tr := range transitions {
		if tr.ID == to || strings.EqualFold(tr.Name, to) {
			matches = append(matches, tr)
		}
	}
	switch len(matches) {
	case 1:
		return matches[0], nil
	case 0:
		return jira.Transition{}, apperr.InvalidInput(fmt.Sprintf(
			"no transition matches %q; available: %s", to, transitionNames(transitions)))
	default:
		return jira.Transition{}, apperr.InvalidInput(fmt.Sprintf(
			"%q is ambiguous; it matches %d transitions", to, len(matches)))
	}
}

// transitionNames joins transition names for an error message.
func transitionNames(transitions []jira.Transition) string {
	names := make([]string, len(transitions))
	for i, tr := range transitions {
		names[i] = tr.Name
	}
	return strings.Join(names, ", ")
}

// writeTransitionList prints transitions as aligned id/name rows.
func writeTransitionList(w io.Writer, transitions []jira.Transition) {
	if len(transitions) == 0 {
		fmt.Fprintln(w, "No transitions found.")
		return
	}
	tw := output.TabWriter(w)
	for _, tr := range transitions {
		fmt.Fprintf(tw, "%s\t%s\n", tr.ID, tr.Name)
	}
	_ = tw.Flush()
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
	tw := output.TabWriter(w)
	for _, iss := range issues {
		fmt.Fprintf(tw, "%s\t%s\t%s\n", iss.Key, namedOr(iss.Fields.Status, "-"), iss.Fields.Summary)
	}
	_ = tw.Flush()
}

// writeIssue prints a single issue as aligned label/value lines.
func writeIssue(w io.Writer, iss jira.Issue) {
	f := iss.Fields
	lw := output.NewLabelWriter(w)
	lw.Add("key", iss.Key)
	lw.Add("summary", f.Summary)
	if f.Status != nil {
		lw.Add("status", f.Status.Name)
	}
	if f.IssueType != nil {
		lw.Add("type", f.IssueType.Name)
	}
	if f.Priority != nil {
		lw.Add("priority", f.Priority.Name)
	}
	if f.Assignee != nil && f.Assignee.DisplayName != "" {
		lw.Add("assignee", f.Assignee.DisplayName)
	}
	if f.Reporter != nil && f.Reporter.DisplayName != "" {
		lw.Add("reporter", f.Reporter.DisplayName)
	}
	lw.AddIf("created", f.Created)
	lw.AddIf("updated", f.Updated)
	_ = lw.Flush()
}
