package bbcmd

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/aurokin/atlassian-cli/internal/apperr"
	"github.com/aurokin/atlassian-cli/internal/appinfo"
	"github.com/aurokin/atlassian-cli/internal/bitbucket"
	"github.com/aurokin/atlassian-cli/internal/cli"
	"github.com/aurokin/atlassian-cli/internal/output"
)

// newBranchCommand builds the "branch" command group.
func newBranchCommand(info appinfo.Info, g *cli.GlobalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "branch",
		Aliases: []string{"branches"},
		Short:   "List, view, create, and delete Bitbucket branches",
	}
	cmd.AddCommand(
		newBranchListCommand(info, g),
		newBranchViewCommand(info, g),
		newBranchCreateCommand(info, g),
		newBranchDeleteCommand(info, g),
	)
	return cmd
}

func newBranchListCommand(info appinfo.Info, g *cli.GlobalFlags) *cobra.Command {
	var (
		repoFlag      string
		workspaceFlag string
		limit         int
		all           bool
	)
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List a repository's branches",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			target, err := resolveRepoTarget(nil, repoFlag, workspaceFlag)
			if err != nil {
				return err
			}
			bc, err := bbClient(info, g)
			if err != nil {
				return err
			}
			list := bc.ListBranches
			if all {
				list = bc.ListBranchesAll
				limit = allPageSize(limit)
			}
			raw, err := list(cmd.Context(), target.Workspace, target.Repo, limit)
			if err != nil {
				return err
			}
			return cli.RenderDecoded(cmd, g, raw, bitbucket.Decode[bitbucket.BranchPage],
				func(w io.Writer, page bitbucket.BranchPage) {
					writeBranchList(w, page.Values)
				})
		},
	}
	addRepoFlags(cmd, &repoFlag, &workspaceFlag)
	cli.AddPaginationFlags(cmd, &limit, &all, "branches")
	return cmd
}

func newBranchViewCommand(info appinfo.Info, g *cli.GlobalFlags) *cobra.Command {
	var (
		repoFlag      string
		workspaceFlag string
	)
	cmd := &cobra.Command{
		Use:   "view <name>",
		Short: "View a single branch",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := strings.TrimSpace(args[0])
			if name == "" {
				return apperr.InvalidInput("a branch name is required")
			}
			target, err := resolveRepoTarget(nil, repoFlag, workspaceFlag)
			if err != nil {
				return err
			}
			bc, err := bbClient(info, g)
			if err != nil {
				return err
			}
			raw, err := bc.GetBranch(cmd.Context(), target.Workspace, target.Repo, name)
			if err != nil {
				return err
			}
			return cli.RenderDecoded(cmd, g, raw, bitbucket.Decode[bitbucket.Branch], writeBranch)
		},
	}
	addRepoFlags(cmd, &repoFlag, &workspaceFlag)
	return cmd
}

func newBranchCreateCommand(info appinfo.Info, g *cli.GlobalFlags) *cobra.Command {
	var (
		repoFlag      string
		workspaceFlag string
		opts          bitbucket.CreateBranchOptions
	)
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a branch",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if strings.TrimSpace(opts.Name) == "" {
				return apperr.InvalidInput("a branch name is required; pass --name")
			}
			if strings.TrimSpace(opts.Target) == "" {
				return apperr.InvalidInput("a branch target is required; pass --target with a commit hash or branch name")
			}
			target, err := resolveRepoTarget(nil, repoFlag, workspaceFlag)
			if err != nil {
				return err
			}
			bc, err := bbClient(info, g)
			if err != nil {
				return err
			}
			raw, err := bc.CreateBranch(cmd.Context(), target.Workspace, target.Repo, opts)
			if err != nil {
				return err
			}
			return cli.RenderDecoded(cmd, g, raw, bitbucket.Decode[bitbucket.Branch],
				func(w io.Writer, branch bitbucket.Branch) {
					fmt.Fprintf(w, "created branch %s\n", branch.Name)
				})
		},
	}
	addRepoFlags(cmd, &repoFlag, &workspaceFlag)
	f := cmd.Flags()
	f.StringVar(&opts.Name, "name", "", "new branch name (required)")
	f.StringVar(&opts.Target, "target", "", "commit hash or existing branch the new branch points at (required)")
	return cmd
}

func newBranchDeleteCommand(info appinfo.Info, g *cli.GlobalFlags) *cobra.Command {
	var (
		repoFlag      string
		workspaceFlag string
	)
	cmd := &cobra.Command{
		Use:   "delete <name>",
		Short: "Delete a branch",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := strings.TrimSpace(args[0])
			if name == "" {
				return apperr.InvalidInput("a branch name is required")
			}
			target, err := resolveRepoTarget(nil, repoFlag, workspaceFlag)
			if err != nil {
				return err
			}
			bc, err := bbClient(info, g)
			if err != nil {
				return err
			}
			if err := bc.DeleteBranch(cmd.Context(), target.Workspace, target.Repo, name); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "deleted branch %s\n", name)
			return nil
		},
	}
	addRepoFlags(cmd, &repoFlag, &workspaceFlag)
	return cmd
}

// writeBranchList prints branches as aligned name/tip-hash rows.
func writeBranchList(w io.Writer, branches []bitbucket.Branch) {
	if len(branches) == 0 {
		fmt.Fprintln(w, "No branches found.")
		return
	}
	tw := output.TabWriter(w)
	for _, b := range branches {
		fmt.Fprintf(tw, "%s\t%s\n", b.Name, branchTipHash(b))
	}
	_ = tw.Flush()
}

// writeBranch prints a single branch as aligned label/value lines.
func writeBranch(w io.Writer, b bitbucket.Branch) {
	lw := output.NewLabelWriter(w)
	lw.Add("name", b.Name)
	lw.AddIf("target", branchTipHash(b))
	_ = lw.Flush()
}

// branchTipHash returns the short hash of a branch's tip commit, or "" when the
// target is absent.
func branchTipHash(b bitbucket.Branch) string {
	if b.Target == nil {
		return ""
	}
	return shortHash(b.Target.Hash)
}
