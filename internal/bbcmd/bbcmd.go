// Package bbcmd assembles the Bitbucket-specific command tree for the atl-bb
// binary. The shared commands (auth, api, resolve, browse, version) are built
// by internal/cli; AddCommands layers the Bitbucket product commands on top of
// that root, mirroring internal/jiracmd and internal/confcmd.
package bbcmd

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/aurokin/atlassian-cli/internal/apperr"
	"github.com/aurokin/atlassian-cli/internal/appinfo"
	"github.com/aurokin/atlassian-cli/internal/bitbucket"
	"github.com/aurokin/atlassian-cli/internal/cli"
	"github.com/aurokin/atlassian-cli/internal/git"
)

// inferRepoTarget infers a workspace/repo from the local git checkout. It is a
// package variable so tests can stub inference without a real git repository.
// A false result means inference was not possible (no git repo, no Bitbucket
// remote, etc.), and the caller falls back to requiring an explicit target.
var inferRepoTarget = func() (repoTarget, bool) {
	rt, ok := git.InferBitbucketRepo(context.Background(), ".")
	if !ok {
		return repoTarget{}, false
	}
	return repoTarget{Workspace: rt.Workspace, Repo: rt.Repo}, true
}

// parsePositiveInt parses a positive integer, returning an error for a
// non-integer or non-positive value. Callers wrap it with a domain-specific
// message (e.g. "invalid issue id").
func parsePositiveInt(s string) (int, error) {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("not a positive integer: %q", s)
	}
	return n, nil
}

// AddCommands registers the Bitbucket product commands on the atl-bb root.
func AddCommands(root *cobra.Command, info appinfo.Info, g *cli.GlobalFlags) {
	cli.AddProductCommands(root,
		newRepoCommand(info, g),
		newPRCommand(info, g),
		newPipelineCommand(info, g),
		newIssueCommand(info, g),
		newWorkspaceCommand(info, g),
		newProjectCommand(info, g),
		newCommitCommand(info, g),
		newSourceCommand(info, g),
		newFileCommand(info, g),
		newBranchCommand(info, g),
		newTagCommand(info, g),
		newDeploymentCommand(info, g),
		newEnvironmentCommand(info, g),
		newSearchCommand(info, g),
		newStatusCommand(info, g),
	)
}

// allPageSize picks the per-page size for an --all request: the explicit
// --limit when the caller set one, otherwise Bitbucket's maximum page size so
// the page follow makes the fewest round-trips and is least likely to hit the
// page-follow cap. A non-positive limit means "unset" (the --limit default).
func allPageSize(limit int) int {
	if limit > 0 {
		return limit
	}
	return bitbucket.MaxPageLen
}

// bbClient builds a typed Bitbucket client for the profile named by --site.
func bbClient(info appinfo.Info, g *cli.GlobalFlags) (*bitbucket.Client, error) {
	c, err := cli.SiteClient(info, g)
	if err != nil {
		return nil, err
	}
	return bitbucket.New(c), nil
}

// repoTarget identifies a single Bitbucket repository.
type repoTarget struct {
	Workspace string
	Repo      string
}

// resolveRepoTarget determines the workspace/repo a repo-scoped command acts
// on, honoring the decision-D2 targeting surface: an optional positional
// "<workspace>/<repo>" argument, the --repo flag (same form, or a bare "<repo>"
// paired with --workspace), and --workspace as the workspace fallback. The
// positional argument wins when both it and --repo are supplied. When no
// target is supplied at all (no positional, no --repo, no --workspace), it
// falls back to inferring the workspace/repo from the local git checkout's
// Bitbucket remote.
func resolveRepoTarget(args []string, repoFlag, workspaceFlag string) (repoTarget, error) {
	raw := strings.TrimSpace(repoFlag)
	if len(args) == 1 {
		raw = strings.TrimSpace(args[0])
	}
	if raw == "" {
		// With nothing specified, try git-checkout inference before failing. An
		// explicit --workspace means the caller is targeting deliberately, so
		// inference is skipped and the missing repository is reported.
		if strings.TrimSpace(workspaceFlag) == "" {
			if t, ok := inferRepoTarget(); ok {
				return t, nil
			}
		}
		return repoTarget{}, apperr.InvalidInput(
			"a repository is required; pass it as <workspace>/<repo> (positional or --repo), optionally with --workspace, or run inside a Bitbucket git checkout")
	}

	workspace := strings.TrimSpace(workspaceFlag)
	repo := raw
	if ws, name, ok := strings.Cut(raw, "/"); ok {
		if strings.TrimSpace(ws) == "" || strings.TrimSpace(name) == "" {
			return repoTarget{}, apperr.InvalidInput(
				fmt.Sprintf("invalid repository target %q; expected <workspace>/<repo>", raw))
		}
		// A "<workspace>/<repo>" target is fully qualified; reject a conflicting
		// --workspace so the command can never act on an ambiguous resource.
		if workspace != "" && workspace != strings.TrimSpace(ws) {
			return repoTarget{}, apperr.InvalidInput(
				fmt.Sprintf("--workspace %q conflicts with the workspace in %q", workspace, raw))
		}
		workspace = strings.TrimSpace(ws)
		repo = strings.TrimSpace(name)
	}
	if strings.Contains(repo, "/") {
		return repoTarget{}, apperr.InvalidInput(
			fmt.Sprintf("invalid repository target %q; expected <workspace>/<repo>", raw))
	}
	if workspace == "" {
		return repoTarget{}, apperr.InvalidInput(
			fmt.Sprintf("repository %q has no workspace; use <workspace>/<repo> or pass --workspace", repo))
	}
	return repoTarget{Workspace: workspace, Repo: repo}, nil
}

// resolveWorkspace determines the workspace a workspace-scoped command acts on,
// from an optional positional argument or the --workspace flag. The positional
// and the flag name the same resource, so a positional that conflicts with a
// supplied --workspace is rejected rather than silently overriding it
// (matching resolveRepoTarget's conflict handling). When neither is supplied it
// falls back to the workspace of the local git checkout's Bitbucket remote, so
// workspace-scoped listings (repo list, project list, search repos) work
// in-repo without --workspace.
func resolveWorkspace(args []string, workspaceFlag string) (string, error) {
	flag := strings.TrimSpace(workspaceFlag)
	ws := flag
	if len(args) == 1 {
		arg := strings.TrimSpace(args[0])
		if flag != "" && arg != "" && flag != arg {
			return "", apperr.InvalidInput(
				fmt.Sprintf("--workspace %q conflicts with the workspace argument %q", flag, arg))
		}
		ws = arg
	}
	if ws == "" {
		if t, ok := inferRepoTarget(); ok && t.Workspace != "" {
			return t.Workspace, nil
		}
		return "", apperr.InvalidInput(
			"a workspace is required; pass it as a positional argument or --workspace, or run inside a Bitbucket git checkout")
	}
	if strings.Contains(ws, "/") {
		return "", apperr.InvalidInput(
			fmt.Sprintf("invalid workspace %q; a workspace slug must not contain '/'", ws))
	}
	return ws, nil
}
