package jiracmd

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

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
	)
	return cmd
}

func newCommentListCommand(info appinfo.Info, g *cli.GlobalFlags) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:   "list <issue>",
		Short: "List comments on an issue",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			jc, err := jiraClient(info, g)
			if err != nil {
				return err
			}
			raw, err := jc.ListComments(cmd.Context(), args[0], limit)
			if err != nil {
				return err
			}
			if g.JSON != "" || g.JQ != "" {
				return cli.Render(cmd, g, raw)
			}
			page, err := jira.Decode[jira.CommentPage](raw)
			if err != nil {
				return err
			}
			writeCommentList(cmd.OutOrStdout(), page.Comments)
			return nil
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 0, "maximum number of comments to return")
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
			if g.JSON != "" || g.JQ != "" {
				return cli.Render(cmd, g, raw)
			}
			c, err := jira.Decode[jira.Comment](raw)
			if err != nil {
				return err
			}
			writeComment(cmd.OutOrStdout(), c)
			return nil
		},
	}
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
