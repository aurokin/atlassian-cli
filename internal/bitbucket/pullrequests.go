package bitbucket

import (
	"context"
	"encoding/json"
	"net/url"
	"strconv"

	"github.com/aurokin/atlassian-cli/internal/restutil"
)

// prBase returns the pull-requests collection path for a repository.
func prBase(workspace, repo string) string {
	return "/repositories/" + url.PathEscape(workspace) + "/" + url.PathEscape(repo) + "/pullrequests"
}

// pullRequestsQuery assembles the list query: a page size and an optional
// state filter. A state of "" or "ALL" (case-insensitive is handled by the
// caller) lists every state.
func pullRequestsQuery(state string, limit int) url.Values {
	q := url.Values{}
	setLimit(q, limit)
	if state != "" {
		q.Set("state", state)
	}
	return q
}

// ListPullRequests returns one page of a repository's pull requests
// (GET /repositories/{ws}/{repo}/pullrequests). state filters by Bitbucket PR
// state (OPEN, MERGED, DECLINED, SUPERSEDED); an empty state lists all.
func (c *Client) ListPullRequests(ctx context.Context, workspace, repo, state string, limit int) (json.RawMessage, error) {
	return c.get(ctx, restutil.WithQuery(prBase(workspace, repo), pullRequestsQuery(state, limit)))
}

// ListPullRequestsAll follows a repository's pull-request listing to
// completion and returns an aggregated {"values": [...]} body.
func (c *Client) ListPullRequestsAll(ctx context.Context, workspace, repo, state string, limit int) (json.RawMessage, error) {
	return c.followValues(ctx, restutil.WithQuery(prBase(workspace, repo), pullRequestsQuery(state, limit)))
}

// GetPullRequest returns a single pull request by numeric id
// (GET /repositories/{ws}/{repo}/pullrequests/{id}).
func (c *Client) GetPullRequest(ctx context.Context, workspace, repo string, id int) (json.RawMessage, error) {
	return c.get(ctx, prBase(workspace, repo)+"/"+strconv.Itoa(id))
}

// CreatePullRequestOptions holds the fields a pull-request creation accepts.
// Title, SourceBranch, and DestinationBranch are required by Bitbucket.
type CreatePullRequestOptions struct {
	Title             string
	SourceBranch      string
	DestinationBranch string
	Description       string
	CloseSourceBranch bool
	Draft             bool
}

// CreatePullRequest opens a pull request
// (POST /repositories/{ws}/{repo}/pullrequests) and returns the created PR.
func (c *Client) CreatePullRequest(ctx context.Context, workspace, repo string, opts CreatePullRequestOptions) (json.RawMessage, error) {
	body := map[string]any{
		"title":       opts.Title,
		"source":      map[string]any{"branch": map[string]string{"name": opts.SourceBranch}},
		"destination": map[string]any{"branch": map[string]string{"name": opts.DestinationBranch}},
	}
	if opts.Description != "" {
		body["description"] = opts.Description
	}
	if opts.CloseSourceBranch {
		body["close_source_branch"] = true
	}
	if opts.Draft {
		body["draft"] = true
	}
	return c.send(ctx, "POST", prBase(workspace, repo), body)
}
