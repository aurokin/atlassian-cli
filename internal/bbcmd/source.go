package bbcmd

import (
	"context"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/aurokin/atlassian-cli/internal/apperr"
	"github.com/aurokin/atlassian-cli/internal/appinfo"
	"github.com/aurokin/atlassian-cli/internal/bitbucket"
	"github.com/aurokin/atlassian-cli/internal/cli"
	"github.com/aurokin/atlassian-cli/internal/output"
)

func newSourceCommand(info appinfo.Info, g *cli.GlobalFlags) *cobra.Command {
	var (
		repoFlag, workspaceFlag string
		ref                     string
		limit                   int
		all                     bool
	)
	cmd := &cobra.Command{
		Use:     "src [path]",
		Aliases: []string{"source"},
		Short:   "List a repository directory at a ref",
		Long: "Lists the entries of a repository directory at the given --ref (default:\n" +
			"the repository's main branch). With no path, lists the repository root.",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := ""
			if len(args) == 1 {
				path = args[0]
			}
			target, err := resolveRepoTarget(nil, repoFlag, workspaceFlag)
			if err != nil {
				return err
			}
			bc, err := bbClient(info, g)
			if err != nil {
				return err
			}
			resolvedRef, err := resolveSourceRef(cmd.Context(), bc, target, ref)
			if err != nil {
				return err
			}
			list := bc.ListSource
			if all {
				list = bc.ListSourceAll
				limit = allPageSize(limit)
			}
			raw, err := list(cmd.Context(), target.Workspace, target.Repo, resolvedRef, path, limit)
			if err != nil {
				return err
			}
			return cli.RenderDecoded(cmd, g, raw, bitbucket.Decode[bitbucket.SourcePage],
				func(w io.Writer, page bitbucket.SourcePage) {
					writeSourceList(w, page.Values)
				})
		},
	}
	addRepoFlags(cmd, &repoFlag, &workspaceFlag)
	f := cmd.Flags()
	f.StringVar(&ref, "ref", "", "branch, tag, or commit to browse (default: the main branch)")
	cli.AddPaginationFlags(cmd, &limit, &all, "entries")
	return cmd
}

func newFileCommand(info appinfo.Info, g *cli.GlobalFlags) *cobra.Command {
	var (
		repoFlag, workspaceFlag string
		ref                     string
	)
	cmd := &cobra.Command{
		Use:   "file <path>",
		Short: "Print a file's contents at a ref",
		Long: "Writes the file at the given path and --ref (default: the repository's\n" +
			"main branch) to stdout verbatim. File content is raw, so --json/--jq do\n" +
			"not apply.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			target, err := resolveRepoTarget(nil, repoFlag, workspaceFlag)
			if err != nil {
				return err
			}
			bc, err := bbClient(info, g)
			if err != nil {
				return err
			}
			resolvedRef, err := resolveSourceRef(cmd.Context(), bc, target, ref)
			if err != nil {
				return err
			}
			data, err := bc.GetFileContent(cmd.Context(), target.Workspace, target.Repo, resolvedRef, args[0])
			if err != nil {
				return err
			}
			_, err = cmd.OutOrStdout().Write(data)
			return err
		},
	}
	addRepoFlags(cmd, &repoFlag, &workspaceFlag)
	cmd.Flags().StringVar(&ref, "ref", "", "branch, tag, or commit to read from (default: the main branch)")
	return cmd
}

// resolveSourceRef returns ref unchanged when set, otherwise the repository's
// default (main) branch, looked up via GetRepository.
func resolveSourceRef(ctx context.Context, bc *bitbucket.Client, target repoTarget, ref string) (string, error) {
	if ref != "" {
		return ref, nil
	}
	raw, err := bc.GetRepository(ctx, target.Workspace, target.Repo)
	if err != nil {
		return "", err
	}
	repo, err := bitbucket.Decode[bitbucket.Repository](raw)
	if err != nil {
		return "", err
	}
	if repo.MainBranch == nil || repo.MainBranch.Name == "" {
		return "", apperr.New("ref_unresolved",
			"could not determine the repository's main branch; pass --ref")
	}
	return repo.MainBranch.Name, nil
}

// writeSourceList prints directory entries as aligned type/size/path rows,
// directories first within the API's ordering.
func writeSourceList(w io.Writer, entries []bitbucket.SourceEntry) {
	if len(entries) == 0 {
		fmt.Fprintln(w, "No entries found.")
		return
	}
	tw := output.TabWriter(w)
	for _, e := range entries {
		fmt.Fprintf(tw, "%s\t%d\t%s\n", sourceKind(e.Type), e.Size, e.Path)
	}
	_ = tw.Flush()
}

// sourceKind maps the API entry type to a short label.
func sourceKind(t string) string {
	switch t {
	case "commit_directory":
		return "dir"
	case "commit_file":
		return "file"
	default:
		return t
	}
}
