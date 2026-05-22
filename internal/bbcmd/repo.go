package bbcmd

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/aurokin/atlassian-cli/internal/appinfo"
	"github.com/aurokin/atlassian-cli/internal/bitbucket"
	"github.com/aurokin/atlassian-cli/internal/cli"
	"github.com/aurokin/atlassian-cli/internal/output"
)

// newRepoCommand builds the "repo" command group.
func newRepoCommand(info appinfo.Info, g *cli.GlobalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "repo",
		Aliases: []string{"repos", "repository"},
		Short:   "List and view Bitbucket repositories",
	}
	cmd.AddCommand(
		newRepoViewCommand(info, g),
		newRepoListCommand(info, g),
	)
	return cmd
}

func newRepoViewCommand(info appinfo.Info, g *cli.GlobalFlags) *cobra.Command {
	var (
		repoFlag      string
		workspaceFlag string
	)
	cmd := &cobra.Command{
		Use:   "view [<workspace>/<repo>]",
		Short: "View a single Bitbucket repository",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			target, err := resolveRepoTarget(args, repoFlag, workspaceFlag)
			if err != nil {
				return err
			}
			bc, err := bbClient(info, g)
			if err != nil {
				return err
			}
			raw, err := bc.GetRepository(cmd.Context(), target.Workspace, target.Repo)
			if err != nil {
				return err
			}
			if g.JSON != "" || g.JQ != "" {
				return cli.Render(cmd, g, raw)
			}
			repo, err := bitbucket.Decode[bitbucket.Repository](raw)
			if err != nil {
				return err
			}
			writeRepo(cmd.OutOrStdout(), repo)
			return nil
		},
	}
	addRepoFlags(cmd, &repoFlag, &workspaceFlag)
	return cmd
}

func newRepoListCommand(info appinfo.Info, g *cli.GlobalFlags) *cobra.Command {
	var (
		workspaceFlag string
		limit         int
		all           bool
	)
	cmd := &cobra.Command{
		Use:   "list [<workspace>]",
		Short: "List repositories in a workspace",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			workspace, err := resolveWorkspace(args, workspaceFlag)
			if err != nil {
				return err
			}
			bc, err := bbClient(info, g)
			if err != nil {
				return err
			}
			list := bc.ListRepositories
			if all {
				list = bc.ListRepositoriesAll
			}
			raw, err := list(cmd.Context(), workspace, limit)
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
	f.StringVar(&workspaceFlag, "workspace", "", "workspace slug to list repositories from")
	f.IntVar(&limit, "limit", 0, "maximum number of repositories per page")
	f.BoolVar(&all, "all", false, "follow pagination and return every page (--limit sets the page size)")
	return cmd
}

// addRepoFlags binds the shared repo-targeting flags (decision D2).
func addRepoFlags(cmd *cobra.Command, repoFlag, workspaceFlag *string) {
	f := cmd.Flags()
	f.StringVar(repoFlag, "repo", "", "repository target as <workspace>/<repo> (or <repo> with --workspace)")
	f.StringVar(workspaceFlag, "workspace", "", "workspace slug; disambiguates a bare repository target")
}

// writeRepo prints a single repository as aligned label/value lines.
func writeRepo(w io.Writer, r bitbucket.Repository) {
	fmt.Fprintf(w, "%-12s %s\n", "full name:", r.FullName)
	if r.Name != "" && r.Name != r.FullName {
		fmt.Fprintf(w, "%-12s %s\n", "name:", r.Name)
	}
	fmt.Fprintf(w, "%-12s %s\n", "visibility:", visibilityLabel(r.IsPrivate))
	if r.Project != nil && (r.Project.Key != "" || r.Project.Name != "") {
		fmt.Fprintf(w, "%-12s %s\n", "project:", projectLabel(r.Project))
	}
	if r.MainBranch != nil && r.MainBranch.Name != "" {
		fmt.Fprintf(w, "%-12s %s\n", "main branch:", r.MainBranch.Name)
	}
	if r.Description != "" {
		fmt.Fprintf(w, "%-12s %s\n", "description:", r.Description)
	}
}

// writeRepoList prints repositories as aligned full-name/visibility rows.
func writeRepoList(w io.Writer, repos []bitbucket.Repository) {
	if len(repos) == 0 {
		fmt.Fprintln(w, "No repositories found.")
		return
	}
	tw := output.TabWriter(w)
	for _, r := range repos {
		fmt.Fprintf(tw, "%s\t%s\n", r.FullName, visibilityLabel(r.IsPrivate))
	}
	_ = tw.Flush()
}

// projectLabel renders a project as "KEY — Name", "KEY", or "Name" depending
// on which fields are present.
func projectLabel(p *bitbucket.Project) string {
	switch {
	case p.Key != "" && p.Name != "":
		return p.Key + " — " + p.Name
	case p.Key != "":
		return p.Key
	default:
		return p.Name
	}
}
