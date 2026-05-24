package confcmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/aurokin/atlassian-cli/internal/apperr"
	"github.com/aurokin/atlassian-cli/internal/appinfo"
	"github.com/aurokin/atlassian-cli/internal/cli"
	"github.com/aurokin/atlassian-cli/internal/conf"
	"github.com/aurokin/atlassian-cli/internal/output"
)

// newAttachmentCommand builds the "attachment" command group.
func newAttachmentCommand(info appinfo.Info, g *cli.GlobalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "attachment",
		Short: "List, download, and upload Confluence attachments",
	}
	cmd.AddCommand(
		newAttachmentListCommand(info, g),
		newAttachmentDownloadCommand(info, g),
		newAttachmentUploadCommand(info, g),
	)
	return cmd
}

func newAttachmentUploadCommand(info appinfo.Info, g *cli.GlobalFlags) *cobra.Command {
	var file string
	cmd := &cobra.Command{
		Use:   "upload <page-id> --file <path>",
		Short: "Upload a file as an attachment on a page",
		Long: "Uploads --file as an attachment on the page. Confluence v2 has no\n" +
			"attachment-create endpoint, so this uses the REST v1 surface. The API\n" +
			"rejects a file whose name already exists on the page.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if file == "" {
				return apperr.InvalidInput("a file is required; pass --file")
			}
			f, err := os.Open(file)
			if err != nil {
				return apperr.New("file_read_failed",
					fmt.Sprintf("open attachment file %s: %v", file, err))
			}
			defer func() { _ = f.Close() }()
			cc, err := confClient(info, g)
			if err != nil {
				return err
			}
			raw, err := cc.CreateAttachment(cmd.Context(), args[0], filepath.Base(file), f)
			if err != nil {
				return err
			}
			if g.WantsStructured() {
				return cli.Render(cmd, g, raw)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "uploaded %s to page %s\n", filepath.Base(file), args[0])
			return nil
		},
	}
	cmd.Flags().StringVar(&file, "file", "", "path to the file to upload (required)")
	return cmd
}

func newAttachmentListCommand(info appinfo.Info, g *cli.GlobalFlags) *cobra.Command {
	var (
		limit int
		all   bool
	)
	cmd := &cobra.Command{
		Use:   "list <page-id>",
		Short: "List the attachments on a page",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cc, err := confClient(info, g)
			if err != nil {
				return err
			}
			list := cc.ListAttachments
			if all {
				list = cc.ListAttachmentsAll
			}
			raw, err := list(cmd.Context(), args[0], limit)
			if err != nil {
				return err
			}
			return cli.RenderDecoded(cmd, g, raw, conf.Decode[conf.AttachmentList],
				func(w io.Writer, al conf.AttachmentList) {
					writeAttachmentList(w, al.Results)
				})
		},
	}
	cli.AddPaginationFlags(cmd, &limit, &all, "attachments")
	return cmd
}

func newAttachmentDownloadCommand(info appinfo.Info, g *cli.GlobalFlags) *cobra.Command {
	var out string
	cmd := &cobra.Command{
		Use:   "download <attachment-id>",
		Short: "Download an attachment's binary content",
		Long: "Downloads an attachment to the path given by --out; pass --out - to\n" +
			"stream the bytes to stdout. With --json or --jq the attachment\n" +
			"metadata is printed instead and no download occurs.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			renderMeta := g.WantsStructured()
			if !renderMeta && out == "" {
				return apperr.InvalidInput(
					"attachment download requires --out (use --out - to stream to stdout)")
			}
			cc, err := confClient(info, g)
			if err != nil {
				return err
			}
			raw, err := cc.GetAttachment(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			if renderMeta {
				return cli.Render(cmd, g, raw)
			}
			att, err := conf.Decode[conf.Attachment](raw)
			if err != nil {
				return err
			}
			data, err := cc.FetchAttachmentData(cmd.Context(), att.DownloadLink)
			if err != nil {
				return err
			}
			if out == "-" {
				_, err := cmd.OutOrStdout().Write(data)
				return err
			}
			if err := os.WriteFile(out, data, 0o644); err != nil {
				return apperr.New("file_write_failed",
					fmt.Sprintf("write attachment to %s: %v", out, err))
			}
			fmt.Fprintf(cmd.OutOrStdout(), "downloaded attachment %s to %s (%d bytes)\n",
				att.ID, out, len(data))
			return nil
		},
	}
	cmd.Flags().StringVar(&out, "out", "",
		"destination file path, or - to stream to stdout (required)")
	return cmd
}

// writeAttachmentList prints attachments as aligned id/media-type/size/title
// rows.
func writeAttachmentList(w io.Writer, attachments []conf.Attachment) {
	if len(attachments) == 0 {
		fmt.Fprintln(w, "No attachments found.")
		return
	}
	tw := output.TabWriter(w)
	for _, a := range attachments {
		fmt.Fprintf(tw, "%s\t%s\t%d\t%s\n", a.ID, a.MediaType, a.FileSize, a.Title)
	}
	_ = tw.Flush()
}
