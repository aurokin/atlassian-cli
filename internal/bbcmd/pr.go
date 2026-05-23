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

// prStates lists the Bitbucket pull-request states accepted by --state, plus
// the synthetic "ALL" that lists every state.
var prStates = map[string]bool{
	"OPEN": true, "MERGED": true, "DECLINED": true, "SUPERSEDED": true, "ALL": true,
}

// normalizePRState upper-cases and validates a --state value, mapping "ALL" to
// the empty filter the client treats as "every state".
func normalizePRState(state string) (string, error) {
	s := strings.ToUpper(strings.TrimSpace(state))
	if s == "" || s == "ALL" {
		return "", nil
	}
	if !prStates[s] {
		return "", apperr.InvalidInput(fmt.Sprintf(
			"invalid --state %q; expected one of OPEN, MERGED, DECLINED, SUPERSEDED, ALL", state))
	}
	return s, nil
}

// newPRCommand builds the "pr" command group.
func newPRCommand(info appinfo.Info, g *cli.GlobalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "pr",
		Aliases: []string{"pull-request", "pullrequest"},
		Short:   "List, view, and create Bitbucket pull requests",
	}
	cmd.AddCommand(
		newPRListCommand(info, g),
		newPRViewCommand(info, g),
		newPRCreateCommand(info, g),
		newPRApproveCommand(info, g),
		newPRUnapproveCommand(info, g),
		newPRDeclineCommand(info, g),
		newPRMergeCommand(info, g),
	)
	return cmd
}

func newPRListCommand(info appinfo.Info, g *cli.GlobalFlags) *cobra.Command {
	var (
		repoFlag      string
		workspaceFlag string
		state         string
		limit         int
		all           bool
	)
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List a repository's pull requests",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			target, err := resolveRepoTarget(nil, repoFlag, workspaceFlag)
			if err != nil {
				return err
			}
			st, err := normalizePRState(state)
			if err != nil {
				return err
			}
			bc, err := bbClient(info, g)
			if err != nil {
				return err
			}
			list := bc.ListPullRequests
			if all {
				list = bc.ListPullRequestsAll
				limit = allPageSize(limit)
			}
			raw, err := list(cmd.Context(), target.Workspace, target.Repo, st, limit)
			if err != nil {
				return err
			}
			return cli.RenderDecoded(cmd, g, raw, bitbucket.Decode[bitbucket.PullRequestPage],
				func(w io.Writer, page bitbucket.PullRequestPage) {
					writePRList(w, page.Values)
				})
		},
	}
	addRepoFlags(cmd, &repoFlag, &workspaceFlag)
	f := cmd.Flags()
	f.StringVar(&state, "state", "OPEN", "filter by state: OPEN, MERGED, DECLINED, SUPERSEDED, or ALL")
	cli.AddPaginationFlags(cmd, &limit, &all, "pull requests")
	return cmd
}

func newPRViewCommand(info appinfo.Info, g *cli.GlobalFlags) *cobra.Command {
	var (
		repoFlag      string
		workspaceFlag string
	)
	cmd := &cobra.Command{
		Use:   "view <id>",
		Short: "View a single pull request",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parsePRID(args[0])
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
			raw, err := bc.GetPullRequest(cmd.Context(), target.Workspace, target.Repo, id)
			if err != nil {
				return err
			}
			return cli.RenderDecoded(cmd, g, raw, bitbucket.Decode[bitbucket.PullRequest], writePR)
		},
	}
	addRepoFlags(cmd, &repoFlag, &workspaceFlag)
	return cmd
}

