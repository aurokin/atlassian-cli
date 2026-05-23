package bitbucket

import (
	"context"
	"encoding/json"
	"net/url"

	"github.com/aurokin/atlassian-cli/internal/restutil"
)

// branchesBase returns the branch-refs collection path for a repository.
func branchesBase(workspace, repo string) string {
	return repoBase(workspace, repo) + "/refs/branches"
}

// tagsBase returns the tag-refs collection path for a repository.
func tagsBase(workspace, repo string) string {
	return repoBase(workspace, repo) + "/refs/tags"
}

// ListBranches returns one page of a repository's branches
// (GET /repositories/{ws}/{repo}/refs/branches).
func (c *Client) ListBranches(ctx context.Context, workspace, repo string, limit int) (json.RawMessage, error) {
	q := url.Values{}
	setLimit(q, limit)
	return c.Get(ctx, restutil.WithQuery(branchesBase(workspace, repo), q))
}

// ListBranchesAll follows a repository's branch listing to completion and
// returns an aggregated {"values": [...]} body.
func (c *Client) ListBranchesAll(ctx context.Context, workspace, repo string, limit int) (json.RawMessage, error) {
	q := url.Values{}
	setLimit(q, limit)
	return c.followValues(ctx, restutil.WithQuery(branchesBase(workspace, repo), q))
}

// GetBranch returns a single branch by name
// (GET /repositories/{ws}/{repo}/refs/branches/{name}).
func (c *Client) GetBranch(ctx context.Context, workspace, repo, name string) (json.RawMessage, error) {
	return c.Get(ctx, branchesBase(workspace, repo)+"/"+url.PathEscape(name))
}

// CreateBranchOptions holds the fields a branch creation accepts. Both are
// required by Bitbucket: Name is the new branch, Target is the commit hash (or
// existing branch name) it points at.
type CreateBranchOptions struct {
	Name   string
	Target string
}

// CreateBranch creates a branch (POST /repositories/{ws}/{repo}/refs/branches)
// and returns the created branch.
func (c *Client) CreateBranch(ctx context.Context, workspace, repo string, opts CreateBranchOptions) (json.RawMessage, error) {
	body := map[string]any{
		"name":   opts.Name,
		"target": map[string]string{"hash": opts.Target},
	}
	return c.Send(ctx, "POST", branchesBase(workspace, repo), body)
}

// DeleteBranch removes a branch
// (DELETE /repositories/{ws}/{repo}/refs/branches/{name}). Bitbucket returns no
// content on success.
func (c *Client) DeleteBranch(ctx context.Context, workspace, repo, name string) error {
	_, err := c.Send(ctx, "DELETE", branchesBase(workspace, repo)+"/"+url.PathEscape(name), nil)
	return err
}

// ListTags returns one page of a repository's tags
// (GET /repositories/{ws}/{repo}/refs/tags).
func (c *Client) ListTags(ctx context.Context, workspace, repo string, limit int) (json.RawMessage, error) {
	q := url.Values{}
	setLimit(q, limit)
	return c.Get(ctx, restutil.WithQuery(tagsBase(workspace, repo), q))
}

// ListTagsAll follows a repository's tag listing to completion and returns an
// aggregated {"values": [...]} body.
func (c *Client) ListTagsAll(ctx context.Context, workspace, repo string, limit int) (json.RawMessage, error) {
	q := url.Values{}
	setLimit(q, limit)
	return c.followValues(ctx, restutil.WithQuery(tagsBase(workspace, repo), q))
}

// GetTag returns a single tag by name
// (GET /repositories/{ws}/{repo}/refs/tags/{name}).
func (c *Client) GetTag(ctx context.Context, workspace, repo, name string) (json.RawMessage, error) {
	return c.Get(ctx, tagsBase(workspace, repo)+"/"+url.PathEscape(name))
}

// CreateTagOptions holds the fields a tag creation accepts. Name and Target are
// required by Bitbucket; Message is optional and, when set, produces an
// annotated tag.
type CreateTagOptions struct {
	Name    string
	Target  string
	Message string
}

// CreateTag creates a tag (POST /repositories/{ws}/{repo}/refs/tags) and
// returns the created tag.
func (c *Client) CreateTag(ctx context.Context, workspace, repo string, opts CreateTagOptions) (json.RawMessage, error) {
	body := map[string]any{
		"name":   opts.Name,
		"target": map[string]string{"hash": opts.Target},
	}
	if opts.Message != "" {
		body["message"] = opts.Message
	}
	return c.Send(ctx, "POST", tagsBase(workspace, repo), body)
}

// DeleteTag removes a tag
// (DELETE /repositories/{ws}/{repo}/refs/tags/{name}). Bitbucket returns no
// content on success.
func (c *Client) DeleteTag(ctx context.Context, workspace, repo, name string) error {
	_, err := c.Send(ctx, "DELETE", tagsBase(workspace, repo)+"/"+url.PathEscape(name), nil)
	return err
}
