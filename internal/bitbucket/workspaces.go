package bitbucket

import (
	"context"
	"encoding/json"
	"net/url"

	"github.com/aurokin/atlassian-cli/internal/restutil"
)

// workspacesBase is the collection of workspaces the authenticated account is
// a member of.
func workspacesQuery(limit int) url.Values {
	q := url.Values{}
	// role=member scopes the listing to workspaces the account belongs to,
	// matching the legacy bb behavior.
	q.Set("role", "member")
	setLimit(q, limit)
	return q
}

// ListWorkspaces returns one page of the workspaces the authenticated account
// is a member of (GET /workspaces?role=member).
func (c *Client) ListWorkspaces(ctx context.Context, limit int) (json.RawMessage, error) {
	return c.get(ctx, restutil.WithQuery("/workspaces", workspacesQuery(limit)))
}

// ListWorkspacesAll follows the workspace listing to completion and returns an
// aggregated {"values": [...]} body.
func (c *Client) ListWorkspacesAll(ctx context.Context, limit int) (json.RawMessage, error) {
	return c.followValues(ctx, restutil.WithQuery("/workspaces", workspacesQuery(limit)))
}

// GetWorkspace returns a single workspace by slug (GET /workspaces/{slug}).
func (c *Client) GetWorkspace(ctx context.Context, workspace string) (json.RawMessage, error) {
	return c.get(ctx, "/workspaces/"+url.PathEscape(workspace))
}
