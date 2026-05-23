package bitbucket

import (
	"context"
	"encoding/json"
	"net/url"

	"github.com/aurokin/atlassian-cli/internal/apperr"
	"github.com/aurokin/atlassian-cli/internal/restutil"
)

// deploymentsBase returns the deployments collection path for a repository.
// Bitbucket requires the trailing slash on this listing endpoint.
func deploymentsBase(workspace, repo string) string {
	return repoBase(workspace, repo) + "/deployments/"
}

// environmentsBase returns the environments collection path for a repository.
// Bitbucket requires the trailing slash on this listing endpoint.
func environmentsBase(workspace, repo string) string {
	return repoBase(workspace, repo) + "/environments/"
}

// ListDeployments returns one page of a repository's deployments
// (GET /repositories/{ws}/{repo}/deployments/).
func (c *Client) ListDeployments(ctx context.Context, workspace, repo string, limit int) (json.RawMessage, error) {
	q := url.Values{}
	setLimit(q, limit)
	return c.Get(ctx, restutil.WithQuery(deploymentsBase(workspace, repo), q))
}

// ListDeploymentsAll follows a repository's deployment listing to completion and
// returns an aggregated {"values": [...]} body.
func (c *Client) ListDeploymentsAll(ctx context.Context, workspace, repo string, limit int) (json.RawMessage, error) {
	q := url.Values{}
	setLimit(q, limit)
	return c.followValues(ctx, restutil.WithQuery(deploymentsBase(workspace, repo), q))
}

// GetDeployment returns a single deployment by UUID
// (GET /repositories/{ws}/{repo}/deployments/{uuid}). The UUID is normalized to
// the brace-wrapped form Bitbucket expects.
func (c *Client) GetDeployment(ctx context.Context, workspace, repo, uuid string) (json.RawMessage, error) {
	norm := NormalizePipelineUUID(uuid)
	if norm == "" {
		return nil, apperr.InvalidInput("a deployment UUID is required")
	}
	return c.Get(ctx, deploymentsBase(workspace, repo)+url.PathEscape(norm))
}

// ListEnvironments returns one page of a repository's deployment environments
// (GET /repositories/{ws}/{repo}/environments/).
func (c *Client) ListEnvironments(ctx context.Context, workspace, repo string, limit int) (json.RawMessage, error) {
	q := url.Values{}
	setLimit(q, limit)
	return c.Get(ctx, restutil.WithQuery(environmentsBase(workspace, repo), q))
}

// ListEnvironmentsAll follows a repository's environment listing to completion
// and returns an aggregated {"values": [...]} body.
func (c *Client) ListEnvironmentsAll(ctx context.Context, workspace, repo string, limit int) (json.RawMessage, error) {
	q := url.Values{}
	setLimit(q, limit)
	return c.followValues(ctx, restutil.WithQuery(environmentsBase(workspace, repo), q))
}

// GetEnvironment returns a single deployment environment by UUID
// (GET /repositories/{ws}/{repo}/environments/{uuid}). The UUID is normalized to
// the brace-wrapped form Bitbucket expects.
func (c *Client) GetEnvironment(ctx context.Context, workspace, repo, uuid string) (json.RawMessage, error) {
	norm := NormalizePipelineUUID(uuid)
	if norm == "" {
		return nil, apperr.InvalidInput("an environment UUID is required")
	}
	return c.Get(ctx, environmentsBase(workspace, repo)+url.PathEscape(norm))
}
