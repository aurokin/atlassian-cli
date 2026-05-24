package confcmd

import (
	"context"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/aurokin/atlassian-cli/internal/apperr"
	"github.com/aurokin/atlassian-cli/internal/appinfo"
	"github.com/aurokin/atlassian-cli/internal/cli"
	"github.com/aurokin/atlassian-cli/internal/conf"
	"github.com/aurokin/atlassian-cli/internal/output"
)

// newBlogpostCommand builds the "blogpost" command group.
func newBlogpostCommand(info appinfo.Info, g *cli.GlobalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "blogpost",
		Aliases: []string{"blogposts", "blog"},
		Short:   "List, view, create, and edit Confluence blogposts",
	}
	cmd.AddCommand(
		newBlogpostListCommand(info, g),
		newBlogpostViewCommand(info, g),
		newBlogpostCreateCommand(info, g),
		newBlogpostEditCommand(info, g),
	)
	return cmd
}

func newBlogpostListCommand(info appinfo.Info, g *cli.GlobalFlags) *cobra.Command {
	var (
		space string
		limit int
		all   bool
	)
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List blogposts, optionally filtered to a space",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cc, err := confClient(info, g)
			if err != nil {
				return err
			}
			// --space is optional; when given, resolve its key to an id.
			spaceID := ""
			if space != "" {
				sp, err := resolveSpace(cmd.Context(), cc, space)
				if err != nil {
					return err
				}
				spaceID = sp.ID
			}
			list := cc.ListBlogposts
			if all {
				list = cc.ListBlogpostsAll
			}
			raw, err := list(cmd.Context(), spaceID, limit)
			if err != nil {
				return err
			}
			return cli.RenderDecoded(cmd, g, raw, conf.Decode[conf.BlogpostList],
				func(w io.Writer, list conf.BlogpostList) {
					writeBlogpostList(w, list.Results)
				})
		},
	}
	f := cmd.Flags()
	f.StringVar(&space, "space", "", "space key to filter by (optional)")
	cli.AddPaginationFlags(cmd, &limit, &all, "blogposts")
	return cmd
}

func newBlogpostViewCommand(info appinfo.Info, g *cli.GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "view <id>",
		Short: "View a single Confluence blogpost",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cc, err := confClient(info, g)
			if err != nil {
				return err
			}
			raw, err := cc.GetBlogpost(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			return cli.RenderDecoded(cmd, g, raw, conf.Decode[conf.Blogpost], writeBlogpost)
		},
	}
}

func newBlogpostCreateCommand(info appinfo.Info, g *cli.GlobalFlags) *cobra.Command {
	var space, title, body, bodyFormat string
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a Confluence blogpost",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if space == "" || title == "" || body == "" || bodyFormat == "" {
				return apperr.InvalidInput(
					"blogpost create requires --space, --title, --body, and --body-format")
			}
			if err := validateBodyFormat(bodyFormat); err != nil {
				return err
			}
			cc, err := confClient(info, g)
			if err != nil {
				return err
			}
			sp, err := resolveSpace(cmd.Context(), cc, space)
			if err != nil {
				return err
			}
			raw, err := cc.CreateBlogpost(cmd.Context(), sp.ID, title, bodyFormat, body)
			if err != nil {
				return err
			}
			return cli.RenderDecoded(cmd, g, raw, conf.Decode[conf.Blogpost],
				func(w io.Writer, b conf.Blogpost) {
					fmt.Fprintf(w, "created blogpost %s\n", b.ID)
				})
		},
	}
	f := cmd.Flags()
	f.StringVar(&space, "space", "", "space key (required)")
	f.StringVar(&title, "title", "", "blogpost title (required)")
	f.StringVar(&body, "body", "", "blogpost body, sent verbatim (required)")
	f.StringVar(&bodyFormat, "body-format", "",
		"body representation: storage, atlas_doc_format, or wiki (required)")
	_ = cmd.RegisterFlagCompletionFunc("body-format", cli.FixedCompletion(bodyFormatValues...))
	return cmd
}

func newBlogpostEditCommand(info appinfo.Info, g *cli.GlobalFlags) *cobra.Command {
	var title, body, bodyFormat string
	cmd := &cobra.Command{
		Use:   "edit <id>",
		Short: "Edit a Confluence blogpost",
		Long: "Edits a blogpost's title and/or body. Confluence treats an update as\n" +
			"a full replacement, so a title-only edit re-sends the blogpost's\n" +
			"current body: storage representation for classic blogposts, falling\n" +
			"back to atlas_doc_format for those authored in the modern editor.\n" +
			"Pass --body with --body-format to replace the body instead.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			titleSet, bodySet := cmd.Flags().Changed("title"), cmd.Flags().Changed("body")
			if err := validateContentEditFlags("blogpost", titleSet, bodySet, bodyFormat); err != nil {
				return err
			}
			cc, err := confClient(info, g)
			if err != nil {
				return err
			}
			ops := contentEditOps{
				noun: "blogpost",
				get:  cc.GetBlogpost,
				getADF: func(ctx context.Context, id string) (string, error) {
					return adfBodyFrom(ctx, cc.GetBlogpostWithFormat, id)
				},
				update: cc.UpdateBlogpost,
			}
			return runContentEdit(cmd, g, ops, args[0], titleSet, title, bodySet, body, bodyFormat)
		},
	}
	f := cmd.Flags()
	f.StringVar(&title, "title", "", "new blogpost title")
	f.StringVar(&body, "body", "", "new blogpost body, sent verbatim")
	f.StringVar(&bodyFormat, "body-format", "",
		"body representation for --body: storage, atlas_doc_format, or wiki")
	_ = cmd.RegisterFlagCompletionFunc("body-format", cli.FixedCompletion(bodyFormatValues...))
	return cmd
}

// writeBlogpostList prints blogposts as aligned id/status/title rows.
func writeBlogpostList(w io.Writer, posts []conf.Blogpost) {
	if len(posts) == 0 {
		fmt.Fprintln(w, "No blogposts found.")
		return
	}
	tw := output.TabWriter(w)
	for _, b := range posts {
		fmt.Fprintf(tw, "%s\t%s\t%s\n", b.ID, b.Status, b.Title)
	}
	_ = tw.Flush()
}

// writeBlogpost prints a single blogpost as aligned label/value lines.
func writeBlogpost(w io.Writer, b conf.Blogpost) {
	lw := output.NewLabelWriter(w)
	lw.Add("id", b.ID)
	lw.Add("title", b.Title)
	lw.AddIf("status", b.Status)
	lw.AddIf("space-id", b.SpaceID)
	if b.Version.Number > 0 {
		lw.Addf("version", "%d", b.Version.Number)
	}
	_ = lw.Flush()
}
