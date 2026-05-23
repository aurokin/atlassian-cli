package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// AddPaginationFlags registers the standard --limit/--all pair on a list
// command, binding them to the provided pointers. noun is the plural resource
// name shown in --limit's help (for example "issues" or "pull requests").
//
// The wording is defined here once so it cannot drift between products:
// --limit is always the per-page size (never a total cap), which is the
// behavior across all three clients — with --all, the page is re-requested at
// this size until every page is followed.
func AddPaginationFlags(cmd *cobra.Command, limit *int, all *bool, noun string) {
	f := cmd.Flags()
	f.IntVar(limit, "limit", 0, fmt.Sprintf("maximum number of %s per page", noun))
	f.BoolVar(all, "all", false, "follow pagination and return every page (--limit sets the page size)")
}