func newPRCreateCommand(info appinfo.Info, g *cli.GlobalFlags) *cobra.Command {
	var (
		repoFlag      string
		workspaceFlag string
		opts          bitbucket.CreatePullRequestOptions
	)
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Open a new pull request",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if strings.TrimSpace(opts.Title) == "" {
				return apperr.InvalidInput("a title is required; pass --title")
			}
			if strings.TrimSpace(opts.SourceBranch) == "" {
				return apperr.InvalidInput("a source branch is required; pass --source")
			}
			if strings.TrimSpace(opts.DestinationBranch) == "" {
				return apperr.InvalidInput("a destination branch is required; pass --destination")
			}
			target, err := resolveRepoTarget(nil, repoFlag, workspaceFlag)
			if err != nil {
				return err
			}
			bc, err := bbClient(info, g)
			if err != nil {
				return err
			}
			raw, err := bc.CreatePullRequest(cmd.Context(), target.Workspace, target.Repo, opts)
			if err != nil {
				return err
			}
			return cli.RenderDecoded(cmd, g, raw, bitbucket.Decode[bitbucket.PullRequest],
				func(w io.Writer, pr bitbucket.PullRequest) {
					fmt.Fprintf(w, "created pull request #%d: %s\n", pr.ID, pr.Title)
				})
		},
	}
	addRepoFlags(cmd, &repoFlag, &workspaceFlag)
	f := cmd.Flags()
	f.StringVar(&opts.Title, "title", "", "pull request title (required)")
	f.StringVar(&opts.SourceBranch, "source", "", "source branch name (required)")
	f.StringVar(&opts.DestinationBranch, "destination", "", "destination branch name (required)")
	f.StringVar(&opts.Description, "description", "", "pull request description")
	f.BoolVar(&opts.Draft, "draft", false, "open the pull request as a draft")
	f.BoolVar(&opts.CloseSourceBranch, "close-source-branch", false, "close the source branch after merge")
	return cmd
}

// prActionPreamble resolves the inputs the pr action subcommands share: the
// parsed PR id, the resolved repo target, and an authenticated client.
func prActionPreamble(info appinfo.Info, g *cli.GlobalFlags, repoFlag, workspaceFlag, idArg string) (*bitbucket.Client, repoTarget, int, error) {
	id, err := parsePRID(idArg)
	if err != nil {
		return nil, repoTarget{}, 0, err
	}
	target, err := resolveRepoTarget(nil, repoFlag, workspaceFlag)
	if err != nil {
		return nil, repoTarget{}, 0, err
	}
	bc, err := bbClient(info, g)
	if err != nil {
		return nil, repoTarget{}, 0, err
	}
	return bc, target, id, nil
}

func newPRApproveCommand(info appinfo.Info, g *cli.GlobalFlags) *cobra.Command {
	var repoFlag, workspaceFlag string
	cmd := &cobra.Command{
		Use:   "approve <id>",
		Short: "Approve a pull request",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			bc, target, id, err := prActionPreamble(info, g, repoFlag, workspaceFlag, args[0])
			if err != nil {
				return err
			}
			raw, err := bc.ApprovePullRequest(cmd.Context(), target.Workspace, target.Repo, id)
			if err != nil {
				return err
			}
			if g.WantsStructured() {
				return cli.Render(cmd, g, raw)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "approved pull request #%d\n", id)
			return nil
		},
	}
	addRepoFlags(cmd, &repoFlag, &workspaceFlag)
	return cmd
}

func newPRUnapproveCommand(info appinfo.Info, g *cli.GlobalFlags) *cobra.Command {
	var repoFlag, workspaceFlag string
	cmd := &cobra.Command{
		Use:   "unapprove <id>",
		Short: "Withdraw your approval of a pull request",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			bc, target, id, err := prActionPreamble(info, g, repoFlag, workspaceFlag, args[0])
			if err != nil {
				return err
			}
			if err := bc.UnapprovePullRequest(cmd.Context(), target.Workspace, target.Repo, id); err != nil {
				return err
			}
			if g.WantsStructured() {
				return cli.Render(cmd, g, prActionResult{ID: id, Action: "unapprove", Done: true})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "removed approval from pull request #%d\n", id)
			return nil
		},
	}
	addRepoFlags(cmd, &repoFlag, &workspaceFlag)
	return cmd
}

func newPRDeclineCommand(info appinfo.Info, g *cli.GlobalFlags) *cobra.Command {
	var repoFlag, workspaceFlag string
	cmd := &cobra.Command{
		Use:   "decline <id>",
		Short: "Decline a pull request",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			bc, target, id, err := prActionPreamble(info, g, repoFlag, workspaceFlag, args[0])
			if err != nil {
				return err
			}
			raw, err := bc.DeclinePullRequest(cmd.Context(), target.Workspace, target.Repo, id)
			if err != nil {
				return err
			}
			if g.WantsStructured() {
				return cli.Render(cmd, g, raw)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "declined pull request #%d\n", id)
			return nil
		},
	}
	addRepoFlags(cmd, &repoFlag, &workspaceFlag)
	return cmd
}

