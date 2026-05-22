package bbcmd

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/aurokin/atlassian-cli/internal/apperr"
	"github.com/aurokin/atlassian-cli/internal/appinfo"
	"github.com/aurokin/atlassian-cli/internal/cli"
	"github.com/aurokin/atlassian-cli/internal/config"
	"github.com/aurokin/atlassian-cli/internal/output"
)

// aliasMaxDepth bounds recursive alias expansion so a self- or mutually
// referential alias can never loop forever.
const aliasMaxDepth = 8

// newAliasCommand builds the "alias" command group: user-defined command
// shorthands stored in the shared config and expanded before dispatch.
func newAliasCommand(info appinfo.Info, g *cli.GlobalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "alias",
		Short: "Manage command aliases",
		Long: "Define, list, and delete command aliases. An alias replaces its name " +
			"as the first argument with its expansion before the command runs, e.g. " +
			"`alias set prs \"pr list\"` makes `atl-bb prs` run `atl-bb pr list`.",
	}
	cmd.AddCommand(
		newAliasSetCommand(info, g),
		newAliasListCommand(info, g),
		newAliasDeleteCommand(info, g),
	)
	return cmd
}

func newAliasSetCommand(_ appinfo.Info, _ *cli.GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "set <name> <expansion>",
		Short: "Define or overwrite an alias",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := strings.TrimSpace(args[0])
			expansion := strings.TrimSpace(args[1])
			if name == "" {
				return apperr.InvalidInput("an alias name is required")
			}
			if expansion == "" {
				return apperr.InvalidInput("an alias expansion is required")
			}
			// Reject an expansion that cannot be parsed now, so a broken alias is
			// never persisted to be discovered only at expansion time.
			if _, err := splitCommandLine(expansion); err != nil {
				return apperr.InvalidInput(fmt.Sprintf("invalid alias expansion %q: %v", expansion, err))
			}
			path, err := config.DefaultPath()
			if err != nil {
				return err
			}
			cfg, err := config.Load(path)
			if err != nil {
				return err
			}
			if cfg.Aliases == nil {
				cfg.Aliases = map[string]string{}
			}
			cfg.Aliases[name] = expansion
			if err := config.Save(path, cfg); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "set alias %s = %s\n", name, expansion)
			return nil
		},
	}
}

func newAliasListCommand(_ appinfo.Info, g *cli.GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List defined aliases",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			path, err := config.DefaultPath()
			if err != nil {
				return err
			}
			cfg, err := config.Load(path)
			if err != nil {
				return err
			}
			if g.JSON != "" || g.JQ != "" {
				return cli.Render(cmd, g, cfg.Aliases)
			}
			writeAliasList(cmd.OutOrStdout(), cfg.Aliases)
			return nil
		},
	}
}

func newAliasDeleteCommand(_ appinfo.Info, _ *cli.GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:     "delete <name>",
		Aliases: []string{"remove", "rm"},
		Short:   "Delete an alias",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := strings.TrimSpace(args[0])
			if name == "" {
				return apperr.InvalidInput("an alias name is required")
			}
			path, err := config.DefaultPath()
			if err != nil {
				return err
			}
			cfg, err := config.Load(path)
			if err != nil {
				return err
			}
			if _, ok := cfg.Aliases[name]; !ok {
				return apperr.NotFoundOrNotVisible(fmt.Sprintf("no alias named %q", name))
			}
			delete(cfg.Aliases, name)
			if err := config.Save(path, cfg); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "deleted alias %s\n", name)
			return nil
		},
	}
}

// writeAliasList prints aliases as aligned name/expansion rows, sorted by name.
func writeAliasList(w io.Writer, aliases map[string]string) {
	if len(aliases) == 0 {
		fmt.Fprintln(w, "No aliases defined.")
		return
	}
	names := make([]string, 0, len(aliases))
	for name := range aliases {
		names = append(names, name)
	}
	sort.Strings(names)
	tw := output.TabWriter(w)
	for _, name := range names {
		fmt.Fprintf(tw, "%s\t%s\n", name, aliases[name])
	}
	_ = tw.Flush()
}

// ExpandAliases loads the configured aliases and expands args accordingly. A
// missing or unreadable config yields the args unchanged (best effort): alias
// expansion must never block a command that does not use aliases.
func ExpandAliases(args []string) ([]string, error) {
	path, err := config.DefaultPath()
	if err != nil {
		return args, nil
	}
	cfg, err := config.Load(path)
	if err != nil {
		return args, nil
	}
	return expandAliasArgs(args, cfg.Aliases)
}

// expandAliasArgs replaces the leading argument with its alias expansion,
// repeating up to aliasMaxDepth times so an alias may reference another. A
// cycle (an alias seen twice) stops expansion rather than erroring, leaving the
// args as expanded so far.
func expandAliasArgs(args []string, aliases map[string]string) ([]string, error) {
	if len(args) == 0 || len(aliases) == 0 {
		return args, nil
	}
	expanded := append([]string(nil), args...)
	seen := map[string]struct{}{}
	for depth := 0; depth < aliasMaxDepth && len(expanded) > 0; depth++ {
		replacement, ok := aliases[expanded[0]]
		if !ok || strings.TrimSpace(replacement) == "" {
			return expanded, nil
		}
		if _, ok := seen[expanded[0]]; ok {
			return expanded, nil
		}
		seen[expanded[0]] = struct{}{}

		fields, err := splitCommandLine(replacement)
		if err != nil {
			return nil, apperr.InvalidInput(fmt.Sprintf("invalid alias %q: %v", expanded[0], err))
		}
		if len(fields) == 0 {
			return expanded, nil
		}
		expanded = append(fields, expanded[1:]...)
	}
	return expanded, nil
}

// splitCommandLine splits a command string into fields with shell-like quoting:
// single and double quotes group whitespace, and a backslash escapes the next
// character. An unterminated quote is an error.
func splitCommandLine(s string) ([]string, error) {
	var (
		fields  []string
		cur     strings.Builder
		inField bool
		quote   rune // 0, '\'', or '"'
		escaped bool
	)
	flush := func() {
		if inField {
			fields = append(fields, cur.String())
			cur.Reset()
			inField = false
		}
	}
	for _, r := range s {
		switch {
		case escaped:
			cur.WriteRune(r)
			inField = true
			escaped = false
		case r == '\\' && quote != '\'':
			// A backslash escapes inside double quotes and unquoted text, but is
			// literal inside single quotes.
			escaped = true
			inField = true
		case quote != 0:
			if r == quote {
				quote = 0
			} else {
				cur.WriteRune(r)
			}
		case r == '\'' || r == '"':
			quote = r
			inField = true
		case r == ' ' || r == '\t' || r == '\n':
			flush()
		default:
			cur.WriteRune(r)
			inField = true
		}
	}
	if escaped {
		return nil, fmt.Errorf("trailing backslash")
	}
	if quote != 0 {
		return nil, fmt.Errorf("unterminated %c quote", quote)
	}
	flush()
	return fields, nil
}
