package confcmd

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/aurokin/atlassian-cli/internal/appinfo"
	"github.com/aurokin/atlassian-cli/internal/cli"
	"github.com/aurokin/atlassian-cli/internal/conf"
)

// newSearchCommand builds the "search" command group.
func newSearchCommand(info appinfo.Info, g *cli.GlobalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "search",
		Short: "Search Confluence with CQL",
	}
	cmd.AddCommand(newSearchCQLCommand(info, g))
	return cmd
}

func newSearchCQLCommand(info appinfo.Info, g *cli.GlobalFlags) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:   "cql <cql>",
		Short: "Search content with a raw CQL query",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cc, err := confClient(info, g)
			if err != nil {
				return err
			}
			raw, err := cc.SearchCQL(cmd.Context(), args[0], limit)
			if err != nil {
				return err
			}
			if g.JSON != "" || g.JQ != "" {
				return cli.Render(cmd, g, raw)
			}
			results, err := conf.Decode[conf.SearchResults](raw)
			if err != nil {
				return err
			}
			writeSearchResults(cmd.OutOrStdout(), results.Results)
			return nil
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 0, "maximum number of results to return")
	return cmd
}

// writeSearchResults prints search hits as aligned id/type/title rows.
func writeSearchResults(w io.Writer, results []conf.SearchResult) {
	if len(results) == 0 {
		fmt.Fprintln(w, "No results found.")
		return
	}
	tw := tabWriter(w)
	for _, r := range results {
		fmt.Fprintf(tw, "%s\t%s\t%s\n", r.Content.ID, r.Content.Type, r.Content.Title)
	}
	_ = tw.Flush()
}
