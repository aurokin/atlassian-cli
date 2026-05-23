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

// newTagCommand builds the "tag" command group.
func newTagCommand(info appinfo.Info, g *cli.GlobalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "tag",
		Aliases: []string{"tags"},
		Short:   "List, view, create, and delete Bitbucket tags",
	}
	cmd.AddCommand(
		newTagListCommand(info, g),
		newTagViewCommand(info, g),
		newTagCreateCommand(info, g),
		newTagDeleteCommand(info, g),
	)
	return cmd
}

func newTagListCommand(info appinfo.Info, g *cli.GlobalFlags) *cobra.Command {
	var (
		repoFlag      string
		workspaceFlag string
		limit         int
		all           bool
	)
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List a repository's tags",
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
			list := bc.ListTags
			if all {
				list = bc.ListTagsAll
			}
			raw, err := list(cmd.Context(), target.Workspace, target.Repo, limit)
			if err != nil {
				return err
			}
			return cli.RenderDecoded(cmd, g, raw, bitbucket.Decode[bitbucket.TagPage],
				func(w io.Writer, page bitbucket.TagPage) {
					writeTagList(w, page.Values)
				})
		},
	}
	addRepoFlags(cmd, &repoFlag, &workspaceFlag)
	cli.AddPaginationFlags(cmd, &limit, &all, "tags")
	return cmd
}

func newTagViewCommand(info appinfo.Info, g *cli.GlobalFlags) *cobra.Command {
	var (
		repoFlag      string
		workspaceFlag string
	)
	cmd := &cobra.Command{
		Use:   "view <name>",
		Short: "View a single tag",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := strings.TrimSpace(args[0])
			if name == "" {
				return apperr.InvalidInput("a tag name is required")
			}
			target, err := resolveRepoTarget(nil, repoFlag, workspaceFlag)
			if err != nil {
				return err
			}
			bc, err := bbClient(info, g)
			if err != nil {
				return err
			}
			raw, err := bc.GetTag(cmd.Context(), target.Workspace, target.Repo, name)
			if err != nil {
				return err
			}
			return cli.RenderDecoded(cmd, g, raw, bitbucket.Decode[bitbucket.Tag], writeTag)
		},
	}
	addRepoFlags(cmd, &repoFlag, &workspaceFlag)
	return cmd
}

func newTagCreateCommand(info appinfo.Info, g *cli.GlobalFlags) *cobra.Command {
	var (
		repoFlag      string
		workspaceFlag string
		opts          bitbucket.CreateTagOptions
	)
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a tag",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if strings.TrimSpace(opts.Name) == "" {
				return apperr.InvalidInput("a tag name is required; pass --name")
			}
			if strings.TrimSpace(opts.Target) == "" {
				return apperr.InvalidInput("a tag target is required; pass --target with a commit hash")
			}
			target, err := resolveRepoTarget(nil, repoFlag, workspaceFlag)
			if err != nil {
				return err
			}
			bc, err := bbClient(info, g)
			if err != nil {
				return err
			}
			raw, err := bc.CreateTag(cmd.Context(), target.Workspace, target.Repo, opts)
			if err != nil {
				return err
			}
			return cli.RenderDecoded(cmd, g, raw, bitbucket.Decode[bitbucket.Tag],
				func(w io.Writer, tag bitbucket.Tag) {
					fmt.Fprintf(w, "created tag %s\n", tag.Name)
				})
		},
	}
	addRepoFlags(cmd, &repoFlag, &workspaceFlag)
	f := cmd.Flags()
	f.StringVar(&opts.Name, "name", "", "new tag name (required)")
	f.StringVar(&opts.Target, "target", "", "commit hash the tag points at (required)")
	f.StringVar(&opts.Message, "message", "", "annotation message (creates an annotated tag)")
	return cmd
}

func newTagDeleteCommand(info appinfo.Info, g *cli.GlobalFlags) *cobra.Command {
	var (
		repoFlag      string
		workspaceFlag string
	)
	cmd := &cobra.Command{
		Use:   "delete <name>",
		Short: "Delete a tag",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := strings.TrimSpace(args[0])
			if name == "" {
				return apperr.InvalidInput("a tag name is required")
			}
			target, err := resolveRepoTarget(nil, repoFlag, workspaceFlag)
			if err != nil {
				return err
			}
			bc, err := bbClient(info, g)
			if err != nil {
				return err
			}
			if err := bc.DeleteTag(cmd.Context(), target.Workspace, target.Repo, name); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "deleted tag %s\n", name)
			return nil
		},
	}
	addRepoFlags(cmd, &repoFlag, &workspaceFlag)
	return cmd
}

// writeTagList prints tags as aligned name/target-hash rows.
func writeTagList(w io.Writer, tags []bitbucket.Tag) {
	if len(tags) == 0 {
		fmt.Fprintln(w, "No tags found.")
		return
	}
	tw := output.TabWriter(w)
	for _, t := range tags {
		fmt.Fprintf(tw, "%s\t%s\n", t.Name, tagTargetHash(t))
	}
	_ = tw.Flush()
}

// writeTag prints a single tag as aligned label/value lines.
func writeTag(w io.Writer, t bitbucket.Tag) {
	lw := output.NewLabelWriter(w)
	lw.Add("name", t.Name)
	lw.AddIf("target", tagTargetHash(t))
	lw.AddIf("date", t.Date)
	if t.Message != "" {
		lw.Add("message", strings.TrimSpace(t.Message))
	}
	_ = lw.Flush()
}

// tagTargetHash returns the short hash of a tag's target commit, or "" when the
// target is absent.
func tagTargetHash(t bitbucket.Tag) string {
	if t.Target == nil {
		return ""
	}
	return shortHash(t.Target.Hash)
}
