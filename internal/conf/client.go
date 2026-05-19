// Package conf is a typed client over the Confluence Cloud REST API. It wraps
// the shared httpclient.Client and returns raw JSON response bodies: callers
// render them verbatim under --json or decode them through the models in this
// package for human output.
//
// The primary surface is Confluence REST v2 (the configured API base is
// <site>/wiki/api/v2). Two resources the MVP needs — CQL search and the
// current user — have no v2 equivalent, so they fall back to REST v1.
package conf

import (
	"context"
	"encoding/json"
	"net/url"
	"strconv"
	"strings"

	"github.com/aurokin/atlassian-cli/internal/httpclient"
)

// Client is a typed Confluence API client bound to one authenticated site.
type Client struct {
	http *httpclient.Client
}

// New wraps an authenticated httpclient.Client as a Confluence client.
func New(c *httpclient.Client) *Client {
	return &Client{http: c}
}

// APIBase returns the resolved Confluence v2 API base URL the client sends
// requests to.
func (c *Client) APIBase() (string, error) {
	return c.http.APIBase()
}

// get issues a GET against an API-relative path or an absolute URL and returns
// the raw body. A non-2xx response surfaces as the structured *apperr.Error
// from httpclient.
func (c *Client) get(ctx context.Context, pathOrURL string) (json.RawMessage, error) {
	resp, err := c.http.Do(ctx, "GET", pathOrURL, nil)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(resp.Body), nil
}

// v1URL builds an absolute URL for a Confluence REST v1 path. The v1 and v2
// bases share a prefix and differ only in the trailing version segment
// (.../rest/api vs .../api/v2), so the v1 base is derived from the configured
// v2 API base. The result is an absolute URL on the same origin, which the
// httpclient accepts.
func (c *Client) v1URL(path string, q url.Values) (string, error) {
	base, err := c.http.APIBase()
	if err != nil {
		return "", err
	}
	return withQuery(strings.TrimSuffix(base, "/api/v2")+"/rest/api"+path, q), nil
}

// CurrentUser returns the authenticated account. Confluence REST v2 has no
// current-user resource, so this uses the v1 endpoint.
func (c *Client) CurrentUser(ctx context.Context) (json.RawMessage, error) {
	u, err := c.v1URL("/user/current", nil)
	if err != nil {
		return nil, err
	}
	return c.get(ctx, u)
}

// ListSpaces returns a page of spaces (GET /spaces).
func (c *Client) ListSpaces(ctx context.Context, limit int) (json.RawMessage, error) {
	q := url.Values{}
	setLimit(q, limit)
	return c.get(ctx, withQuery("/spaces", q))
}

// GetSpace returns a single space by id (GET /spaces/{id}).
func (c *Client) GetSpace(ctx context.Context, id string) (json.RawMessage, error) {
	return c.get(ctx, "/spaces/"+url.PathEscape(id))
}

// FindSpaceByKey returns the spaces matching a key (GET /spaces?keys={key}).
// Confluence v2 addresses a space by numeric id, so a key lookup is a filtered
// list; the caller takes the single expected result.
func (c *Client) FindSpaceByKey(ctx context.Context, key string) (json.RawMessage, error) {
	q := url.Values{}
	q.Set("keys", key)
	return c.get(ctx, withQuery("/spaces", q))
}

// ListPages returns a page of pages in a space (GET /pages?space-id={id}).
func (c *Client) ListPages(ctx context.Context, spaceID string, limit int) (json.RawMessage, error) {
	q := url.Values{}
	q.Set("space-id", spaceID)
	setLimit(q, limit)
	return c.get(ctx, withQuery("/pages", q))
}

// GetPage returns a single page by id with its storage-format body
// (GET /pages/{id}?body-format=storage).
func (c *Client) GetPage(ctx context.Context, id string) (json.RawMessage, error) {
	q := url.Values{}
	q.Set("body-format", "storage")
	return c.get(ctx, withQuery("/pages/"+url.PathEscape(id), q))
}

// GetChildPages returns a page of a page's direct children
// (GET /pages/{id}/children).
func (c *Client) GetChildPages(ctx context.Context, id string, limit int) (json.RawMessage, error) {
	q := url.Values{}
	setLimit(q, limit)
	return c.get(ctx, withQuery("/pages/"+url.PathEscape(id)+"/children", q))
}

// SearchCQL runs a CQL query. CQL is a v1-only surface, so this uses the v1
// search endpoint.
func (c *Client) SearchCQL(ctx context.Context, cql string, limit int) (json.RawMessage, error) {
	q := url.Values{}
	q.Set("cql", cql)
	setLimit(q, limit)
	u, err := c.v1URL("/search", q)
	if err != nil {
		return nil, err
	}
	return c.get(ctx, u)
}

// setLimit records a positive limit as the API limit parameter.
func setLimit(q url.Values, limit int) {
	if limit > 0 {
		q.Set("limit", strconv.Itoa(limit))
	}
}

// withQuery appends an encoded query string to path when it is non-empty.
func withQuery(path string, q url.Values) string {
	if len(q) == 0 {
		return path
	}
	return path + "?" + q.Encode()
}
