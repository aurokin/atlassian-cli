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

// newFieldCommand builds the "field" command group, whose `list` sub-command
// discovers the field ids and types accepted by issue create/edit and the
// issue-view --fields selector.
func newFieldCommand(info appinfo.Info, g *cli.GlobalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "field",
		Short: "Discover Jira field ids and types",
	}
	cmd.AddCommand(newFieldListCommand(info, g))
	return cmd
}

func newFieldListCommand(info appinfo.Info, g *cli.GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List the fields available on this site (id, name, type)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			jc, err := jiraClient(info, g)
			if err != nil {
				return err
			}
			raw, err := jc.ListFields(cmd.Context())
			if err != nil {
				return err
			}
			return cli.RenderDecoded(cmd, g, raw, jira.Decode[[]jira.Field], writeFieldList)
		},
	}
}

// writeFieldList prints fields as aligned id/type/scope/name rows. The id
// column is what create/edit's --field and view's --fields expect.
func writeFieldList(w io.Writer, fields []jira.Field) {
	if len(fields) == 0 {
		fmt.Fprintln(w, "No fields found.")
		return
	}
	tw := output.TabWriter(w)
	for _, f := range fields {
		scope := "system"
		if f.Custom {
			scope = "custom"
		}
		typ := f.Schema.Type
		if typ == "" {
			typ = "-"
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", f.ID, typ, scope, f.Name)
	}
	_ = tw.Flush()
}
