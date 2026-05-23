// Package confcmd assembles the Confluence-specific command tree for the
// atl-conf binary: space, page, attachment, search, and status. The shared commands (auth,
// api, resolve, browse, version) are built by internal/cli; AddCommands layers
// the Confluence product commands on top of that root.
package confcmd

import (
	"github.com/spf13/cobra"

	"github.com/aurokin/atlassian-cli/internal/appinfo"
	"github.com/aurokin/atlassian-cli/internal/cli"
	"github.com/aurokin/atlassian-cli/internal/conf"
)

// AddCommands registers the Confluence product commands on the atl-conf root.
func AddCommands(root *cobra.Command, info appinfo.Info, g *cli.GlobalFlags) {
	root.AddCommand(
		newSpaceCommand(info, g),
		newPageCommand(info, g),
		newAttachmentCommand(info, g),
		newSearchCommand(info, g),
		newStatusCommand(info, g),
	)
}

// confClient builds a typed Confluence client for the profile named by --site.
func confClient(info appinfo.Info, g *cli.GlobalFlags) (*conf.Client, error) {
	c, err := cli.SiteClient(info, g)
	if err != nil {
		return nil, err
	}
	return conf.New(c), nil
}
