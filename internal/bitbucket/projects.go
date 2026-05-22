package bitbucket

import (
	"context"
	"encoding/json"
	"net/url"

	"github.com/aurokin/atlassian-cli/internal/restutil"
)

// projectsBase returns the projects collection path for a workspace.
func projectsBase(workspace string) string {
	return "/workspaces/" + url.PathEscape(workspace) + "/projects"
}

// ListProjects returns one page of a workspace's projects
// (GET /workspaces/{ws}/projects).
func (c *Client) ListProjects(ctx context.Context, workspace string, limit int) (json.RawMessage, error) {
	q := url.Values{}
	setLimit(q, limit)
	return c.get(ctx, restutil.WithQuery(projectsBase(workspace), q))
}

// ListProjectsAll follows a workspace's project listing to completion and
// returns an aggregated {"values": [...]} body.
func (c *Client) ListProjectsAll(ctx context.Context, workspace string, limit int) (json.RawMessage, error) {
	q := url.Values{}
	setLimit(q, limit)
	return c.followValues(ctx, restutil.WithQuery(projectsBase(workspace), q))
}

// GetProject returns a single project by key
// (GET /workspaces/{ws}/projects/{key}).
func (c *Client) GetProject(ctx context.Context, workspace, projectKey string) (json.RawMessage, error) {
	return c.get(ctx, projectsBase(workspace)+"/"+url.PathEscape(projectKey))
}

// CreateProjectOptions holds the fields a project creation accepts. Name is
// required by Bitbucket; IsPrivate is a pointer so an unset flag is omitted
// (letting Bitbucket apply its default) rather than forced to false.
type CreateProjectOptions struct {
	Name        string
	Description string
	IsPrivate   *bool
}

// CreateProject creates a project in a workspace
// (POST /workspaces/{ws}/projects) and returns the created project.
func (c *Client) CreateProject(ctx context.Context, workspace, projectKey string, opts CreateProjectOptions) (json.RawMessage, error) {
	body := map[string]any{"key": projectKey, "name": opts.Name}
	if opts.Description != "" {
		body["description"] = opts.Description
	}
	if opts.IsPrivate != nil {
		body["is_private"] = *opts.IsPrivate
	}
	return c.send(ctx, "POST", projectsBase(workspace), body)
}
