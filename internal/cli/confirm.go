package cli

import (
	"github.com/spf13/cobra"

	"github.com/aurokin/atlassian-cli/internal/apperr"
)

// AddYesFlag registers the --yes confirmation flag that every destructive verb
// requires (ADR 0003), with the shared help text.
func AddYesFlag(cmd *cobra.Command, yes *bool) {
	cmd.Flags().BoolVar(yes, "yes", false, "confirm the irreversible deletion")
}

// RequireYes is the shared --yes gate for destructive verbs: nil when the flag
// was passed, otherwise the standard invalid_input refusal. action names the
// operation ("deleting a branch"). Call it before the product client is built
// so a refused command makes no request.
func RequireYes(yes bool, action string) error {
	if yes {
		return nil
	}
	return apperr.InvalidInput(action + " is irreversible; pass --yes to confirm")
}
