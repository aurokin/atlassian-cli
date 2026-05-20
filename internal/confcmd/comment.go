package confcmd

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/aurokin/atlassian-cli/internal/apperr"
	"github.com/aurokin/atlassian-cli/internal/appinfo"
	"github.com/aurokin/atlassian-cli/internal/cli"
	"github.com/aurokin/atlassian-cli/internal/conf"
	"github.com/aurokin/atlassian-cli/internal/output"
)

// newPageCommentCommand builds the "page comment" sub-group, which operates on
// a page's footer comments.
func newPageCommentCommand(info appinfo.Info, g *cli.GlobalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "comment",
		Short: "List and manage a page's footer comments",
	}
	cmd.AddCommand(
		newPageCommentListCommand(info, g),
		newPageCommentViewCommand(info, g),
		newPageCommentCreateCommand(info, g),
		newPageCommentEditCommand(info, g),
		newPageCommentDeleteCommand(info, g),
	)
	return cmd
}

func newPageCommentListCommand(info appinfo.Info, g *cli.GlobalFlags) *cobra.Command {
	var (
		limit int
		all   bool
	)
	cmd := &cobra.Command{
		Use:   "list <page-id>",
		Short: "List the footer comments on a page",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cc, err := confClient(info, g)
			if err != nil {
				return err
			}
			list := cc.ListFooterComments
			if all {
				list = cc.ListFooterCommentsAll
			}
			raw, err := list(cmd.Context(), args[0], limit)
			if err != nil {
				return err
			}
			if g.JSON != "" || g.JQ != "" {
				return cli.Render(cmd, g, raw)
			}
			cl, err := conf.Decode[conf.CommentList](raw)
			if err != nil {
				return err
			}
			writeCommentList(cmd.OutOrStdout(), cl.Results)
			return nil
		},
	}
	f := cmd.Flags()
	f.IntVar(&limit, "limit", 0, "maximum number of comments to return")
	f.BoolVar(&all, "all", false, "follow pagination and return every page (--limit sets the page size)")
	return cmd
}

func newPageCommentViewCommand(info appinfo.Info, g *cli.GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "view <comment-id>",
		Short: "View a single footer comment",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cc, err := confClient(info, g)
			if err != nil {
				return err
			}
			raw, err := cc.GetFooterComment(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			if g.JSON != "" || g.JQ != "" {
				return cli.Render(cmd, g, raw)
			}
			c, err := conf.Decode[conf.Comment](raw)
			if err != nil {
				return err
			}
			writeComment(cmd.OutOrStdout(), c)
			return nil
		},
	}
}

func newPageCommentCreateCommand(info appinfo.Info, g *cli.GlobalFlags) *cobra.Command {
	var body, bodyFormat string
	cmd := &cobra.Command{
		Use:   "create <page-id>",
		Short: "Add a footer comment to a page",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if body == "" || bodyFormat == "" {
				return apperr.InvalidInput(
					"page comment create requires --body and --body-format")
			}
			if err := validateBodyFormat(bodyFormat); err != nil {
				return err
			}
			cc, err := confClient(info, g)
			if err != nil {
				return err
			}
			raw, err := cc.CreateFooterComment(cmd.Context(), args[0], bodyFormat, body)
			if err != nil {
				return err
			}
			if g.JSON != "" || g.JQ != "" {
				return cli.Render(cmd, g, raw)
			}
			c, err := conf.Decode[conf.Comment](raw)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "created comment %s\n", c.ID)
			return nil
		},
	}
	f := cmd.Flags()
	f.StringVar(&body, "body", "", "comment body, sent verbatim (required)")
	f.StringVar(&bodyFormat, "body-format", "",
		"body representation: storage, atlas_doc_format, or wiki (required)")
	return cmd
}

func newPageCommentEditCommand(info appinfo.Info, g *cli.GlobalFlags) *cobra.Command {
	var body, bodyFormat string
	cmd := &cobra.Command{
		Use:   "edit <comment-id>",
		Short: "Replace a footer comment's body",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if body == "" || bodyFormat == "" {
				return apperr.InvalidInput(
					"page comment edit requires --body and --body-format")
			}
			if err := validateBodyFormat(bodyFormat); err != nil {
				return err
			}
			cc, err := confClient(info, g)
			if err != nil {
				return err
			}
			// Confluence treats the update as a full replacement; fetch the
			// current comment for its version number.
			raw, err := cc.GetFooterComment(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			cur, err := conf.Decode[conf.Comment](raw)
			if err != nil {
				return err
			}
			updated, err := cc.UpdateFooterComment(cmd.Context(), args[0],
				bodyFormat, body, cur.Version.Number+1)
			if err != nil {
				return err
			}
			if g.JSON != "" || g.JQ != "" {
				return cli.Render(cmd, g, updated)
			}
			c, err := conf.Decode[conf.Comment](updated)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "updated comment %s to version %d\n", c.ID, c.Version.Number)
			return nil
		},
	}
	f := cmd.Flags()
	f.StringVar(&body, "body", "", "new comment body, sent verbatim (required)")
	f.StringVar(&bodyFormat, "body-format", "",
		"body representation: storage, atlas_doc_format, or wiki (required)")
	return cmd
}

func newPageCommentDeleteCommand(info appinfo.Info, g *cli.GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "delete <comment-id>",
		Short: "Delete a footer comment",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cc, err := confClient(info, g)
			if err != nil {
				return err
			}
			if err := cc.DeleteFooterComment(cmd.Context(), args[0]); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "deleted comment %s\n", args[0])
			return nil
		},
	}
}

// writeCommentList prints footer comments as aligned id/status/title rows.
func writeCommentList(w io.Writer, comments []conf.Comment) {
	if len(comments) == 0 {
		fmt.Fprintln(w, "No comments found.")
		return
	}
	tw := output.TabWriter(w)
	for _, c := range comments {
		fmt.Fprintf(tw, "%s\t%s\t%s\n", c.ID, c.Status, c.Title)
	}
	_ = tw.Flush()
}

// writeComment prints a single footer comment as label/value lines followed by
// its storage-format body.
func writeComment(w io.Writer, c conf.Comment) {
	fmt.Fprintf(w, "ID:       %s\n", c.ID)
	if c.Title != "" {
		fmt.Fprintf(w, "Title:    %s\n", c.Title)
	}
	fmt.Fprintf(w, "Status:   %s\n", c.Status)
	fmt.Fprintf(w, "Page:     %s\n", c.PageID)
	fmt.Fprintf(w, "Version:  %d\n", c.Version.Number)
	if c.Body.Storage.Value != "" {
		fmt.Fprintf(w, "\n%s\n", c.Body.Storage.Value)
	}
}
