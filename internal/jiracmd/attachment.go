package jiracmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/aurokin/atlassian-cli/internal/apperr"
	"github.com/aurokin/atlassian-cli/internal/appinfo"
	"github.com/aurokin/atlassian-cli/internal/cli"
	"github.com/aurokin/atlassian-cli/internal/jira"
	"github.com/aurokin/atlassian-cli/internal/output"
)

// newAttachmentCommand builds the "issue attachment" command group.
func newAttachmentCommand(info appinfo.Info, g *cli.GlobalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "attachment",
		Short: "List, download, and add Jira issue attachments",
	}
	cmd.AddCommand(
		newAttachmentListCommand(info, g),
		newAttachmentDownloadCommand(info, g),
		newAttachmentAddCommand(info, g),
	)
	return cmd
}

func newAttachmentListCommand(info appinfo.Info, g *cli.GlobalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list <issue>",
		Short: "List the attachments on an issue",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			jc, err := jiraClient(info, g)
			if err != nil {
				return err
			}
			raw, err := jc.ListIssueAttachments(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			// Jira returns the issue with only fields.attachment populated;
			// lift that array out so --json shows the attachments (with every
			// upstream field) rather than the enclosing issue.
			attachments, err := extractAttachments(raw)
			if err != nil {
				return err
			}
			return cli.RenderDecoded(cmd, g, attachments, jira.Decode[[]jira.Attachment],
				writeAttachmentList)
		},
	}
	return cmd
}

// extractAttachments pulls the raw fields.attachment array out of an issue body
// returned by ListIssueAttachments, preserving every upstream field. A missing
// or null attachment field yields an empty JSON array.
func extractAttachments(issue json.RawMessage) (json.RawMessage, error) {
	var wrapper struct {
		Fields struct {
			Attachment json.RawMessage `json:"attachment"`
		} `json:"fields"`
	}
	if err := json.Unmarshal(issue, &wrapper); err != nil {
		return nil, apperr.New(apperr.CodeResponseDecodeFailed,
			"could not decode the Jira API response: "+err.Error())
	}
	if len(wrapper.Fields.Attachment) == 0 || string(wrapper.Fields.Attachment) == "null" {
		return json.RawMessage("[]"), nil
	}
	return wrapper.Fields.Attachment, nil
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
			jc, err := jiraClient(info, g)
			if err != nil {
				return err
			}
			raw, err := jc.GetAttachment(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			if renderMeta {
				return cli.Render(cmd, g, raw)
			}
			att, err := jira.Decode[jira.Attachment](raw)
			if err != nil {
				return err
			}
			data, err := jc.FetchAttachmentData(cmd.Context(), att.Content)
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

func newAttachmentAddCommand(info appinfo.Info, g *cli.GlobalFlags) *cobra.Command {
	var file string
	cmd := &cobra.Command{
		Use:   "add <issue> --file <path>",
		Short: "Upload a file as an attachment on an issue",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if file == "" {
				return apperr.InvalidInput("attachment add requires --file")
			}
			f, err := os.Open(file)
			if err != nil {
				return apperr.New("file_read_failed",
					fmt.Sprintf("open %s: %v", file, err))
			}
			defer func() { _ = f.Close() }()
			jc, err := jiraClient(info, g)
			if err != nil {
				return err
			}
			raw, err := jc.AddAttachment(cmd.Context(), args[0], filepath.Base(file), f)
			if err != nil {
				return err
			}
			return cli.RenderDecoded(cmd, g, raw, jira.Decode[[]jira.Attachment],
				func(w io.Writer, added []jira.Attachment) {
					for _, a := range added {
						fmt.Fprintf(w, "uploaded %s (id %s)\n", a.Filename, a.ID)
					}
				})
		},
	}
	cmd.Flags().StringVar(&file, "file", "", "path to the file to upload (required)")
	return cmd
}

// writeAttachmentList prints attachments as aligned id/mime-type/size/filename
// rows.
func writeAttachmentList(w io.Writer, attachments []jira.Attachment) {
	if len(attachments) == 0 {
		fmt.Fprintln(w, "No attachments found.")
		return
	}
	tw := output.TabWriter(w)
	for _, a := range attachments {
		fmt.Fprintf(tw, "%s\t%s\t%d\t%s\n", a.ID, a.MimeType, a.Size, a.Filename)
	}
	_ = tw.Flush()
}
