package jiracmd

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/aurokin/atlassian-cli/internal/appinfo"
	"github.com/aurokin/atlassian-cli/internal/cli"
	"github.com/aurokin/atlassian-cli/internal/jira"
	"github.com/aurokin/atlassian-cli/internal/output"
)

// newProjectCommand builds the "project" command group.
func newProjectCommand(info appinfo.Info, g *cli.GlobalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "project",
		Short: "List and view Jira projects",
	}
	cmd.AddCommand(
		newProjectListCommand(info, g),
		newProjectViewCommand(info, g),
	)
	return cmd
}

func newProjectListCommand(info appinfo.Info, g *cli.GlobalFlags) *cobra.Command {
	var (
		limit int
		all   bool
	)
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List projects visible to the authenticated account",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			jc, err := jiraClient(info, g)
			if err != nil {
				return err
			}
			search := jc.SearchProjects
			if all {
				search = jc.SearchProjectsAll
			}
			raw, err := search(cmd.Context(), limit)
			if err != nil {
				return err
			}
			return cli.RenderDecoded(cmd, g, raw, jira.Decode[jira.ProjectPage],
				func(w io.Writer, page jira.ProjectPage) {
					writeProjectList(w, page.Values)
				})
		},
	}
	cli.AddPaginationFlags(cmd, &limit, &all, "projects")
	return cmd
}

func newProjectViewCommand(info appinfo.Info, g *cli.GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "view <key>",
		Short: "View a single Jira project",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			jc, err := jiraClient(info, g)
			if err != nil {
				return err
			}
			raw, err := jc.GetProject(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			return cli.RenderDecoded(cmd, g, raw, jira.Decode[jira.Project], writeProject)
		},
	}
}

// writeProjectList prints projects as aligned key/name rows.
func writeProjectList(w io.Writer, projects []jira.Project) {
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
func writeProject(w io.Writer, p jira.Project) {
	lw := output.NewLabelWriter(w)
	lw.Add("key", p.Key)
	lw.Add("name", p.Name)
	lw.AddIf("type", p.ProjectTypeKey)
	if p.Lead != nil && p.Lead.DisplayName != "" {
		lw.Add("lead", p.Lead.DisplayName)
	}
	lw.AddIf("id", p.ID)
	_ = lw.Flush()
}
