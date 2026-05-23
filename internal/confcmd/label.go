package confcmd

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/aurokin/atlassian-cli/internal/appinfo"
	"github.com/aurokin/atlassian-cli/internal/cli"
	"github.com/aurokin/atlassian-cli/internal/conf"
	"github.com/aurokin/atlassian-cli/internal/output"
)

// newPageLabelCommand builds the "page label" sub-group, which operates on a
// page's content labels.
func newPageLabelCommand(info appinfo.Info, g *cli.GlobalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "label",
		Short: "List and manage a page's labels",
	}
	cmd.AddCommand(
		newPageLabelListCommand(info, g),
		newPageLabelAddCommand(info, g),
		newPageLabelRemoveCommand(info, g),
	)
	return cmd
}

func newPageLabelListCommand(info appinfo.Info, g *cli.GlobalFlags) *cobra.Command {
	var (
		limit int
		all   bool
	)
	cmd := &cobra.Command{
		Use:   "list <page-id>",
		Short: "List the labels on a page",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cc, err := confClient(info, g)
			if err != nil {
				return err
			}
			list := cc.ListLabels
			if all {
				list = cc.ListLabelsAll
			}
			raw, err := list(cmd.Context(), args[0], limit)
			if err != nil {
				return err
			}
			return cli.RenderDecoded(cmd, g, raw, conf.Decode[conf.LabelList],
				func(w io.Writer, ll conf.LabelList) {
					writeLabelList(w, ll.Results)
				})
		},
	}
	cli.AddPaginationFlags(cmd, &limit, &all, "labels")
	return cmd
}

func newPageLabelAddCommand(info appinfo.Info, g *cli.GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "add <page-id> <label>",
		Short: "Attach a label to a page",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			cc, err := confClient(info, g)
			if err != nil {
				return err
			}
			raw, err := cc.AddLabel(cmd.Context(), args[0], args[1])
			if err != nil {
				return err
			}
			if g.WantsStructured() {
				return cli.Render(cmd, g, raw)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "added label %s to page %s\n", args[1], args[0])
			return nil
		},
	}
}

func newPageLabelRemoveCommand(info appinfo.Info, g *cli.GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "remove <page-id> <label>",
		Short: "Detach a label from a page",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			cc, err := confClient(info, g)
			if err != nil {
				return err
			}
			if err := cc.RemoveLabel(cmd.Context(), args[0], args[1]); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "removed label %s from page %s\n", args[1], args[0])
			return nil
		},
	}
}

// writeLabelList prints labels as aligned prefix/name rows.
func writeLabelList(w io.Writer, labels []conf.Label) {
	if len(labels) == 0 {
		fmt.Fprintln(w, "No labels found.")
		return
	}
	tw := output.TabWriter(w)
	for _, l := range labels {
		fmt.Fprintf(tw, "%s\t%s\n", l.Prefix, l.Name)
	}
	_ = tw.Flush()
}
