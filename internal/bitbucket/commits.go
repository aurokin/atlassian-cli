package bitbucket

import (
	"context"
	"encoding/json"
	"net/url"

	"github.com/aurokin/atlassian-cli/internal/restutil"
)

// repoBase returns the repository path that the commit and ref endpoints hang
// off of.
func repoBase(workspace, repo string) string {
	return "/repositories/" + url.PathEscape(workspace) + "/" + url.PathEscape(repo)
}

// commitsBase returns the commit-listing path for a repository, optionally
// scoped to a revision (branch, tag, or hash). Bitbucket lists the main
// branch's history at /commits and a specific revision's history at
// /commits/{revision}.
func commitsBase(workspace, repo, revision string) string {
	base := repoBase(workspace, repo) + "/commits"
	if revision != "" {
		base += "/" + url.PathEscape(revision)
	}
	return base
}

// ListCommits returns one page of a repository's commit history
// (GET /repositories/{ws}/{repo}/commits[/{revision}]). An empty revision lists
// the main branch.
func (c *Client) ListCommits(ctx context.Context, workspace, repo, revision string, limit int) (json.RawMessage, error) {
	q := url.Values{}
	setLimit(q, limit)
	return c.Get(ctx, restutil.WithQuery(commitsBase(workspace, repo, revision), q))
}

// ListCommitsAll follows a repository's commit history to completion and
// returns an aggregated {"values": [...]} body.
func (c *Client) ListCommitsAll(ctx context.Context, workspace, repo, revision string, limit int) (json.RawMessage, error) {
	q := url.Values{}
	setLimit(q, limit)
	return c.followValues(ctx, restutil.WithQuery(commitsBase(workspace, repo, revision), q))
}

// GetCommit returns a single commit by revision (branch, tag, or hash)
// (GET /repositories/{ws}/{repo}/commit/{revision}).
func (c *Client) GetCommit(ctx context.Context, workspace, repo, commit string) (json.RawMessage, error) {
	return c.Get(ctx, repoBase(workspace, repo)+"/commit/"+url.PathEscape(commit))
}
