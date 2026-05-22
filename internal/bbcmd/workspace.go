package bbcmd

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/aurokin/atlassian-cli/internal/appinfo"
	"github.com/aurokin/atlassian-cli/internal/bitbucket"
	"github.com/aurokin/atlassian-cli/internal/cli"
	"github.com/aurokin/atlassian-cli/internal/output"
)

// newWorkspaceCommand builds the "workspace" command group.
func newWorkspaceCommand(info appinfo.Info, g *cli.GlobalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "workspace",
		Aliases: []string{"workspaces", "ws"},
		Short:   "List and view Bitbucket workspaces",
	}
	cmd.AddCommand(
		newWorkspaceListCommand(info, g),
		newWorkspaceViewCommand(info, g),
	)
	return cmd
}

func newWorkspaceListCommand(info appinfo.Info, g *cli.GlobalFlags) *cobra.Command {
	var (
		limit int
		all   bool
	)
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List the workspaces the account is a member of",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			bc, err := bbClient(info, g)
			if err != nil {
				return err
			}
			list := bc.ListWorkspaces
			if all {
				list = bc.ListWorkspacesAll
			}
			raw, err := list(cmd.Context(), limit)
			if err != nil {
				return err
			}
			if g.JSON != "" || g.JQ != "" {
				return cli.Render(cmd, g, raw)
			}
			page, err := bitbucket.Decode[bitbucket.WorkspacePage](raw)
			if err != nil {
				return err
			}
			writeWorkspaceList(cmd.OutOrStdout(), page.Values)
			return nil
		},
	}
	f := cmd.Flags()
	f.IntVar(&limit, "limit", 0, "maximum number of workspaces per page")
	f.BoolVar(&all, "all", false, "follow pagination and return every page (--limit sets the page size)")
	return cmd
}

func newWorkspaceViewCommand(info appinfo.Info, g *cli.GlobalFlags) *cobra.Command {
	var workspaceFlag string
	cmd := &cobra.Command{
		Use:   "view [<workspace>]",
		Short: "View a single workspace",
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
			raw, err := bc.GetWorkspace(cmd.Context(), workspace)
			if err != nil {
				return err
			}
			if g.JSON != "" || g.JQ != "" {
				return cli.Render(cmd, g, raw)
			}
			ws, err := bitbucket.Decode[bitbucket.Workspace](raw)
			if err != nil {
				return err
			}
			writeWorkspace(cmd.OutOrStdout(), ws)
			return nil
		},
	}
	cmd.Flags().StringVar(&workspaceFlag, "workspace", "", "workspace slug to view")
	return cmd
}

// writeWorkspaceList prints workspaces as aligned slug/name rows.
func writeWorkspaceList(w io.Writer, workspaces []bitbucket.Workspace) {
	if len(workspaces) == 0 {
		fmt.Fprintln(w, "No workspaces found.")
		return
	}
	tw := output.TabWriter(w)
	for _, ws := range workspaces {
		fmt.Fprintf(tw, "%s\t%s\n", ws.Slug, ws.Name)
	}
	_ = tw.Flush()
}

// writeWorkspace prints a single workspace as aligned label/value lines.
func writeWorkspace(w io.Writer, ws bitbucket.Workspace) {
	fmt.Fprintf(w, "%-12s %s\n", "slug:", ws.Slug)
	if ws.Name != "" {
		fmt.Fprintf(w, "%-12s %s\n", "name:", ws.Name)
	}
	fmt.Fprintf(w, "%-12s %s\n", "visibility:", visibilityLabel(ws.IsPrivate))
	if ws.UUID != "" {
		fmt.Fprintf(w, "%-12s %s\n", "uuid:", ws.UUID)
	}
	if ws.CreatedOn != "" {
		fmt.Fprintf(w, "%-12s %s\n", "created:", ws.CreatedOn)
	}
}

// visibilityLabel renders a private flag as "private"/"public".
func visibilityLabel(private bool) string {
	if private {
		return "private"
	}
	return "public"
}
