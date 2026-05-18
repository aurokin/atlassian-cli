package jiracmd

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/aurokin/atlassian-cli/internal/appinfo"
	"github.com/aurokin/atlassian-cli/internal/cli"
	"github.com/aurokin/atlassian-cli/internal/jira"
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
	var limit int
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List projects visible to the authenticated account",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			jc, err := jiraClient(info, g)
			if err != nil {
				return err
			}
			raw, err := jc.SearchProjects(cmd.Context(), limit)
			if err != nil {
				return err
			}
			if g.JSON != "" || g.JQ != "" {
				return cli.Render(cmd, g, raw)
			}
			page, err := jira.Decode[jira.ProjectPage](raw)
			if err != nil {
				return err
			}
			writeProjectList(cmd.OutOrStdout(), page.Values)
			return nil
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 0, "maximum number of projects to return")
	return cmd
}

func newProjectViewCommand(info appinfo.Info, g *cli.GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "view <KEY>",
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
			if g.JSON != "" || g.JQ != "" {
				return cli.Render(cmd, g, raw)
			}
			p, err := jira.Decode[jira.Project](raw)
			if err != nil {
				return err
			}
			writeProject(cmd.OutOrStdout(), p)
			return nil
		},
	}
}

// writeProjectList prints projects as aligned key/name rows.
func writeProjectList(w io.Writer, projects []jira.Project) {
	if len(projects) == 0 {
		fmt.Fprintln(w, "No projects found.")
		return
	}
	tw := tabWriter(w)
	for _, p := range projects {
		fmt.Fprintf(tw, "%s\t%s\n", p.Key, p.Name)
	}
	_ = tw.Flush()
}

// writeProject prints a single project as aligned label/value lines.
func writeProject(w io.Writer, p jira.Project) {
	fmt.Fprintf(w, "key:   %s\n", p.Key)
	fmt.Fprintf(w, "name:  %s\n", p.Name)
	if p.ProjectTypeKey != "" {
		fmt.Fprintf(w, "type:  %s\n", p.ProjectTypeKey)
	}
	if p.Lead != nil && p.Lead.DisplayName != "" {
		fmt.Fprintf(w, "lead:  %s\n", p.Lead.DisplayName)
	}
	if p.ID != "" {
		fmt.Fprintf(w, "id:    %s\n", p.ID)
	}
}
