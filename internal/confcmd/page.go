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

// newPageCommand builds the "page" command group.
func newPageCommand(info appinfo.Info, g *cli.GlobalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "page",
		Short: "List and view Confluence pages",
	}
	cmd.AddCommand(
		newPageListCommand(info, g),
		newPageViewCommand(info, g),
		newPageChildrenCommand(info, g),
		newPageCreateCommand(info, g),
		newPageEditCommand(info, g),
		newPageCommentCommand(info, g),
		newPageLabelCommand(info, g),
	)
	return cmd
}

func newPageListCommand(info appinfo.Info, g *cli.GlobalFlags) *cobra.Command {
	var (
		space string
		limit int
		all   bool
	)
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List pages in a space",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if space == "" {
				return apperr.InvalidInput("a space key is required; pass --space")
			}
			cc, err := confClient(info, g)
			if err != nil {
				return err
			}
			sp, err := resolveSpace(cmd.Context(), cc, space)
			if err != nil {
				return err
			}
			list := cc.ListPages
			if all {
				list = cc.ListPagesAll
			}
			raw, err := list(cmd.Context(), sp.ID, limit)
			if err != nil {
				return err
			}
			return cli.RenderDecoded(cmd, g, raw, conf.Decode[conf.PageList],
				func(w io.Writer, page conf.PageList) {
					writePageList(w, page.Results)
				})
		},
	}
	f := cmd.Flags()
	f.StringVar(&space, "space", "", "space key (required)")
	cli.AddPaginationFlags(cmd, &limit, &all, "pages")
	return cmd
}

func newPageViewCommand(info appinfo.Info, g *cli.GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "view <id>",
		Short: "View a single Confluence page",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cc, err := confClient(info, g)
			if err != nil {
				return err
			}
			raw, err := cc.GetPage(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			return cli.RenderDecoded(cmd, g, raw, conf.Decode[conf.Page], writePage)
		},
	}
}

func newPageChildrenCommand(info appinfo.Info, g *cli.GlobalFlags) *cobra.Command {
	var (
		limit int
		all   bool
	)
	cmd := &cobra.Command{
		Use:   "children <id>",
		Short: "List the direct child pages of a page",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cc, err := confClient(info, g)
			if err != nil {
				return err
			}
			children := cc.GetChildPages
			if all {
				children = cc.GetChildPagesAll
			}
			raw, err := children(cmd.Context(), args[0], limit)
			if err != nil {
				return err
			}
			return cli.RenderDecoded(cmd, g, raw, conf.Decode[conf.PageList],
				func(w io.Writer, page conf.PageList) {
					writePageList(w, page.Results)
				})
		},
	}
	cli.AddPaginationFlags(cmd, &limit, &all, "child pages")
	return cmd
}

func newPageCreateCommand(info appinfo.Info, g *cli.GlobalFlags) *cobra.Command {
	var space, title, body, bodyFormat string
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a Confluence page",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if space == "" || title == "" || body == "" || bodyFormat == "" {
				return apperr.InvalidInput(
					"page create requires --space, --title, --body, and --body-format")
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
			raw, err := cc.CreatePage(cmd.Context(), sp.ID, title, bodyFormat, body)
			if err != nil {
				return err
			}
			return cli.RenderDecoded(cmd, g, raw, conf.Decode[conf.Page],
				func(w io.Writer, p conf.Page) {
					fmt.Fprintf(w, "created page %s\n", p.ID)
				})
		},
	}
	f := cmd.Flags()
	f.StringVar(&space, "space", "", "space key (required)")
	f.StringVar(&title, "title", "", "page title (required)")
	f.StringVar(&body, "body", "", "page body, sent verbatim (required)")
	f.StringVar(&bodyFormat, "body-format", "",
		"body representation: storage, atlas_doc_format, or wiki (required)")
	return cmd
}

