package confcmd

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/aurokin/atlassian-cli/internal/apperr"
	"github.com/aurokin/atlassian-cli/internal/appinfo"
	"github.com/aurokin/atlassian-cli/internal/cli"
	"github.com/aurokin/atlassian-cli/internal/conf"
	"github.com/aurokin/atlassian-cli/internal/output"
)

// newSearchCommand builds the "search" command group.
func newSearchCommand(info appinfo.Info, g *cli.GlobalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "search",
		Short: "Search Confluence with CQL",
	}
	cmd.AddCommand(
		newSearchCQLCommand(info, g),
		newSearchTextCommand(info, g),
	)
	return cmd
}

func newSearchTextCommand(info appinfo.Info, g *cli.GlobalFlags) *cobra.Command {
	var (
		space       string
		contentType string
		limit       int
		all         bool
	)
	cmd := &cobra.Command{
		Use:   "text <query>",
		Short: "Search content by free text, building the CQL for you",
		Long: "Builds a CQL query of the form text ~ \"<query>\" and runs it, so you\n" +
			"do not have to write CQL by hand. --space (a space key) and --type\n" +
			"(page, blogpost, attachment, comment, …) add the matching CQL clauses.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			query := strings.TrimSpace(args[0])
			if query == "" {
				return apperr.InvalidInput("a search query is required")
			}
			cc, err := confClient(info, g)
			if err != nil {
				return err
			}
			search := cc.SearchCQL
			if all {
				search = cc.SearchCQLAll
			}
			raw, err := search(cmd.Context(), buildTextCQL(query, space, contentType), limit)
			if err != nil {
				return err
			}
			return cli.RenderDecoded(cmd, g, raw, conf.Decode[conf.SearchResults],
				func(w io.Writer, results conf.SearchResults) {
					writeSearchResults(w, results.Results)
				})
		},
	}
	f := cmd.Flags()
	f.StringVar(&space, "space", "", "restrict to a space by key")
	f.StringVar(&contentType, "type", "", "restrict to a content type (page, blogpost, attachment, comment)")
	cli.AddPaginationFlags(cmd, &limit, &all, "results")
	return cmd
}

// buildTextCQL assembles a CQL query matching free text, optionally constrained
// by content type and space key. Each value is quoted as a CQL string literal
// with embedded double quotes escaped.
func buildTextCQL(query, space, contentType string) string {
	var b strings.Builder
	b.WriteString(`text ~ "` + cqlEscape(query) + `"`)
	if contentType != "" {
		b.WriteString(` and type = "` + cqlEscape(contentType) + `"`)
	}
	if space != "" {
		b.WriteString(` and space = "` + cqlEscape(space) + `"`)
	}
	return b.String()
}

// cqlEscape escapes a value for use inside a CQL double-quoted string literal.
// Backslashes are escaped first so an already-escaped quote (or a value ending
// in a backslash) cannot defeat the quote escaping that follows.
func cqlEscape(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, `\`, `\\`), `"`, `\"`)
}

func newSearchCQLCommand(info appinfo.Info, g *cli.GlobalFlags) *cobra.Command {
	var (
		limit int
		all   bool
	)
	cmd := &cobra.Command{
		Use:   "cql <cql>",
		Short: "Search content with a raw CQL query",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cc, err := confClient(info, g)
			if err != nil {
				return err
			}
			search := cc.SearchCQL
			if all {
				search = cc.SearchCQLAll
			}
			raw, err := search(cmd.Context(), args[0], limit)
			if err != nil {
				return err
			}
			return cli.RenderDecoded(cmd, g, raw, conf.Decode[conf.SearchResults],
				func(w io.Writer, results conf.SearchResults) {
					writeSearchResults(w, results.Results)
				})
		},
	}
	cli.AddPaginationFlags(cmd, &limit, &all, "results")
	return cmd
}

// writeSearchResults prints search hits as aligned id/type/title rows.
func writeSearchResults(w io.Writer, results []conf.SearchResult) {
	if len(results) == 0 {
		fmt.Fprintln(w, "No results found.")
		return
	}
	tw := output.TabWriter(w)
	for _, r := range results {
		fmt.Fprintf(tw, "%s\t%s\t%s\n", r.Content.ID, r.Content.Type, r.Content.Title)
	}
	_ = tw.Flush()
}
