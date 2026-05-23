package bitbucket

import (
	"context"
	"encoding/json"
	"net/url"

	"github.com/aurokin/atlassian-cli/internal/restutil"
)

// searchQuery assembles a list query that filters by a raw Bitbucket `q`
// expression and, when provided, sorts by a field. The query is passed through
// verbatim so the caller controls the full Bitbucket filter grammar, matching
// the raw-API philosophy of atl-jira's JQL search.
func searchQuery(query, sort string, limit int) url.Values {
	q := url.Values{}
	setLimit(q, limit)
	if query != "" {
		q.Set("q", query)
	}
	if sort != "" {
		q.Set("sort", sort)
	}
	return q
}

// SearchRepositories lists a workspace's repositories filtered by a raw
// Bitbucket query (GET /repositories/{workspace}?q=…).
func (c *Client) SearchRepositories(ctx context.Context, workspace, query, sort string, limit int) (json.RawMessage, error) {
	return c.Get(ctx, restutil.WithQuery("/repositories/"+url.PathEscape(workspace), searchQuery(query, sort, limit)))
}

// SearchRepositoriesAll follows a repository search to completion and returns
// an aggregated {"values": [...]} body.
func (c *Client) SearchRepositoriesAll(ctx context.Context, workspace, query, sort string, limit int) (json.RawMessage, error) {
	return c.followValues(ctx, restutil.WithQuery("/repositories/"+url.PathEscape(workspace), searchQuery(query, sort, limit)))
}

// SearchPullRequests lists a repository's pull requests filtered by a raw
// Bitbucket query (GET /repositories/{ws}/{repo}/pullrequests?q=…).
func (c *Client) SearchPullRequests(ctx context.Context, workspace, repo, query, sort string, limit int) (json.RawMessage, error) {
	return c.Get(ctx, restutil.WithQuery(prBase(workspace, repo), searchQuery(query, sort, limit)))
}

// SearchPullRequestsAll follows a pull-request search to completion and returns
// an aggregated {"values": [...]} body.
func (c *Client) SearchPullRequestsAll(ctx context.Context, workspace, repo, query, sort string, limit int) (json.RawMessage, error) {
	return c.followValues(ctx, restutil.WithQuery(prBase(workspace, repo), searchQuery(query, sort, limit)))
}

// SearchIssues lists a repository's issues filtered by a raw Bitbucket query
// (GET /repositories/{ws}/{repo}/issues?q=…). A repository with its issue
// tracker disabled surfaces as a feature_disabled error.
func (c *Client) SearchIssues(ctx context.Context, workspace, repo, query, sort string, limit int) (json.RawMessage, error) {
	return c.Get(ctx, restutil.WithQuery(issuesBase(workspace, repo), searchQuery(query, sort, limit)))
}

// SearchIssuesAll follows an issue search to completion and returns an
// aggregated {"values": [...]} body.
func (c *Client) SearchIssuesAll(ctx context.Context, workspace, repo, query, sort string, limit int) (json.RawMessage, error) {
	return c.followValues(ctx, restutil.WithQuery(issuesBase(workspace, repo), searchQuery(query, sort, limit)))
}
