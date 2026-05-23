package jiracmd

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/aurokin/atlassian-cli/internal/appinfo"
	"github.com/aurokin/atlassian-cli/internal/cli"
	"github.com/aurokin/atlassian-cli/internal/jira"
)

// newStatusCommand builds the "status" command: a live authentication check
// against the configured site, distinct from the offline "auth status".
func newStatusCommand(info appinfo.Info, g *cli.GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Check authentication against the configured site",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			jc, err := jiraClient(info, g)
			if err != nil {
				return err
			}
			raw, err := jc.Myself(cmd.Context())
			if err != nil {
				return err
			}
			if g.WantsStructured() {
				return cli.Render(cmd, g, raw)
			}
			user, err := jira.Decode[jira.User](raw)
			if err != nil {
				return err
			}
			// The client built successfully, so its target is valid; ignore
			// any APIBase error and simply omit the line if it is empty.
			apiBase, _ := jc.APIBase()
			writeStatus(cmd.OutOrStdout(), g.Site, apiBase, user)
			return nil
		},
	}
}

// writeStatus prints the resolved authentication state as label/value lines.
func writeStatus(w io.Writer, site, apiBase string, user jira.User) {
	fmt.Fprintf(w, "%-10s %s\n", "status:", "authenticated")
	if site != "" {
		fmt.Fprintf(w, "%-10s %s\n", "site:", site)
	}
	account := user.DisplayName
	if user.AccountID != "" {
		account = fmt.Sprintf("%s (%s)", user.DisplayName, user.AccountID)
	}
	if account != "" {
		fmt.Fprintf(w, "%-10s %s\n", "account:", account)
	}
	if user.EmailAddress != "" {
		fmt.Fprintf(w, "%-10s %s\n", "email:", user.EmailAddress)
	}
	if apiBase != "" {
		fmt.Fprintf(w, "%-10s %s\n", "api base:", apiBase)
	}
}