func newPageEditCommand(info appinfo.Info, g *cli.GlobalFlags) *cobra.Command {
	var title, body, bodyFormat string
	cmd := &cobra.Command{
		Use:   "edit <id>",
		Short: "Edit a Confluence page",
		Long: "Edits a page's title and/or body. Confluence treats an update as a\n" +
			"full replacement, so a title-only edit re-sends the page's current\n" +
			"body: storage representation for classic pages, falling back to\n" +
			"atlas_doc_format for pages authored in the modern editor. Pass --body\n" +
			"with --body-format to replace the body instead.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			titleSet := cmd.Flags().Changed("title")
			bodySet := cmd.Flags().Changed("body")
			if !titleSet && !bodySet {
				return apperr.InvalidInput(
					"page edit requires at least one change; pass --title or --body")
			}
			if bodySet && bodyFormat == "" {
				return apperr.InvalidInput("--body requires --body-format")
			}
			if !bodySet && bodyFormat != "" {
				return apperr.InvalidInput("--body-format is only valid together with --body")
			}
			if bodySet {
				if err := validateBodyFormat(bodyFormat); err != nil {
					return err
				}
			}
			cc, err := confClient(info, g)
			if err != nil {
				return err
			}
			raw, err := cc.GetPage(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			cur, err := conf.Decode[conf.Page](raw)
			if err != nil {
				return err
			}
			newTitle := cur.Title
			if titleSet {
				newTitle = title
			}
			newFormat, newBody := bodyFormat, body
			if !bodySet {
				// A title-only edit must re-send the existing body, since a v2
				// update replaces the whole page. Pages authored in the modern
				// editor store their body as atlas_doc_format and have an empty
				// storage representation, so fall back to a second GET for that
				// representation before giving up.
				switch {
				case cur.Body.Storage.Value != "":
					newFormat, newBody = "storage", cur.Body.Storage.Value
				default:
					adf, err := fetchADFBody(cmd.Context(), cc, args[0])
					if err != nil {
						return err
					}
					if adf == "" {
						return apperr.InvalidInput(fmt.Sprintf(
							"page %s has no storage or atlas_doc_format body to preserve; "+
								"pass --body with --body-format to set the body explicitly", args[0]))
					}
					newFormat, newBody = "atlas_doc_format", adf
				}
			}
			updated, err := cc.UpdatePage(cmd.Context(), args[0],
				cur.Status, newTitle, newFormat, newBody, cur.Version.Number+1)
			if err != nil {
				return err
			}
			return cli.RenderDecoded(cmd, g, updated, conf.Decode[conf.Page],
				func(w io.Writer, p conf.Page) {
					fmt.Fprintf(w, "updated page %s to version %d\n", p.ID, p.Version.Number)
				})
		},
	}
	f := cmd.Flags()
	f.StringVar(&title, "title", "", "new page title")
	f.StringVar(&body, "body", "", "new page body, sent verbatim")
	f.StringVar(&bodyFormat, "body-format", "",
		"body representation for --body: storage, atlas_doc_format, or wiki")
	return cmd
}

// fetchADFBody re-fetches a page in the atlas_doc_format representation and
// returns its body value (empty if the page has none). It backs the title-only
// edit fallback for modern-editor pages, whose storage representation is empty.
func fetchADFBody(ctx context.Context, cc *conf.Client, id string) (string, error) {
	raw, err := cc.GetPageWithFormat(ctx, id, "atlas_doc_format")
	if err != nil {
		return "", err
	}
	p, err := conf.Decode[conf.Page](raw)
	if err != nil {
		return "", err
	}
	return p.Body.AtlasDocFormat.Value, nil
}

// validateBodyFormat checks a --body-format value against the body
// representations the Confluence v2 write API accepts.
func validateBodyFormat(format string) error {
	switch format {
	case "storage", "atlas_doc_format", "wiki":
		return nil
	default:
		return apperr.InvalidInput(fmt.Sprintf(
			"invalid --body-format %q; expected storage, atlas_doc_format, or wiki", format))
	}
}

// writePageList prints pages as aligned id/status/title rows.
func writePageList(w io.Writer, pages []conf.Page) {
	if len(pages) == 0 {
		fmt.Fprintln(w, "No pages found.")
		return
	}
	tw := output.TabWriter(w)
	for _, p := range pages {
		fmt.Fprintf(tw, "%s\t%s\t%s\n", p.ID, p.Status, p.Title)
	}
	_ = tw.Flush()
}

// writePage prints a single page as aligned label/value lines.
func writePage(w io.Writer, p conf.Page) {
	fmt.Fprintf(w, "%-9s %s\n", "id:", p.ID)
	fmt.Fprintf(w, "%-9s %s\n", "title:", p.Title)
	if p.Status != "" {
		fmt.Fprintf(w, "%-9s %s\n", "status:", p.Status)
	}
	if p.SpaceID != "" {
		fmt.Fprintf(w, "%-9s %s\n", "space-id:", p.SpaceID)
	}
	if p.Version.Number > 0 {
		fmt.Fprintf(w, "%-9s %d\n", "version:", p.Version.Number)
	}
}
