package cli

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/aurokin/atlassian-cli/internal/output"
)

// StatusContact is an optional trailing label/value line in the human status
// output — Jira and Confluence put the account email here, Bitbucket the
// username.
type StatusContact struct {
	Label string
	Value string
}

// StatusAccount is the product-neutral account summary the shared status
// renderer prints. The "(accountID)" formatting and the field labels are
// applied centrally so all three products read identically.
type StatusAccount struct {
	DisplayName string
	AccountID   string
	Contact     StatusContact
}

// StatusAdapter supplies the product-specific pieces of the live status check:
// the current-user fetch, the resolved API base, and a decode of the raw body
// into the neutral StatusAccount. Everything else (the structured-output gate,
// the human layout) is shared.
type StatusAdapter struct {
	Fetch   func(ctx context.Context) (json.RawMessage, error)
	APIBase func() (string, error)
	Account func(raw json.RawMessage) (StatusAccount, error)
}

// RunStatus implements the shared "status" command body: a live authentication
// check against the configured site, distinct from the offline "auth status".
// Under --json/--jq it renders the raw current-user body; otherwise it prints
// the resolved authentication state as aligned label/value lines.
func RunStatus(cmd *cobra.Command, g *GlobalFlags, a StatusAdapter) error {
	raw, err := a.Fetch(cmd.Context())
	if err != nil {
		return err
	}
	if g.WantsStructured() {
		return Render(cmd, g, raw)
	}
	acct, err := a.Account(raw)
	if err != nil {
		return err
	}
	// The client built successfully, so its target is valid; ignore any
	// APIBase error and simply omit the line if it is empty.
	apiBase, _ := a.APIBase()
	w := cmd.OutOrStdout()
	lw := output.NewLabelWriter(w)
	lw.Add("status", "authenticated")
	lw.AddIf("site", g.Site)
	account := acct.DisplayName
	if acct.AccountID != "" {
		account = fmt.Sprintf("%s (%s)", acct.DisplayName, acct.AccountID)
	}
	lw.AddIf("account", account)
	if acct.Contact.Value != "" {
		lw.Add(acct.Contact.Label, acct.Contact.Value)
	}
	lw.AddIf("api base", apiBase)
	_ = lw.Flush()
	return nil
}

// NewStatusCommand builds the shared "status" cobra command around an adapter
// built lazily per invocation (so client construction errors surface in RunE,
// not at registration). build returns the adapter or an error.
func NewStatusCommand(build func(cmd *cobra.Command) (StatusAdapter, error), g *GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Check authentication against the configured site",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			a, err := build(cmd)
			if err != nil {
				return err
			}
			return RunStatus(cmd, g, a)
		},
	}
}
