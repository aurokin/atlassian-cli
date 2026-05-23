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

// newDeploymentCommand builds the "deployment" command group.
func newDeploymentCommand(info appinfo.Info, g *cli.GlobalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "deployment",
		Aliases: []string{"deployments", "deploy"},
		Short:   "List and view Bitbucket deployments",
	}
	cmd.AddCommand(
		newDeploymentListCommand(info, g),
		newDeploymentViewCommand(info, g),
	)
	return cmd
}

func newDeploymentListCommand(info appinfo.Info, g *cli.GlobalFlags) *cobra.Command {
	var (
		repoFlag      string
		workspaceFlag string
		limit         int
		all           bool
	)
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List a repository's deployments",
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
			list := bc.ListDeployments
			if all {
				list = bc.ListDeploymentsAll
			}
			raw, err := list(cmd.Context(), target.Workspace, target.Repo, limit)
			if err != nil {
				return err
			}
			if g.JSON != "" || g.JQ != "" {
				return cli.Render(cmd, g, raw)
			}
			page, err := bitbucket.Decode[bitbucket.DeploymentPage](raw)
			if err != nil {
				return err
			}
			writeDeploymentList(cmd.OutOrStdout(), page.Values)
			return nil
		},
	}
	addRepoFlags(cmd, &repoFlag, &workspaceFlag)
	cli.AddPaginationFlags(cmd, &limit, &all, "deployments")
	return cmd
}

func newDeploymentViewCommand(info appinfo.Info, g *cli.GlobalFlags) *cobra.Command {
	var (
		repoFlag      string
		workspaceFlag string
	)
	cmd := &cobra.Command{
		Use:   "view <uuid>",
		Short: "View a single deployment",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(args[0]) == "" {
				return apperr.InvalidInput("a deployment UUID is required")
			}
			target, err := resolveRepoTarget(nil, repoFlag, workspaceFlag)
			if err != nil {
				return err
			}
			bc, err := bbClient(info, g)
			if err != nil {
				return err
			}
			raw, err := bc.GetDeployment(cmd.Context(), target.Workspace, target.Repo, args[0])
			if err != nil {
				return err
			}
			if g.JSON != "" || g.JQ != "" {
				return cli.Render(cmd, g, raw)
			}
			d, err := bitbucket.Decode[bitbucket.Deployment](raw)
			if err != nil {
				return err
			}
			writeDeployment(cmd.OutOrStdout(), d)
			return nil
		},
	}
	addRepoFlags(cmd, &repoFlag, &workspaceFlag)
	return cmd
}

// newEnvironmentCommand builds the "environment" command group.
func newEnvironmentCommand(info appinfo.Info, g *cli.GlobalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "environment",
		Aliases: []string{"environments", "env"},
		Short:   "List and view Bitbucket deployment environments",
	}
	cmd.AddCommand(
		newEnvironmentListCommand(info, g),
		newEnvironmentViewCommand(info, g),
	)
	return cmd
}

func newEnvironmentListCommand(info appinfo.Info, g *cli.GlobalFlags) *cobra.Command {
	var (
		repoFlag      string
		workspaceFlag string
		limit         int
		all           bool
	)
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List a repository's deployment environments",
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
			list := bc.ListEnvironments
			if all {
				list = bc.ListEnvironmentsAll
			}
			raw, err := list(cmd.Context(), target.Workspace, target.Repo, limit)
			if err != nil {
				return err
			}
			if g.JSON != "" || g.JQ != "" {
				return cli.Render(cmd, g, raw)
			}
			page, err := bitbucket.Decode[bitbucket.EnvironmentPage](raw)
			if err != nil {
				return err
			}
			writeEnvironmentList(cmd.OutOrStdout(), page.Values)
			return nil
		},
	}
	addRepoFlags(cmd, &repoFlag, &workspaceFlag)
	cli.AddPaginationFlags(cmd, &limit, &all, "environments")
	return cmd
}

func newEnvironmentViewCommand(info appinfo.Info, g *cli.GlobalFlags) *cobra.Command {
	var (
		repoFlag      string
		workspaceFlag string
	)
	cmd := &cobra.Command{
		Use:   "view <uuid>",
		Short: "View a single deployment environment",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(args[0]) == "" {
				return apperr.InvalidInput("an environment UUID is required")
			}
			target, err := resolveRepoTarget(nil, repoFlag, workspaceFlag)
			if err != nil {
				return err
			}
			bc, err := bbClient(info, g)
			if err != nil {
				return err
			}
			raw, err := bc.GetEnvironment(cmd.Context(), target.Workspace, target.Repo, args[0])
			if err != nil {
				return err
			}
			if g.JSON != "" || g.JQ != "" {
				return cli.Render(cmd, g, raw)
			}
			env, err := bitbucket.Decode[bitbucket.Environment](raw)
			if err != nil {
				return err
			}
			writeEnvironment(cmd.OutOrStdout(), env)
			return nil
		},
	}
	addRepoFlags(cmd, &repoFlag, &workspaceFlag)
	return cmd
}

// writeDeploymentList prints deployments as aligned uuid/state/environment rows.
func writeDeploymentList(w io.Writer, deployments []bitbucket.Deployment) {
	if len(deployments) == 0 {
		fmt.Fprintln(w, "No deployments found.")
		return
	}
	tw := output.TabWriter(w)
	for _, d := range deployments {
		fmt.Fprintf(tw, "%s\t%s\t%s\n", d.UUID, deploymentState(d.State), deploymentEnvironment(d.Environment))
	}
	_ = tw.Flush()
}

// writeDeployment prints a single deployment as aligned label/value lines.
func writeDeployment(w io.Writer, d bitbucket.Deployment) {
	fmt.Fprintf(w, "%-12s %s\n", "uuid:", d.UUID)
	if s := deploymentState(d.State); s != "" {
		fmt.Fprintf(w, "%-12s %s\n", "state:", s)
	}
	if env := deploymentEnvironment(d.Environment); env != "" {
		fmt.Fprintf(w, "%-12s %s\n", "environment:", env)
	}
	if d.Release != nil && d.Release.Name != "" {
		fmt.Fprintf(w, "%-12s %s\n", "release:", d.Release.Name)
	}
}

// writeEnvironmentList prints environments as aligned name/type/uuid rows.
func writeEnvironmentList(w io.Writer, envs []bitbucket.Environment) {
	if len(envs) == 0 {
		fmt.Fprintln(w, "No environments found.")
		return
	}
	tw := output.TabWriter(w)
	for _, e := range envs {
		fmt.Fprintf(tw, "%s\t%s\t%s\n", e.Name, e.Type, e.UUID)
	}
	_ = tw.Flush()
}

// writeEnvironment prints a single environment as aligned label/value lines.
func writeEnvironment(w io.Writer, e bitbucket.Environment) {
	fmt.Fprintf(w, "%-8s %s\n", "name:", e.Name)
	if e.Slug != "" {
		fmt.Fprintf(w, "%-8s %s\n", "slug:", e.Slug)
	}
	if e.Type != "" {
		fmt.Fprintf(w, "%-8s %s\n", "type:", e.Type)
	}
	if e.UUID != "" {
		fmt.Fprintf(w, "%-8s %s\n", "uuid:", e.UUID)
	}
}

// deploymentState renders a deployment state's name, or "".
func deploymentState(s *bitbucket.DeploymentState) string {
	if s == nil {
		return ""
	}
	return s.Name
}

// deploymentEnvironment renders a deployment's environment name, or "".
func deploymentEnvironment(e *bitbucket.Environment) string {
	if e == nil {
		return ""
	}
	return e.Name
}
