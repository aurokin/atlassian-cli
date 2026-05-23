package bitbucket

import (
	"context"
	"encoding/json"
	"net/url"
)

// GetWorkspace returns a single workspace by slug (GET /workspaces/{slug}).
//
// Note: there is no workspace-listing method. Bitbucket removed the
// cross-workspace enumeration endpoint (GET /2.0/workspaces) on 2026-04-14
// (changelog CHANGE-3022); workspaces are addressed by slug.
func (c *Client) GetWorkspace(ctx context.Context, workspace string) (json.RawMessage, error) {
	return c.Get(ctx, "/workspaces/"+url.PathEscape(workspace))
}
