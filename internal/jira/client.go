// Package jira is a typed client over the Jira Cloud REST v3 API. It wraps the
// shared httpclient.Client and returns raw JSON response bodies: callers render
// them verbatim under --json or decode them through the models in this package
// for human output.
package jira

import (
	"context"
	"encoding/json"
	"net/url"
	"strconv"

	"github.com/aurokin/atlassian-cli/internal/httpclient"
)

// Client is a typed Jira API client bound to one authenticated site.
type Client struct {
	http *httpclient.Client
}

// New wraps an authenticated httpclient.Client as a Jira client.
func New(c *httpclient.Client) *Client {
	return &Client{http: c}
}

// APIBase returns the resolved Jira API base URL the client sends requests to.
func (c *Client) APIBase() (string, error) {
	return c.http.APIBase()
}

// get issues a GET against an API-relative path and returns the raw body. A
// non-2xx response surfaces as the structured *apperr.Error from httpclient.
func (c *Client) get(ctx context.Context, path string) (json.RawMessage, error) {
	resp, err := c.http.Do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(resp.Body), nil
}

// Myself returns the authenticated account (GET /myself).
func (c *Client) Myself(ctx context.Context) (json.RawMessage, error) {
	return c.get(ctx, "/myself")
}

// GetProject returns a single project by id or key (GET /project/{idOrKey}).
func (c *Client) GetProject(ctx context.Context, idOrKey string) (json.RawMessage, error) {
	return c.get(ctx, "/project/"+url.PathEscape(idOrKey))
}

// SearchProjects returns a page of projects (GET /project/search).
func (c *Client) SearchProjects(ctx context.Context, limit int) (json.RawMessage, error) {
	q := url.Values{}
	setLimit(q, limit)
	return c.get(ctx, withQuery("/project/search", q))
}

// GetIssue returns a single issue by id or key (GET /issue/{idOrKey}).
func (c *Client) GetIssue(ctx context.Context, idOrKey string) (json.RawMessage, error) {
	return c.get(ctx, "/issue/"+url.PathEscape(idOrKey))
}

// SearchIssues runs a JQL query (GET /search/jql).
//
// The enhanced /search/jql endpoint returns only id and key unless fields are
// requested explicitly, so the navigable field set is always asked for — that
// is the standard set the issue models render.
func (c *Client) SearchIssues(ctx context.Context, jql string, limit int) (json.RawMessage, error) {
	q := url.Values{}
	q.Set("jql", jql)
	q.Set("fields", "*navigable")
	setLimit(q, limit)
	return c.get(ctx, withQuery("/search/jql", q))
}

// ListComments returns a page of comments on an issue
// (GET /issue/{idOrKey}/comment).
func (c *Client) ListComments(ctx context.Context, issue string, limit int) (json.RawMessage, error) {
	q := url.Values{}
	setLimit(q, limit)
	return c.get(ctx, withQuery("/issue/"+url.PathEscape(issue)+"/comment", q))
}

// GetComment returns a single comment (GET /issue/{idOrKey}/comment/{id}).
func (c *Client) GetComment(ctx context.Context, issue, commentID string) (json.RawMessage, error) {
	return c.get(ctx, "/issue/"+url.PathEscape(issue)+"/comment/"+url.PathEscape(commentID))
}

// setLimit records a positive limit as the API maxResults parameter.
func setLimit(q url.Values, limit int) {
	if limit > 0 {
		q.Set("maxResults", strconv.Itoa(limit))
	}
}

// withQuery appends an encoded query string to path when it is non-empty.
func withQuery(path string, q url.Values) string {
	if len(q) == 0 {
		return path
	}
	return path + "?" + q.Encode()
}
