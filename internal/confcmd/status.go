package confcmd

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/aurokin/atlassian-cli/internal/appinfo"
	"github.com/aurokin/atlassian-cli/internal/cli"
	"github.com/aurokin/atlassian-cli/internal/conf"
)

// newStatusCommand builds the "status" command: a live authentication check
// against the configured site, distinct from the offline "auth status".
func newStatusCommand(info appinfo.Info, g *cli.GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Check authentication against the configured site",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cc, err := confClient(info, g)
			if err != nil {
				return err
			}
			raw, err := cc.CurrentUser(cmd.Context())
			if err != nil {
				return err
			}
			if g.WantsStructured() {
				return cli.Render(cmd, g, raw)
			}
			user, err := conf.Decode[conf.User](raw)
			if err != nil {
				return err
			}
			// The client built successfully, so its target is valid; ignore
			// any APIBase error and simply omit the line if it is empty.
			apiBase, _ := cc.APIBase()
			writeStatus(cmd.OutOrStdout(), g.Site, apiBase, user)
			return nil
		},
	}
}

// writeStatus prints the resolved authentication state as label/value lines.
func writeStatus(w io.Writer, site, apiBase string, user conf.User) {
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
	if user.Email != "" {
		fmt.Fprintf(w, "%-10s %s\n", "email:", user.Email)
	}
	if apiBase != "" {
		fmt.Fprintf(w, "%-10s %s\n", "api base:", apiBase)
	}
}
