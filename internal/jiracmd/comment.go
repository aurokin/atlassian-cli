package jiracmd

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/aurokin/atlassian-cli/internal/apperr"
	"github.com/aurokin/atlassian-cli/internal/appinfo"
	"github.com/aurokin/atlassian-cli/internal/cli"
	"github.com/aurokin/atlassian-cli/internal/jira"
)

// newCommentCommand builds the "issue comment" command group.
func newCommentCommand(info appinfo.Info, g *cli.GlobalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "comment",
		Short: "List and view comments on a Jira issue",
	}
	cmd.AddCommand(
		newCommentListCommand(info, g),
		newCommentViewCommand(info, g),
		newCommentCreateCommand(info, g),
		newCommentEditCommand(info, g),
		newCommentDeleteCommand(info, g),
	)
	return cmd
}

func newCommentListCommand(info appinfo.Info, g *cli.GlobalFlags) *cobra.Command {
	var (
		limit int
		all   bool
	)
	cmd := &cobra.Command{
		Use:   "list <issue>",
		Short: "List comments on an issue",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			jc, err := jiraClient(info, g)
			if err != nil {
				return err
			}
			list := jc.ListComments
			if all {
				list = jc.ListCommentsAll
			}
			raw, err := list(cmd.Context(), args[0], limit)
			if err != nil {
				return err
			}
			return cli.RenderDecoded(cmd, g, raw, jira.Decode[jira.CommentPage],
				func(w io.Writer, page jira.CommentPage) {
					writeCommentList(w, page.Comments)
				})
		},
	}
	cli.AddPaginationFlags(cmd, &limit, &all, "comments")
	return cmd
}

func newCommentViewCommand(info appinfo.Info, g *cli.GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "view <issue> <comment-id>",
		Short: "View a single comment on an issue",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			jc, err := jiraClient(info, g)
			if err != nil {
				return err
			}
			raw, err := jc.GetComment(cmd.Context(), args[0], args[1])
			if err != nil {
				return err
			}
			return cli.RenderDecoded(cmd, g, raw, jira.Decode[jira.Comment], writeComment)
		},
	}
}

func newCommentCreateCommand(info appinfo.Info, g *cli.GlobalFlags) *cobra.Command {
	var body string
	cmd := &cobra.Command{
		Use:   "create <issue>",
		Short: "Add a comment to an issue",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if body == "" {
				return apperr.InvalidInput("issue comment create requires --body")
			}
			jc, err := jiraClient(info, g)
			if err != nil {
				return err
			}
			raw, err := jc.CreateComment(cmd.Context(), args[0], jira.DocOf(body))
			if err != nil {
				return err
			}
			return cli.RenderDecoded(cmd, g, raw, jira.Decode[jira.Comment],
				func(w io.Writer, c jira.Comment) {
					fmt.Fprintf(w, "created comment %s on %s\n", c.ID, args[0])
				})
		},
	}
	cmd.Flags().StringVar(&body, "body", "", "comment body; plain text is wrapped as ADF (required)")
	return cmd
}

func newCommentEditCommand(info appinfo.Info, g *cli.GlobalFlags) *cobra.Command {
	var body string
	cmd := &cobra.Command{
		Use:   "edit <issue> <comment-id>",
		Short: "Replace the body of a comment",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if body == "" {
				return apperr.InvalidInput("issue comment edit requires --body")
			}
			jc, err := jiraClient(info, g)
			if err != nil {
				return err
			}
			raw, err := jc.EditComment(cmd.Context(), args[0], args[1], jira.DocOf(body))
			if err != nil {
				return err
			}
			if g.WantsStructured() {
				return cli.Render(cmd, g, raw)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "updated comment %s on %s\n", args[1], args[0])
			return nil
		},
	}
	cmd.Flags().StringVar(&body, "body", "", "new comment body; plain text is wrapped as ADF (required)")
	return cmd
}

func newCommentDeleteCommand(info appinfo.Info, g *cli.GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "delete <issue> <comment-id>",
		Short: "Delete a comment from an issue",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			jc, err := jiraClient(info, g)
			if err != nil {
				return err
			}
			if err := jc.DeleteComment(cmd.Context(), args[0], args[1]); err != nil {
				return err
			}
			if g.WantsStructured() {
				return cli.Render(cmd, g, commentDeleteResult{
					Issue: args[0], Comment: args[1], Deleted: true})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "deleted comment %s on %s\n", args[1], args[0])
			return nil
		},
	}
}

// commentDeleteResult is the synthesized outcome of a comment deletion, whose
// API call returns no body, so --json has a stable object to render.
type commentDeleteResult struct {
	Issue   string `json:"issue"`
	Comment string `json:"comment"`
	Deleted bool   `json:"deleted"`
}

// writeCommentList prints each comment, separated by a blank line.
func writeCommentList(w io.Writer, comments []jira.Comment) {
	if len(comments) == 0 {
		fmt.Fprintln(w, "No comments found.")
		return
	}
	for i, c := range comments {
		if i > 0 {
			fmt.Fprintln(w)
		}
		writeComment(w, c)
	}
}

// writeComment prints a single comment as label/value lines followed by its
// plain-text body.
func writeComment(w io.Writer, c jira.Comment) {
	author := "-"
	if c.Author != nil && c.Author.DisplayName != "" {
		author = c.Author.DisplayName
	}
	fmt.Fprintf(w, "%-9s %s\n", "id:", c.ID)
	fmt.Fprintf(w, "%-9s %s\n", "author:", author)
	if c.Created != "" {
		fmt.Fprintf(w, "%-9s %s\n", "created:", c.Created)
	}
	if c.Updated != "" && c.Updated != c.Created {
		fmt.Fprintf(w, "%-9s %s\n", "updated:", c.Updated)
	}
	if text := jira.TextOf(c.Body); text != "" {
		fmt.Fprintf(w, "\n%s\n", text)
	}
}
