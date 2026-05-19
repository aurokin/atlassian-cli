package confcmd

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/aurokin/atlassian-cli/internal/apperr"
	"github.com/aurokin/atlassian-cli/internal/appinfo"
	"github.com/aurokin/atlassian-cli/internal/cli"
	"github.com/aurokin/atlassian-cli/internal/conf"
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
	)
	return cmd
}

func newPageListCommand(info appinfo.Info, g *cli.GlobalFlags) *cobra.Command {
	var (
		space string
		limit int
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
			raw, err := cc.ListPages(cmd.Context(), sp.ID, limit)
			if err != nil {
				return err
			}
			if g.JSON != "" || g.JQ != "" {
				return cli.Render(cmd, g, raw)
			}
			page, err := conf.Decode[conf.PageList](raw)
			if err != nil {
				return err
			}
			writePageList(cmd.OutOrStdout(), page.Results)
			return nil
		},
	}
	f := cmd.Flags()
	f.StringVar(&space, "space", "", "space key (required)")
	f.IntVar(&limit, "limit", 0, "maximum number of pages to return")
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
			if g.JSON != "" || g.JQ != "" {
				return cli.Render(cmd, g, raw)
			}
			p, err := conf.Decode[conf.Page](raw)
			if err != nil {
				return err
			}
			writePage(cmd.OutOrStdout(), p)
			return nil
		},
	}
}

func newPageChildrenCommand(info appinfo.Info, g *cli.GlobalFlags) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:   "children <id>",
		Short: "List the direct child pages of a page",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cc, err := confClient(info, g)
			if err != nil {
				return err
			}
			raw, err := cc.GetChildPages(cmd.Context(), args[0], limit)
			if err != nil {
				return err
			}
			if g.JSON != "" || g.JQ != "" {
				return cli.Render(cmd, g, raw)
			}
			page, err := conf.Decode[conf.PageList](raw)
			if err != nil {
				return err
			}
			writePageList(cmd.OutOrStdout(), page.Results)
			return nil
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 0, "maximum number of child pages to return")
	return cmd
}

// writePageList prints pages as aligned id/status/title rows.
func writePageList(w io.Writer, pages []conf.Page) {
	if len(pages) == 0 {
		fmt.Fprintln(w, "No pages found.")
		return
	}
	tw := tabWriter(w)
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
