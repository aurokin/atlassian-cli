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

// newCommitCommand builds the "commit" command group.
func newCommitCommand(info appinfo.Info, g *cli.GlobalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "commit",
		Aliases: []string{"commits"},
		Short:   "List and view Bitbucket commits",
	}
	cmd.AddCommand(
		newCommitListCommand(info, g),
		newCommitViewCommand(info, g),
	)
	return cmd
}

func newCommitListCommand(info appinfo.Info, g *cli.GlobalFlags) *cobra.Command {
	var (
		repoFlag      string
		workspaceFlag string
		revision      string
		limit         int
		all           bool
	)
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List a repository's commit history",
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
			rev := strings.TrimSpace(revision)
			list := bc.ListCommits
			if all {
				list = bc.ListCommitsAll
			}
			raw, err := list(cmd.Context(), target.Workspace, target.Repo, rev, limit)
			if err != nil {
				return err
			}
			if g.WantsStructured() {
				return cli.Render(cmd, g, raw)
			}
			page, err := bitbucket.Decode[bitbucket.CommitPage](raw)
			if err != nil {
				return err
			}
			writeCommitList(cmd.OutOrStdout(), page.Values)
			return nil
		},
	}
	addRepoFlags(cmd, &repoFlag, &workspaceFlag)
	f := cmd.Flags()
	f.StringVar(&revision, "revision", "", "branch, tag, or commit to list history from (defaults to the main branch)")
	cli.AddPaginationFlags(cmd, &limit, &all, "commits")
	return cmd
}

func newCommitViewCommand(info appinfo.Info, g *cli.GlobalFlags) *cobra.Command {
	var (
		repoFlag      string
		workspaceFlag string
	)
	cmd := &cobra.Command{
		Use:   "view <revision>",
		Short: "View a single commit",
		Long:  "View a single commit by hash, branch, or tag.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			revision := strings.TrimSpace(args[0])
			if revision == "" {
				return apperr.InvalidInput("a commit revision is required")
			}
			target, err := resolveRepoTarget(nil, repoFlag, workspaceFlag)
			if err != nil {
				return err
			}
			bc, err := bbClient(info, g)
			if err != nil {
				return err
			}
			raw, err := bc.GetCommit(cmd.Context(), target.Workspace, target.Repo, revision)
			if err != nil {
				return err
			}
			if g.WantsStructured() {
				return cli.Render(cmd, g, raw)
			}
			commit, err := bitbucket.Decode[bitbucket.Commit](raw)
			if err != nil {
				return err
			}
			writeCommit(cmd.OutOrStdout(), commit)
			return nil
		},
	}
	addRepoFlags(cmd, &repoFlag, &workspaceFlag)
	return cmd
}

// writeCommitList prints commits as aligned short-hash/summary/author rows.
func writeCommitList(w io.Writer, commits []bitbucket.Commit) {
	if len(commits) == 0 {
		fmt.Fprintln(w, "No commits found.")
		return
	}
	tw := output.TabWriter(w)
	for _, c := range commits {
		fmt.Fprintf(tw, "%s\t%s\t%s\n", shortHash(c.Hash), commitSummary(c), commitAuthorLabel(c.Author))
	}
	_ = tw.Flush()
}

// writeCommit prints a single commit as aligned label/value lines.
func writeCommit(w io.Writer, c bitbucket.Commit) {
	fmt.Fprintf(w, "%-10s %s\n", "hash:", c.Hash)
	if author := commitAuthorLabel(c.Author); author != "" {
		fmt.Fprintf(w, "%-10s %s\n", "author:", author)
	}
	if c.Date != "" {
		fmt.Fprintf(w, "%-10s %s\n", "date:", c.Date)
	}
	if summary := commitSummary(c); summary != "" {
		fmt.Fprintf(w, "%-10s %s\n", "message:", summary)
	}
}

// shortHash truncates a commit hash to the conventional 12-character prefix.
func shortHash(hash string) string {
	if len(hash) > 12 {
		return hash[:12]
	}
	return hash
}

// commitSummary returns the first line of a commit's message, preferring the
// summary's raw markup and falling back to the plain message field.
func commitSummary(c bitbucket.Commit) string {
	msg := c.Message
	if c.Summary != nil && c.Summary.Raw != "" {
		msg = c.Summary.Raw
	}
	if line, _, ok := strings.Cut(msg, "\n"); ok {
		return strings.TrimSpace(line)
	}
	return strings.TrimSpace(msg)
}

// commitAuthorLabel renders a commit author, preferring the linked account's
// display name and falling back to the raw "Name <email>" string.
func commitAuthorLabel(a *bitbucket.CommitAuthor) string {
	if a == nil {
		return ""
	}
	if label := accountLabel(a.User); label != "" {
		return label
	}
	return strings.TrimSpace(a.Raw)
}
