package jiracmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/aurokin/atlassian-cli/internal/apperr"
	"github.com/aurokin/atlassian-cli/internal/jira"
)

// meAlias is the shorthand a user can pass anywhere an assignee is accepted to
// mean "the authenticated account", resolved via /myself.
const meAlias = "@me"

// resolveAssignee resolves a --assignee flag value to an account id, passing
// through the empty string unchanged so callers can distinguish "no change"
// from a real assignee. A non-empty value is resolved via resolveAccountID.
func resolveAssignee(ctx context.Context, jc *jira.Client, input string) (string, error) {
	if input == "" {
		return "", nil
	}
	return resolveAccountID(ctx, jc, input)
}

// resolveAccountID turns an assignee input into a Jira account id:
//
//   - "@me" resolves to the authenticated account via /myself.
//   - a value containing "@" is treated as an email and resolved via
//     /user/search, requiring exactly one match.
//   - anything else is assumed to already be an account id and returned as-is.
//
// It never resolves the empty string; callers handle "" (no change) and the
// unassign sentinel before calling this.
func resolveAccountID(ctx context.Context, jc *jira.Client, input string) (string, error) {
	switch {
	case input == meAlias:
		raw, err := jc.Myself(ctx)
		if err != nil {
			return "", err
		}
		me, err := jira.Decode[jira.User](raw)
		if err != nil {
			return "", err
		}
		if me.AccountID == "" {
			return "", apperr.New("user_unresolved", "could not determine the authenticated account id")
		}
		return me.AccountID, nil
	case strings.Contains(input, "@"):
		raw, err := jc.SearchUsers(ctx, input)
		if err != nil {
			return "", err
		}
		users, err := jira.Decode[[]jira.User](raw)
		if err != nil {
			return "", err
		}
		switch len(users) {
		case 0:
			return "", apperr.New("user_unresolved",
				fmt.Sprintf("no Jira user matched %q", input))
		case 1:
			return users[0].AccountID, nil
		default:
			return "", apperr.New("user_unresolved",
				fmt.Sprintf("%q matched %d Jira users; pass an account id to disambiguate", input, len(users)))
		}
	default:
		return input, nil
	}
}
