// Package bbcmd assembles the Bitbucket-specific command tree for the atl-bb
// binary. The shared commands (auth, api, resolve, browse, version) are built
// by internal/cli; AddCommands layers the Bitbucket product commands on top of
// that root, mirroring internal/jiracmd and internal/confcmd.
package bbcmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/aurokin/atlassian-cli/internal/apperr"
	"github.com/aurokin/atlassian-cli/internal/appinfo"
	"github.com/aurokin/atlassian-cli/internal/bitbucket"
	"github.com/aurokin/atlassian-cli/internal/cli"
)

// AddCommands registers the Bitbucket product commands on the atl-bb root.
func AddCommands(root *cobra.Command, info appinfo.Info, g *cli.GlobalFlags) {
	root.AddCommand(
		newRepoCommand(info, g),
		newPRCommand(info, g),
		newPipelineCommand(info, g),
	)
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
// positional argument wins when both it and --repo are supplied. Git-checkout
// inference is deferred to a later slice (B3c).
func resolveRepoTarget(args []string, repoFlag, workspaceFlag string) (repoTarget, error) {
	raw := strings.TrimSpace(repoFlag)
	if len(args) == 1 {
		raw = strings.TrimSpace(args[0])
	}
	if raw == "" {
		return repoTarget{}, apperr.InvalidInput(
			"a repository is required; pass it as <workspace>/<repo> (positional or --repo), optionally with --workspace")
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
// (matching resolveRepoTarget's conflict handling).
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
		return "", apperr.InvalidInput(
			"a workspace is required; pass it as a positional argument or --workspace")
	}
	if strings.Contains(ws, "/") {
		return "", apperr.InvalidInput(
			fmt.Sprintf("invalid workspace %q; a workspace slug must not contain '/'", ws))
	}
	return ws, nil
}
