package cli

import (
	"sort"

	"github.com/spf13/cobra"

	"github.com/aurokin/atlassian-cli/internal/config"
)

// completeSiteNames is a cobra flag-completion function for --site. It offers
// the configured site profile names read from the on-disk config, so a shell
// can complete `--site <TAB>` without any network call. Any error (missing or
// unreadable config) yields no suggestions rather than a completion failure.
func completeSiteNames(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	path, err := config.DefaultPath()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	cfg, err := config.Load(path)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	names := make([]string, 0, len(cfg.Sites))
	for name := range cfg.Sites {
		names = append(names, name)
	}
	sort.Strings(names)
	return names, cobra.ShellCompDirectiveNoFileComp
}

// FixedCompletion returns a cobra completion function that always offers the
// given fixed set of values (and never falls back to file completion). It is
// used for flags whose valid values are a known, closed enum — token styles,
// body formats, HTTP methods — so shells complete them without a network call.
func FixedCompletion(values ...string) func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
	return func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		return values, cobra.ShellCompDirectiveNoFileComp
	}
}
