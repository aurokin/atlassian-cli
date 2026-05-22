package bbcmd

import (
	"strings"

	"github.com/spf13/cobra"

	"github.com/aurokin/atlassian-cli/internal/apperr"
	"github.com/aurokin/atlassian-cli/internal/appinfo"
	"github.com/aurokin/atlassian-cli/internal/bitbucket"
	"github.com/aurokin/atlassian-cli/internal/cli"
)

// newSearchCommand builds the "search" command group. Each subcommand takes a
// raw Bitbucket query expression (the `q` filter), mirroring atl-jira's raw-JQL
// search, and renders results with the same human/JSON paths as the matching
// list command.
func newSearchCommand(info appinfo.Info, g *cli.GlobalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "search",
		Short: "Search repositories, pull requests, and issues with a raw Bitbucket query",
	}
	cmd.AddCommand(
		newSearchReposCommand(info, g),
		newSearchPRsCommand(info, g),
		newSearchIssuesCommand(info, g),
	)
	return cmd
}

func newSearchReposCommand(info appinfo.Info, g *cli.GlobalFlags) *cobra.Command {
	var (
		workspaceFlag string
		sort          string
		limit         int
		all           bool
	)
	cmd := &cobra.Command{
		Use:   "repos <query>",
		Short: "Search a workspace's repositories with a raw Bitbucket query",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			query, err := requireQuery(args[0])
			if err != nil {
				return err
			}
			workspace, err := resolveWorkspace(nil, workspaceFlag)
			if err != nil {
				return err
			}
			bc, err := bbClient(info, g)
			if err != nil {
				return err
			}
			search := bc.SearchRepositories
			if all {
				search = bc.SearchRepositoriesAll
			}
			raw, err := search(cmd.Context(), workspace, query, sort, limit)
			if err != nil {
				return err
			}
			if g.JSON != "" || g.JQ != "" {
				return cli.Render(cmd, g, raw)
			}
			page, err := bitbucket.Decode[bitbucket.RepositoryPage](raw)
			if err != nil {
				return err
			}
			writeRepoList(cmd.OutOrStdout(), page.Values)
			return nil
		},
	}
	f := cmd.Flags()
	f.StringVar(&workspaceFlag, "workspace", "", "workspace slug to search (required)")
	addSearchFlags(cmd, &sort, &limit, &all)
	return cmd
}

func newSearchPRsCommand(info appinfo.Info, g *cli.GlobalFlags) *cobra.Command {
	var (
		repoFlag      string
		workspaceFlag string
		sort          string
		limit         int
		all           bool
	)
	cmd := &cobra.Command{
		Use:   "prs <query>",
		Short: "Search a repository's pull requests with a raw Bitbucket query",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			query, err := requireQuery(args[0])
			if err != nil {
				return err
			}
			target, err := resolveRepoTarget(nil, repoFlag, workspaceFlag)
			if err != nil {
				return err
			}
			bc, err := bbClient(info, g)
			if err != nil {
				return err
			}
			search := bc.SearchPullRequests
			if all {
				search = bc.SearchPullRequestsAll
			}
			raw, err := search(cmd.Context(), target.Workspace, target.Repo, query, sort, limit)
			if err != nil {
				return err
			}
			if g.JSON != "" || g.JQ != "" {
				return cli.Render(cmd, g, raw)
			}
			page, err := bitbucket.Decode[bitbucket.PullRequestPage](raw)
			if err != nil {
				return err
			}
			writePRList(cmd.OutOrStdout(), page.Values)
			return nil
		},
	}
	addRepoFlags(cmd, &repoFlag, &workspaceFlag)
	addSearchFlags(cmd, &sort, &limit, &all)
	return cmd
}

func newSearchIssuesCommand(info appinfo.Info, g *cli.GlobalFlags) *cobra.Command {
	var (
		repoFlag      string
		workspaceFlag string
		sort          string
		limit         int
		all           bool
	)
	cmd := &cobra.Command{
		Use:   "issues <query>",
		Short: "Search a repository's issues with a raw Bitbucket query",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			query, err := requireQuery(args[0])
			if err != nil {
				return err
			}
			target, err := resolveRepoTarget(nil, repoFlag, workspaceFlag)
			if err != nil {
				return err
			}
			bc, err := bbClient(info, g)
			if err != nil {
				return err
			}
			search := bc.SearchIssues
			if all {
				search = bc.SearchIssuesAll
			}
			raw, err := search(cmd.Context(), target.Workspace, target.Repo, query, sort, limit)
			if err != nil {
				return err
			}
			if g.JSON != "" || g.JQ != "" {
				return cli.Render(cmd, g, raw)
			}
			page, err := bitbucket.Decode[bitbucket.IssuePage](raw)
			if err != nil {
				return err
			}
			writeIssueList(cmd.OutOrStdout(), page.Values)
			return nil
		},
	}
	addRepoFlags(cmd, &repoFlag, &workspaceFlag)
	addSearchFlags(cmd, &sort, &limit, &all)
	return cmd
}

// addSearchFlags binds the flags shared by every search subcommand.
func addSearchFlags(cmd *cobra.Command, sort *string, limit *int, all *bool) {
	f := cmd.Flags()
	f.StringVar(sort, "sort", "", "sort field (e.g. -updated_on); defaults to the Bitbucket API order")
	f.IntVar(limit, "limit", 0, "maximum number of results per page")
	f.BoolVar(all, "all", false, "follow pagination and return every page (--limit sets the page size)")
}

// requireQuery trims and validates a search query argument.
func requireQuery(arg string) (string, error) {
	q := strings.TrimSpace(arg)
	if q == "" {
		return "", apperr.InvalidInput("a query is required")
	}
	return q, nil
}
