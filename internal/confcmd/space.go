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

// newSpaceCommand builds the "space" command group.
func newSpaceCommand(info appinfo.Info, g *cli.GlobalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "space",
		Short: "List and view Confluence spaces",
	}
	cmd.AddCommand(
		newSpaceListCommand(info, g),
		newSpaceViewCommand(info, g),
	)
	return cmd
}

func newSpaceListCommand(info appinfo.Info, g *cli.GlobalFlags) *cobra.Command {
	var (
		limit int
		all   bool
	)
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List spaces visible to the authenticated account",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cc, err := confClient(info, g)
			if err != nil {
				return err
			}
			list := cc.ListSpaces
			if all {
				list = cc.ListSpacesAll
			}
			raw, err := list(cmd.Context(), limit)
			if err != nil {
				return err
			}
			return cli.RenderDecoded(cmd, g, raw, conf.Decode[conf.SpacePage],
				func(w io.Writer, page conf.SpacePage) {
					writeSpaceList(w, page.Results)
				})
		},
	}
	cli.AddPaginationFlags(cmd, &limit, &all, "spaces")
	return cmd
}

func newSpaceViewCommand(info appinfo.Info, g *cli.GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "view <key>",
		Short: "View a single Confluence space",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cc, err := confClient(info, g)
			if err != nil {
				return err
			}
			space, err := resolveSpace(cmd.Context(), cc, args[0])
			if err != nil {
				return err
			}
			raw, err := cc.GetSpace(cmd.Context(), space.ID)
			if err != nil {
				return err
			}
			return cli.RenderDecoded(cmd, g, raw, conf.Decode[conf.Space], writeSpace)
		},
	}
}

// resolveSpace looks up a space by key, since Confluence v2 addresses a space
// by numeric id. It returns a structured not-found error when no space matches.
func resolveSpace(ctx context.Context, cc *conf.Client, key string) (conf.Space, error) {
	raw, err := cc.FindSpaceByKey(ctx, key)
	if err != nil {
		return conf.Space{}, err
	}
	page, err := conf.Decode[conf.SpacePage](raw)
	if err != nil {
		return conf.Space{}, err
	}
	if len(page.Results) == 0 {
		return conf.Space{}, apperr.NotFoundOrNotVisible("no space found with key " + key)
	}
	return page.Results[0], nil
}

// writeSpaceList prints spaces as aligned key/name rows.
func writeSpaceList(w io.Writer, spaces []conf.Space) {
	if len(spaces) == 0 {
		fmt.Fprintln(w, "No spaces found.")
		return
	}
	tw := output.TabWriter(w)
	for _, s := range spaces {
		fmt.Fprintf(tw, "%s\t%s\n", s.Key, s.Name)
	}
	_ = tw.Flush()
}

// writeSpace prints a single space as aligned label/value lines.
func writeSpace(w io.Writer, s conf.Space) {
	lw := output.NewLabelWriter(w)
	lw.Add("key", s.Key)
	lw.Add("name", s.Name)
	lw.AddIf("type", s.Type)
	lw.AddIf("id", s.ID)
	_ = lw.Flush()
}
