package confcmd

import (
	"context"
	"encoding/json"
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
			// The keys-filtered list already returns the full space object, so
			// render its first match directly rather than making a second
			// GetSpace round-trip by id.
			raw, _, err := findSpaceRaw(cmd.Context(), cc, args[0])
			if err != nil {
				return err
			}
			return cli.RenderDecoded(cmd, g, raw, conf.Decode[conf.Space], writeSpace)
		},
	}
}

// rawSpacePage decodes a space listing while keeping each result's raw JSON, so
// a single matched space can be rendered verbatim under --json.
type rawSpacePage struct {
	Results []json.RawMessage `json:"results"`
}

// findSpaceRaw looks up a space by key (Confluence v2 addresses a space by
// numeric id, so a key lookup is a keys-filtered list) and returns both the
// matched result's raw JSON and its decoded form. It returns a structured
// not-found error when no space matches.
func findSpaceRaw(ctx context.Context, cc *conf.Client, key string) (json.RawMessage, conf.Space, error) {
	raw, err := cc.FindSpaceByKey(ctx, key)
	if err != nil {
		return nil, conf.Space{}, err
	}
	page, err := conf.Decode[rawSpacePage](raw)
	if err != nil {
		return nil, conf.Space{}, err
	}
	if len(page.Results) == 0 {
		return nil, conf.Space{}, apperr.NotFoundOrNotVisible("no space found with key " + key)
	}
	space, err := conf.Decode[conf.Space](page.Results[0])
	if err != nil {
		return nil, conf.Space{}, err
	}
	return page.Results[0], space, nil
}

// resolveSpace looks up a space by key and returns its decoded form, for
// callers that only need the id/key/name (page and blogpost commands).
func resolveSpace(ctx context.Context, cc *conf.Client, key string) (conf.Space, error) {
	_, space, err := findSpaceRaw(ctx, cc, key)
	return space, err
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
