package bbcmd

import (
	"io"

	"github.com/spf13/cobra"

	"github.com/aurokin/atlassian-cli/internal/appinfo"
	"github.com/aurokin/atlassian-cli/internal/bitbucket"
	"github.com/aurokin/atlassian-cli/internal/cli"
	"github.com/aurokin/atlassian-cli/internal/output"
)

// newWorkspaceCommand builds the "workspace" command group.
//
// There is intentionally no "list" subcommand: Bitbucket removed the
// cross-workspace enumeration endpoint (GET /2.0/workspaces) on 2026-04-14
// (changelog CHANGE-3022), and there is no API-token replacement for listing
// the workspaces an account belongs to. Workspaces are addressed by slug.
func newWorkspaceCommand(info appinfo.Info, g *cli.GlobalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "workspace",
		Aliases: []string{"workspaces", "ws"},
		Short:   "View Bitbucket workspaces",
	}
	cmd.AddCommand(
		newWorkspaceViewCommand(info, g),
	)
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
			return cli.RenderDecoded(cmd, g, raw, bitbucket.Decode[bitbucket.Workspace], writeWorkspace)
		},
	}
	cmd.Flags().StringVar(&workspaceFlag, "workspace", "", "workspace slug to view")
	return cmd
}

// writeWorkspace prints a single workspace as aligned label/value lines.
func writeWorkspace(w io.Writer, ws bitbucket.Workspace) {
	lw := output.NewLabelWriter(w)
	lw.Add("slug", ws.Slug)
	lw.AddIf("name", ws.Name)
	lw.Add("visibility", visibilityLabel(ws.IsPrivate))
	lw.AddIf("uuid", ws.UUID)
	lw.AddIf("created", ws.CreatedOn)
	_ = lw.Flush()
}

// visibilityLabel renders a private flag as "private"/"public".
func visibilityLabel(private bool) string {
	if private {
		return "private"
	}
	return "public"
}
