package jiracmd

import (
	"encoding/json"

	"github.com/spf13/cobra"

	"github.com/aurokin/atlassian-cli/internal/appinfo"
	"github.com/aurokin/atlassian-cli/internal/cli"
	"github.com/aurokin/atlassian-cli/internal/jira"
)

// newStatusCommand builds the "status" command: a live authentication check
// against the configured site, distinct from the offline "auth status".
func newStatusCommand(info appinfo.Info, g *cli.GlobalFlags) *cobra.Command {
	return cli.NewStatusCommand(func(*cobra.Command) (cli.StatusAdapter, error) {
		jc, err := jiraClient(info, g)
		if err != nil {
			return cli.StatusAdapter{}, err
		}
		return cli.StatusAdapter{
			Fetch:   jc.Myself,
			APIBase: jc.APIBase,
			Account: func(raw json.RawMessage) (cli.StatusAccount, error) {
				u, err := jira.Decode[jira.User](raw)
				if err != nil {
					return cli.StatusAccount{}, err
				}
				return cli.StatusAccount{
					DisplayName: u.DisplayName,
					AccountID:   u.AccountID,
					Contact:     cli.StatusContact{Label: "email", Value: u.EmailAddress},
				}, nil
			},
		}, nil
	}, g)
}
