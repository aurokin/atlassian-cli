// Package bitbucket is a typed client over the Bitbucket Cloud REST 2.0 API.
// It wraps the shared httpclient.Client and returns raw JSON response bodies:
// callers render them verbatim under --json or decode them through the models
// in this package for human output. The shape mirrors the internal/jira and
// internal/conf clients so all three products sit on one foundation.
package bitbucket

import (
	"context"
	"encoding/json"
	"net/url"

	"github.com/aurokin/atlassian-cli/internal/httpclient"
	"github.com/aurokin/atlassian-cli/internal/restutil"
)

// productName labels this product in shared structured error messages.
const productName = "Bitbucket"

// MaxPageLen is the maximum page size (the "pagelen" parameter) for most
// Bitbucket Cloud collection endpoints; the pull-requests listing caps lower
// (see MaxPullRequestPageLen). An --all request with no explicit --limit
// defaults to the endpoint's maximum, so the page follow makes the fewest
// round-trips and is least likely to hit the page-follow cap.
const MaxPageLen = 100

// Client is a typed Bitbucket API client bound to one authenticated site. It
// embeds restutil.Base for the shared request plumbing. There is no
// issue-tracker surface: Bitbucket removed it on 2026-08-20 (ADR 0008).
type Client struct {
	restutil.Base
}

// New wraps an authenticated httpclient.Client as a Bitbucket client.
func New(c *httpclient.Client) *Client {
	return &Client{Base: restutil.Base{HTTP: c, Product: productName}}
}

// CurrentUser returns the authenticated account (GET /user).
func (c *Client) CurrentUser(ctx context.Context) (json.RawMessage, error) {
	return c.Get(ctx, "/user")
}

// repositoryBase returns the path of a single repository.
func repositoryBase(workspace, repo string) string {
	return "/repositories/" + url.PathEscape(workspace) + "/" + url.PathEscape(repo)
}

// GetRepository returns a single repository (GET /repositories/{ws}/{repo}).
func (c *Client) GetRepository(ctx context.Context, workspace, repo string) (json.RawMessage, error) {
	return c.Get(ctx, repositoryBase(workspace, repo))
}

// CreateRepositoryOptions holds the fields a repository creation accepts.
// Bitbucket only supports git, so the SCM is fixed. IsPrivate is a pointer so
// an unset flag is omitted (letting Bitbucket apply its default) rather than
// forced to false; ProjectKey places the repository in a project.
type CreateRepositoryOptions struct {
	Description string
	IsPrivate   *bool
	ProjectKey  string
}

// CreateRepository creates a repository (POST /repositories/{ws}/{repo}) and
// returns the created repository. The repo slug is taken from the URL.
func (c *Client) CreateRepository(ctx context.Context, workspace, repo string, opts CreateRepositoryOptions) (json.RawMessage, error) {
	body := map[string]any{"scm": "git"}
	if opts.Description != "" {
		body["description"] = opts.Description
	}
	if opts.IsPrivate != nil {
		body["is_private"] = *opts.IsPrivate
	}
	if opts.ProjectKey != "" {
		body["project"] = map[string]string{"key": opts.ProjectKey}
	}
	return c.Send(ctx, "POST", repositoryBase(workspace, repo), body)
}

// DeleteRepository removes a repository (DELETE /repositories/{ws}/{repo}).
// This is irreversible; Bitbucket returns no content on success.
func (c *Client) DeleteRepository(ctx context.Context, workspace, repo string) error {
	_, err := c.Send(ctx, "DELETE", repositoryBase(workspace, repo), nil)
	return err
}

// ListRepositories returns one page of a workspace's repositories
// (GET /repositories/{workspace}).
func (c *Client) ListRepositories(ctx context.Context, workspace string, limit int) (json.RawMessage, error) {
	q := url.Values{}
	setLimit(q, limit)
	return c.Get(ctx, restutil.WithQuery("/repositories/"+url.PathEscape(workspace), q))
}

// ListRepositoriesAll follows a workspace's repository listing to completion
// and returns an aggregated {"values": [...]} body.
func (c *Client) ListRepositoriesAll(ctx context.Context, workspace string, limit int) (json.RawMessage, error) {
	q := url.Values{}
	setLimit(q, limit)
	return c.followValues(ctx, restutil.WithQuery("/repositories/"+url.PathEscape(workspace), q))
}

// followValues follows Bitbucket's "next"-URL pagination from firstPath to
// completion, collecting every page's "values" into one aggregated body. It
// stops at restutil.MaxFollowPages to guard against an unbounded loop from a
// malformed cursor. Bitbucket returns an absolute "next" URL whose host is the
// configured API origin, so the httpclient same-origin guard accepts it.
func (c *Client) followValues(ctx context.Context, firstPath string) (json.RawMessage, error) {
	items, err := restutil.FollowAll(ctx, firstPath,
		func(ctx context.Context, cursor string) (json.RawMessage, error) {
			return c.Get(ctx, cursor)
		},
		func(raw json.RawMessage, _ string) ([]json.RawMessage, string, error) {
			var pg struct {
				Values []json.RawMessage `json:"values"`
				Next   string            `json:"next"`
			}
			if err := json.Unmarshal(raw, &pg); err != nil {
				return nil, "", decodeError(err)
			}
			// Bitbucket's "next" is an absolute same-origin URL accepted by the
			// httpclient origin guard; follow it directly.
			return pg.Values, pg.Next, nil
		},
	)
	if err != nil {
		return nil, err
	}
	return restutil.Aggregate(productName, "values", items)
}

// decodeError wraps a pagination-aggregation or decode failure as a structured
// error.
func decodeError(err error) error {
	return restutil.DecodeError(productName, err)
}

// setLimit records a positive limit as Bitbucket's pagelen page-size parameter.
func setLimit(q url.Values, limit int) {
	restutil.SetLimit(q, "pagelen", limit)
}
