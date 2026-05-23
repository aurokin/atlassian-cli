package bbcmd

import (
	"encoding/json"

	"github.com/spf13/cobra"

	"github.com/aurokin/atlassian-cli/internal/appinfo"
	"github.com/aurokin/atlassian-cli/internal/bitbucket"
	"github.com/aurokin/atlassian-cli/internal/cli"
)

// newStatusCommand builds the "status" command: a live authentication check
// against the configured site (GET /user), distinct from the offline
// "auth status". It mirrors atl-jira/atl-conf status.
func newStatusCommand(info appinfo.Info, g *cli.GlobalFlags) *cobra.Command {
	return cli.NewStatusCommand(func(*cobra.Command) (cli.StatusAdapter, error) {
		bc, err := bbClient(info, g)
		if err != nil {
			return cli.StatusAdapter{}, err
		}
		return cli.StatusAdapter{
			Fetch:   bc.CurrentUser,
			APIBase: bc.APIBase,
			Account: func(raw json.RawMessage) (cli.StatusAccount, error) {
				u, err := bitbucket.Decode[bitbucket.CurrentUser](raw)
				if err != nil {
					return cli.StatusAccount{}, err
				}
				return cli.StatusAccount{
					DisplayName: u.DisplayName,
					AccountID:   u.AccountID,
					Contact:     cli.StatusContact{Label: "username", Value: u.Username},
				}, nil
			},
		}, nil
	}, g)
}
