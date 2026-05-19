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
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/url"
	"strconv"
	"strings"

	"github.com/aurokin/atlassian-cli/internal/apperr"
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

// send marshals payload as a JSON request body, issues method against an
// API-relative path or absolute URL, and returns the raw response body. A
// non-2xx response surfaces as the structured *apperr.Error from httpclient.
func (c *Client) send(ctx context.Context, method, pathOrURL string, payload any) (json.RawMessage, error) {
	var body io.Reader
	if payload != nil {
		b, err := json.Marshal(payload)
		if err != nil {
			return nil, apperr.New("request_encode_failed",
				"could not encode the Confluence API request body: "+err.Error())
		}
		body = bytes.NewReader(b)
	}
	resp, err := c.http.Do(ctx, method, pathOrURL, body)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(resp.Body), nil
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

// CreatePage creates a page in a space (POST /pages) and returns the created
// page. The body is sent verbatim under the named representation; Confluence
// never converts it.
func (c *Client) CreatePage(ctx context.Context, spaceID, title, bodyFormat, body string) (json.RawMessage, error) {
	return c.send(ctx, "POST", "/pages", map[string]any{
		"spaceId": spaceID,
		"status":  "current",
		"title":   title,
		"body": map[string]string{
			"representation": bodyFormat,
			"value":          body,
		},
	})
}

// UpdatePage replaces a page (PUT /pages/{id}) and returns the updated page.
// Confluence v2 treats an update as a full replacement, so the caller supplies
// the complete post-edit state: status, title, body, and the next version
// number (the current number plus one).
func (c *Client) UpdatePage(ctx context.Context, id, status, title, bodyFormat, body string, version int) (json.RawMessage, error) {
	return c.send(ctx, "PUT", "/pages/"+url.PathEscape(id), map[string]any{
		"id":     id,
		"status": status,
		"title":  title,
		"body": map[string]string{
			"representation": bodyFormat,
			"value":          body,
		},
		"version": map[string]int{"number": version},
	})
}

// maxFollowPages caps how many pages an --all request follows, guarding
// against an unbounded loop from a malformed cursor.
const maxFollowPages = 100

// decodeError wraps a pagination decode failure as a structured error.
func decodeError(err error) error {
	return apperr.New("response_decode_failed",
		"could not decode the Confluence API response: "+err.Error())
}

// nextCursor extracts the opaque pagination cursor from a _links.next
// reference. An empty reference — the final page — yields an empty cursor, as
// does a reference that carries no cursor query parameter.
func nextCursor(next string) (string, error) {
	if next == "" {
		return "", nil
	}
	u, err := url.Parse(next)
	if err != nil {
		return "", apperr.InvalidInput("invalid pagination link " + next + ": " + err.Error())
	}
	return u.Query().Get("cursor"), nil
}

// followList follows a Confluence cursor-paginated endpoint to completion and
// returns an aggregated {"results": [...]} body. fetch issues the request for
// a given cursor ("" for the first page); the next cursor is read from each
// response's _links.next. Following pages from the opaque cursor — rather than
// replaying the server's _links.next URL — keeps the caller in control of the
// request, so --limit governs every page and resolution is unaffected by any
// API base path prefix (such as a cloud-scoped /ex/{product}/{cloudId}
// gateway). Confluence list responses — v2 spaces/pages/children and the v1
// CQL search — all page via the cursor query parameter. Items are kept
// verbatim, so every field each page returned is preserved. Following stops at
// maxFollowPages.
func (c *Client) followList(ctx context.Context, fetch func(context.Context, string) (json.RawMessage, error)) (json.RawMessage, error) {
	all := []json.RawMessage{}
	cursor := ""
	for page := 0; page < maxFollowPages; page++ {
		raw, err := fetch(ctx, cursor)
		if err != nil {
			return nil, err
		}
		var pg struct {
			Results []json.RawMessage `json:"results"`
			Links   struct {
				Next string `json:"next"`
			} `json:"_links"`
		}
		if err := json.Unmarshal(raw, &pg); err != nil {
			return nil, decodeError(err)
		}
		all = append(all, pg.Results...)
		if cursor, err = nextCursor(pg.Links.Next); err != nil {
			return nil, err
		}
		if cursor == "" {
			break
		}
	}
	out, err := json.Marshal(map[string][]json.RawMessage{"results": all})
	if err != nil {
		return nil, decodeError(err)
	}
	return out, nil
}

// pageQuery seeds a query with the page size and, when set, the follow-up
// cursor shared by every cursor-paginated Confluence endpoint.
func pageQuery(limit int, cursor string) url.Values {
	q := url.Values{}
	setLimit(q, limit)
	if cursor != "" {
		q.Set("cursor", cursor)
	}
	return q
}

// ListSpacesAll follows GET /spaces to completion.
func (c *Client) ListSpacesAll(ctx context.Context, limit int) (json.RawMessage, error) {
	return c.followList(ctx, func(ctx context.Context, cursor string) (json.RawMessage, error) {
		return c.get(ctx, withQuery("/spaces", pageQuery(limit, cursor)))
	})
}

// ListPagesAll follows GET /pages for a space to completion.
func (c *Client) ListPagesAll(ctx context.Context, spaceID string, limit int) (json.RawMessage, error) {
	return c.followList(ctx, func(ctx context.Context, cursor string) (json.RawMessage, error) {
		q := pageQuery(limit, cursor)
		q.Set("space-id", spaceID)
		return c.get(ctx, withQuery("/pages", q))
	})
}

// GetChildPagesAll follows a page's children list to completion.
func (c *Client) GetChildPagesAll(ctx context.Context, id string, limit int) (json.RawMessage, error) {
	return c.followList(ctx, func(ctx context.Context, cursor string) (json.RawMessage, error) {
		return c.get(ctx, withQuery("/pages/"+url.PathEscape(id)+"/children", pageQuery(limit, cursor)))
	})
}

// SearchCQLAll follows the v1 CQL search to completion.
func (c *Client) SearchCQLAll(ctx context.Context, cql string, limit int) (json.RawMessage, error) {
	return c.followList(ctx, func(ctx context.Context, cursor string) (json.RawMessage, error) {
		q := pageQuery(limit, cursor)
		q.Set("cql", cql)
		u, err := c.v1URL("/search", q)
		if err != nil {
			return nil, err
		}
		return c.get(ctx, u)
	})
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