func newPRMergeCommand(info appinfo.Info, g *cli.GlobalFlags) *cobra.Command {
	var (
		repoFlag, workspaceFlag string
		opts                    bitbucket.MergePullRequestOptions
	)
	cmd := &cobra.Command{
		Use:   "merge <id>",
		Short: "Merge a pull request",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			strategy, err := normalizeMergeStrategy(opts.Strategy)
			if err != nil {
				return err
			}
			opts.Strategy = strategy
			bc, target, id, err := prActionPreamble(info, g, repoFlag, workspaceFlag, args[0])
			if err != nil {
				return err
			}
			raw, err := bc.MergePullRequest(cmd.Context(), target.Workspace, target.Repo, id, opts)
			if err != nil {
				return err
			}
			if g.WantsStructured() {
				return cli.Render(cmd, g, raw)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "merged pull request #%d\n", id)
			return nil
		},
	}
	addRepoFlags(cmd, &repoFlag, &workspaceFlag)
	f := cmd.Flags()
	f.StringVar(&opts.Strategy, "strategy", "",
		"merge strategy: merge-commit, squash, or fast-forward (default: repository setting)")
	f.StringVar(&opts.Message, "message", "", "custom merge commit message")
	f.BoolVar(&opts.CloseSourceBranch, "close-source-branch", false, "close the source branch after merge")
	return cmd
}

// normalizeMergeStrategy maps the friendly --strategy values to the Bitbucket
// API's merge_strategy tokens. An empty value is left unset (the repository's
// default applies); any other value is rejected.
func normalizeMergeStrategy(strategy string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(strategy)) {
	case "":
		return "", nil
	case "merge-commit", "merge_commit":
		return "merge_commit", nil
	case "squash":
		return "squash", nil
	case "fast-forward", "fast_forward":
		return "fast_forward", nil
	default:
		return "", apperr.InvalidInput(fmt.Sprintf(
			"invalid --strategy %q; expected merge-commit, squash, or fast-forward", strategy))
	}
}

// prActionResult is the synthesized outcome of a pull-request action whose API
// call returns no body, so --json has a stable object to render.
type prActionResult struct {
	ID     int    `json:"id"`
	Action string `json:"action"`
	Done   bool   `json:"done"`
}

// parsePRID parses a positive pull-request id.
func parsePRID(s string) (int, error) {
	id, err := parsePositiveInt(s)
	if err != nil {
		return 0, apperr.InvalidInput(fmt.Sprintf("invalid pull request id %q; expected a positive integer", s))
	}
	return id, nil
}

// writePRList prints pull requests as aligned id/state/title rows.
func writePRList(w io.Writer, prs []bitbucket.PullRequest) {
	if len(prs) == 0 {
		fmt.Fprintln(w, "No pull requests found.")
		return
	}
	tw := output.TabWriter(w)
	for _, pr := range prs {
		fmt.Fprintf(tw, "#%d\t%s\t%s\n", pr.ID, pr.State, pr.Title)
	}
	_ = tw.Flush()
}

// writePR prints a single pull request as aligned label/value lines.
func writePR(w io.Writer, pr bitbucket.PullRequest) {
	lw := output.NewLabelWriter(w)
	lw.Addf("id", "#%d", pr.ID)
	lw.Add("title", pr.Title)
	lw.AddIf("state", pr.State)
	lw.AddIf("author", accountLabel(pr.Author))
	lw.AddIf("branches", branchFlow(pr.Source, pr.Destination))
	_ = lw.Flush()
}

// accountLabel renders an account's display name (or nickname) for human
// output, or "" when neither is present.
func accountLabel(a *bitbucket.Account) string {
	if a == nil {
		return ""
	}
	if a.DisplayName != "" {
		return a.DisplayName
	}
	return a.Nickname
}

// branchFlow renders "source → destination" when both branch names are known.
func branchFlow(source, destination *bitbucket.PullRequestRef) string {
	src := refBranch(source)
	dst := refBranch(destination)
	switch {
	case src != "" && dst != "":
		return src + " → " + dst
	case src != "":
		return src
	default:
		return dst
	}
}

func refBranch(ref *bitbucket.PullRequestRef) string {
	if ref == nil || ref.Branch == nil {
		return ""
	}
	return ref.Branch.Name
}
