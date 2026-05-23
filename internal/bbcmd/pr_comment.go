package bbcmd

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/aurokin/atlassian-cli/internal/apperr"
	"github.com/aurokin/atlassian-cli/internal/appinfo"
	"github.com/aurokin/atlassian-cli/internal/bitbucket"
	"github.com/aurokin/atlassian-cli/internal/cli"
	"github.com/aurokin/atlassian-cli/internal/output"
)

func newPRDiffCommand(info appinfo.Info, g *cli.GlobalFlags) *cobra.Command {
	var repoFlag, workspaceFlag string
	cmd := &cobra.Command{
		Use:   "diff <id>",
		Short: "Show a pull request's unified diff",
		Long: "Writes the pull request's unified diff to stdout verbatim. The diff is\n" +
			"raw text, so --json/--jq do not apply.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			bc, target, id, err := prActionPreamble(info, g, repoFlag, workspaceFlag, args[0])
			if err != nil {
				return err
			}
			data, err := bc.GetPullRequestDiff(cmd.Context(), target.Workspace, target.Repo, id)
			if err != nil {
				return err
			}
			_, err = cmd.OutOrStdout().Write(data)
			return err
		},
	}
	addRepoFlags(cmd, &repoFlag, &workspaceFlag)
	return cmd
}

// newPRCommentsCommand builds the "pr comments" command group.
func newPRCommentsCommand(info appinfo.Info, g *cli.GlobalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "comments",
		Short: "List and add pull-request comments",
	}
	cmd.AddCommand(
		newPRCommentsListCommand(info, g),
		newPRCommentsAddCommand(info, g),
	)
	return cmd
}

func newPRCommentsListCommand(info appinfo.Info, g *cli.GlobalFlags) *cobra.Command {
	var (
		repoFlag, workspaceFlag string
		limit                   int
		all                     bool
	)
	cmd := &cobra.Command{
		Use:   "list <id>",
		Short: "List a pull request's comments",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			bc, target, id, err := prActionPreamble(info, g, repoFlag, workspaceFlag, args[0])
			if err != nil {
				return err
			}
			list := bc.ListPullRequestComments
			if all {
				list = bc.ListPullRequestCommentsAll
				limit = allPageSize(limit)
			}
			raw, err := list(cmd.Context(), target.Workspace, target.Repo, id, limit)
			if err != nil {
				return err
			}
			return cli.RenderDecoded(cmd, g, raw, bitbucket.Decode[bitbucket.PullRequestCommentPage],
				func(w io.Writer, page bitbucket.PullRequestCommentPage) {
					writePRCommentList(w, page.Values)
				})
		},
	}
	addRepoFlags(cmd, &repoFlag, &workspaceFlag)
	cli.AddPaginationFlags(cmd, &limit, &all, "comments")
	return cmd
}

func newPRCommentsAddCommand(info appinfo.Info, g *cli.GlobalFlags) *cobra.Command {
	var (
		repoFlag, workspaceFlag string
		body                    string
	)
	cmd := &cobra.Command{
		Use:   "add <id>",
		Short: "Add a comment to a pull request",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if body == "" {
				return apperr.InvalidInput("pr comments add requires --body")
			}
			bc, target, id, err := prActionPreamble(info, g, repoFlag, workspaceFlag, args[0])
			if err != nil {
				return err
			}
			raw, err := bc.AddPullRequestComment(cmd.Context(), target.Workspace, target.Repo, id, body)
			if err != nil {
				return err
			}
			return cli.RenderDecoded(cmd, g, raw, bitbucket.Decode[bitbucket.PullRequestComment],
				func(w io.Writer, c bitbucket.PullRequestComment) {
					fmt.Fprintf(w, "added comment %d to pull request #%d\n", c.ID, id)
				})
		},
	}
	addRepoFlags(cmd, &repoFlag, &workspaceFlag)
	cmd.Flags().StringVar(&body, "body", "", "comment body (required)")
	return cmd
}

// writePRCommentList prints each comment as a label/value block followed by its
// raw body, separated by blank lines.
func writePRCommentList(w io.Writer, comments []bitbucket.PullRequestComment) {
	if len(comments) == 0 {
		fmt.Fprintln(w, "No comments found.")
		return
	}
	for i, c := range comments {
		if i > 0 {
			fmt.Fprintln(w)
		}
		writePRComment(w, c)
	}
}

// writePRComment prints a single comment as label/value lines followed by its
// raw body, if any.
func writePRComment(w io.Writer, c bitbucket.PullRequestComment) {
	lw := output.NewLabelWriter(w)
	lw.Addf("id", "%d", c.ID)
	lw.AddIf("author", accountLabel(c.User))
	lw.AddIf("created", c.CreatedOn)
	if c.Deleted {
		lw.Add("deleted", "true")
	}
	_ = lw.Flush()
	if c.Content != nil && c.Content.Raw != "" {
		fmt.Fprintf(w, "\n%s\n", c.Content.Raw)
	}
}
