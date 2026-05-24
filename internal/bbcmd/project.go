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

// newProjectCommand builds the "project" command group.
func newProjectCommand(info appinfo.Info, g *cli.GlobalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "project",
		Aliases: []string{"projects"},
		Short:   "List, view, create, and delete Bitbucket projects",
	}
	cmd.AddCommand(
		newProjectListCommand(info, g),
		newProjectViewCommand(info, g),
		newProjectCreateCommand(info, g),
		newProjectDeleteCommand(info, g),
	)
	return cmd
}

func newProjectListCommand(info appinfo.Info, g *cli.GlobalFlags) *cobra.Command {
	var (
		workspaceFlag string
		limit         int
		all           bool
	)
	cmd := &cobra.Command{
		Use:   "list [<workspace>]",
		Short: "List a workspace's projects",
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
			list := bc.ListProjects
			if all {
				list = bc.ListProjectsAll
				limit = allPageSize(limit)
			}
			raw, err := list(cmd.Context(), workspace, limit)
			if err != nil {
				return err
			}
			return cli.RenderDecoded(cmd, g, raw, bitbucket.Decode[bitbucket.ProjectPage],
				func(w io.Writer, page bitbucket.ProjectPage) {
					writeProjectList(w, page.Values)
				})
		},
	}
	f := cmd.Flags()
	f.StringVar(&workspaceFlag, "workspace", "", "workspace slug to list projects from")
	cli.AddPaginationFlags(cmd, &limit, &all, "projects")
	return cmd
}

func newProjectViewCommand(info appinfo.Info, g *cli.GlobalFlags) *cobra.Command {
	var workspaceFlag string
	cmd := &cobra.Command{
		Use:   "view <project-key>",
		Short: "View a single project",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			workspace, err := resolveWorkspace(nil, workspaceFlag)
			if err != nil {
				return err
			}
			bc, err := bbClient(info, g)
			if err != nil {
				return err
			}
			raw, err := bc.GetProject(cmd.Context(), workspace, args[0])
			if err != nil {
				return err
			}
			return cli.RenderDecoded(cmd, g, raw, bitbucket.Decode[bitbucket.Project], writeProject)
		},
	}
	cmd.Flags().StringVar(&workspaceFlag, "workspace", "", "workspace slug the project belongs to (required)")
	return cmd
}

func newProjectCreateCommand(info appinfo.Info, g *cli.GlobalFlags) *cobra.Command {
	var (
		workspaceFlag string
		name          string
		description   string
		private       bool
	)
	cmd := &cobra.Command{
		Use:   "create <project-key>",
		Short: "Create a project in a workspace",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(name) == "" {
				return apperr.InvalidInput("a name is required; pass --name")
			}
			workspace, err := resolveWorkspace(nil, workspaceFlag)
			if err != nil {
				return err
			}
			bc, err := bbClient(info, g)
			if err != nil {
				return err
			}
			opts := bitbucket.CreateProjectOptions{Name: name, Description: description}
			// Only forward --private when the user set it, so an unset flag
			// leaves Bitbucket's default visibility in place.
			if cmd.Flags().Changed("private") {
				opts.IsPrivate = &private
			}
			raw, err := bc.CreateProject(cmd.Context(), workspace, args[0], opts)
			if err != nil {
				return err
			}
			return cli.RenderDecoded(cmd, g, raw, bitbucket.Decode[bitbucket.Project],
				func(w io.Writer, p bitbucket.Project) {
					fmt.Fprintf(w, "created project %s: %s\n", p.Key, p.Name)
				})
		},
	}
	f := cmd.Flags()
	f.StringVar(&workspaceFlag, "workspace", "", "workspace slug to create the project in (required)")
	f.StringVar(&name, "name", "", "project name (required)")
	f.StringVar(&description, "description", "", "project description")
	f.BoolVar(&private, "private", false, "create the project as private")
	return cmd
}

func newProjectDeleteCommand(info appinfo.Info, g *cli.GlobalFlags) *cobra.Command {
	var (
		workspaceFlag string
		yes           bool
	)
	cmd := &cobra.Command{
		Use:   "delete <project-key>",
		Short: "Delete a project (irreversible)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !yes {
				return apperr.InvalidInput("deleting a project is irreversible; pass --yes to confirm")
			}
			workspace, err := resolveWorkspace(nil, workspaceFlag)
			if err != nil {
				return err
			}
			bc, err := bbClient(info, g)
			if err != nil {
				return err
			}
			if err := bc.DeleteProject(cmd.Context(), workspace, args[0]); err != nil {
				return err
			}
			if g.WantsStructured() {
				return cli.Render(cmd, g, deleteResult{Resource: "project", ID: args[0], Deleted: true})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "deleted project %s\n", args[0])
			return nil
		},
	}
	cmd.Flags().StringVar(&workspaceFlag, "workspace", "", "workspace slug the project belongs to (required)")
	cmd.Flags().BoolVar(&yes, "yes", false, "confirm the irreversible deletion")
	return cmd
}

// writeProjectList prints projects as aligned key/name rows.
func writeProjectList(w io.Writer, projects []bitbucket.Project) {
	if len(projects) == 0 {
		fmt.Fprintln(w, "No projects found.")
		return
	}
	tw := output.TabWriter(w)
	for _, p := range projects {
		fmt.Fprintf(tw, "%s\t%s\n", p.Key, p.Name)
	}
	_ = tw.Flush()
}

// writeProject prints a single project as aligned label/value lines.
func writeProject(w io.Writer, p bitbucket.Project) {
	lw := output.NewLabelWriter(w)
	lw.Add("key", p.Key)
	lw.AddIf("name", p.Name)
	lw.Add("visibility", visibilityLabel(p.IsPrivate))
	lw.AddIf("description", p.Description)
	lw.AddIf("uuid", p.UUID)
	_ = lw.Flush()
}
