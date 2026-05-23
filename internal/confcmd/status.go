package confcmd

import (
	"encoding/json"

	"github.com/spf13/cobra"

	"github.com/aurokin/atlassian-cli/internal/appinfo"
	"github.com/aurokin/atlassian-cli/internal/cli"
	"github.com/aurokin/atlassian-cli/internal/conf"
)

// newStatusCommand builds the "status" command: a live authentication check
// against the configured site, distinct from the offline "auth status".
func newStatusCommand(info appinfo.Info, g *cli.GlobalFlags) *cobra.Command {
	return cli.NewStatusCommand(func(*cobra.Command) (cli.StatusAdapter, error) {
		cc, err := confClient(info, g)
		if err != nil {
			return cli.StatusAdapter{}, err
		}
		return cli.StatusAdapter{
			Fetch:   cc.CurrentUser,
			APIBase: cc.APIBase,
			Account: func(raw json.RawMessage) (cli.StatusAccount, error) {
				u, err := conf.Decode[conf.User](raw)
				if err != nil {
					return cli.StatusAccount{}, err
				}
				return cli.StatusAccount{
					DisplayName: u.DisplayName,
					AccountID:   u.AccountID,
					Contact:     cli.StatusContact{Label: "email", Value: u.Email},
				}, nil
			},
		}, nil
	}, g)
}
