package jiracmd

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/aurokin/atlassian-cli/internal/apperr"
	"github.com/aurokin/atlassian-cli/internal/appinfo"
	"github.com/aurokin/atlassian-cli/internal/cli"
	"github.com/aurokin/atlassian-cli/internal/jira"
	"github.com/aurokin/atlassian-cli/internal/output"
)

// newIssueLinkCommand builds the "issue link" command, which both creates a
// new link when given two issue keys and a --type, and groups the `types`
// sub-command that lists the available link types. Jira issue keys always
// contain a hyphen-and-number (`PROJ-1`), so they never collide with the
// `types` sub-command name.
func newIssueLinkCommand(info appinfo.Info, g *cli.GlobalFlags) *cobra.Command {
	var linkType string
	cmd := &cobra.Command{
		Use:   "link <inward> <outward> --type <link-type>",
		Short: "Create a directional link between two issues, or list link types",
		Long: "Creates a link between two issues. The first positional is the\n" +
			"inward issue and the second is the outward issue, matching the\n" +
			"Jira API field names: with --type Blocks, `issue link A B --type\n" +
			"Blocks` means A is blocked by B and B blocks A.\n\n" +
			"`issue link types` lists the available link types with their\n" +
			"inward and outward phrases.",
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if linkType == "" {
				return apperr.InvalidInput("issue link requires --type")
			}
			jc, err := jiraClient(info, g)
			if err != nil {
				return err
			}
			if err := jc.CreateIssueLink(cmd.Context(), args[0], args[1], linkType); err != nil {
				return err
			}
			// Avoid an arrow here: the API field names ("inward"/"outward")
			// don't match the natural arrow reading. Print them explicitly so
			// it is unambiguous which issue carries which role.
			fmt.Fprintf(cmd.OutOrStdout(),
				"created %s link: inward %s, outward %s\n", linkType, args[0], args[1])
			return nil
		},
	}
	cmd.Flags().StringVar(&linkType, "type", "", "link type name, e.g. Blocks (required)")
	cmd.AddCommand(newIssueLinkTypesCommand(info, g))
	return cmd
}

func newIssueLinkTypesCommand(info appinfo.Info, g *cli.GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "types",
		Short: "List the issue link types available on this site",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			jc, err := jiraClient(info, g)
			if err != nil {
				return err
			}
			raw, err := jc.ListIssueLinkTypes(cmd.Context())
			if err != nil {
				return err
			}
			return cli.RenderDecoded(cmd, g, raw, jira.Decode[jira.LinkTypeList],
				func(w io.Writer, lt jira.LinkTypeList) {
					writeLinkTypes(w, lt.Types)
				})
		},
	}
}

// writeLinkTypes prints link types as aligned name/inward/outward rows.
func writeLinkTypes(w io.Writer, types []jira.LinkType) {
	if len(types) == 0 {
		fmt.Fprintln(w, "No link types found.")
		return
	}
	tw := output.TabWriter(w)
	for _, t := range types {
		fmt.Fprintf(tw, "%s\t%s\t%s\n", t.Name, t.Inward, t.Outward)
	}
	_ = tw.Flush()
}
