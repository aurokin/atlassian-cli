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
	"fmt"
	"io"
	"net/url"
	"strconv"
	"strings"

	"github.com/aurokin/atlassian-cli/internal/apperr"
	"github.com/aurokin/atlassian-cli/internal/httpclient"
	"github.com/aurokin/atlassian-cli/internal/restutil"
)

// productName labels this product in shared structured error messages.
const productName = "Confluence"

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
	return restutil.WithQuery(strings.TrimSuffix(base, "/api/v2")+"/rest/api"+path, q), nil
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
	return c.get(ctx, restutil.WithQuery("/spaces", q))
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
	return c.get(ctx, restutil.WithQuery("/spaces", q))
}

// ListPages returns a page of pages in a space (GET /pages?space-id={id}).
func (c *Client) ListPages(ctx context.Context, spaceID string, limit int) (json.RawMessage, error) {
	q := url.Values{}
	q.Set("space-id", spaceID)
	setLimit(q, limit)
	return c.get(ctx, restutil.WithQuery("/pages", q))
}

// GetPage returns a single page by id with its storage-format body
// (GET /pages/{id}?body-format=storage).
func (c *Client) GetPage(ctx context.Context, id string) (json.RawMessage, error) {
	return c.GetPageWithFormat(ctx, id, "storage")
}

// GetPageWithFormat returns a single page by id in the requested body
// representation. The v2 body-format query param is a single value (not a
// comma list), so each representation needs its own GET; callers that need
// both storage and atlas_doc_format make two requests.
func (c *Client) GetPageWithFormat(ctx context.Context, id, bodyFormat string) (json.RawMessage, error) {
	q := url.Values{}
	q.Set("body-format", bodyFormat)
	return c.get(ctx, restutil.WithQuery("/pages/"+url.PathEscape(id), q))
}

// GetChildPages returns a page of a page's direct children
// (GET /pages/{id}/children).
func (c *Client) GetChildPages(ctx context.Context, id string, limit int) (json.RawMessage, error) {
	q := url.Values{}
	setLimit(q, limit)
	return c.get(ctx, restutil.WithQuery("/pages/"+url.PathEscape(id)+"/children", q))
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

// decodeError wraps a pagination decode failure as a structured error.
func decodeError(err error) error {
	return restutil.DecodeError(productName, err)
}

// nextPageURL builds the next-page request from the current request URL and
// the _links.next reference Confluence returned with the current page. Only
// the query — carrying the cursor (or the v1 search start offset) and the
// echoed page size — is taken from _links.next; the scheme, host, and path are
// kept from the current request. Confluence omits the API base path prefix
// from _links.next (a cloud-scoped /ex/{product}/{cloudId} gateway, or the v1
// /wiki/rest/api search base), so reusing the current request's path keeps
// resolution correct and works whether the endpoint pages by cursor or offset.
func nextPageURL(current, next string) (string, error) {
	cu, err := url.Parse(current)
	if err != nil {
		return "", apperr.InvalidInput("invalid request URL " + current + ": " + err.Error())
	}
	nu, err := url.Parse(next)
	if err != nil {
		return "", apperr.InvalidInput("invalid pagination link " + next + ": " + err.Error())
	}
	cu.RawQuery = nu.RawQuery
	return cu.String(), nil
}

// followList follows a Confluence list endpoint to completion, starting from
// firstURL (an API-relative path or an absolute same-origin URL), and returns
// an aggregated {"results": [...]} body. Confluence list responses page via a
// _links.next reference; nextPageURL threads each page onto the original
// request path. Items are kept verbatim, so every field each page returned is
// preserved. Following stops at restutil.MaxFollowPages.
func (c *Client) followList(ctx context.Context, firstURL string) (json.RawMessage, error) {
	all := []json.RawMessage{}
	reqURL := firstURL
	done := false
	for page := 0; page < restutil.MaxFollowPages; page++ {
		raw, err := c.get(ctx, reqURL)
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
		if pg.Links.Next == "" {
			done = true
			break
		}
		if reqURL, err = nextPageURL(reqURL, pg.Links.Next); err != nil {
			return nil, err
		}
	}
	// Exiting the loop without seeing the last page means the cap was hit and
	// the aggregate is incomplete.
	if !done {
		return nil, restutil.TruncatedError()
	}
	out, err := json.Marshal(map[string][]json.RawMessage{"results": all})
	if err != nil {
		return nil, decodeError(err)
	}
	return out, nil
}

// ListSpacesAll follows GET /spaces to completion.
func (c *Client) ListSpacesAll(ctx context.Context, limit int) (json.RawMessage, error) {
	q := url.Values{}
	setLimit(q, limit)
	return c.followList(ctx, restutil.WithQuery("/spaces", q))
}

// ListPagesAll follows GET /pages for a space to completion.
func (c *Client) ListPagesAll(ctx context.Context, spaceID string, limit int) (json.RawMessage, error) {
	q := url.Values{}
	q.Set("space-id", spaceID)
	setLimit(q, limit)
	return c.followList(ctx, restutil.WithQuery("/pages", q))
}

// GetChildPagesAll follows a page's children list to completion.
func (c *Client) GetChildPagesAll(ctx context.Context, id string, limit int) (json.RawMessage, error) {
	q := url.Values{}
	setLimit(q, limit)
	return c.followList(ctx, restutil.WithQuery("/pages/"+url.PathEscape(id)+"/children", q))
}

// SearchCQLAll follows the v1 CQL search to completion.
func (c *Client) SearchCQLAll(ctx context.Context, cql string, limit int) (json.RawMessage, error) {
	q := url.Values{}
	q.Set("cql", cql)
	setLimit(q, limit)
	u, err := c.v1URL("/search", q)
	if err != nil {
		return nil, err
	}
	return c.followList(ctx, u)
}

// ListFooterComments returns a page of a page's footer comments
// (GET /pages/{id}/footer-comments).
func (c *Client) ListFooterComments(ctx context.Context, pageID string, limit int) (json.RawMessage, error) {
	q := url.Values{}
	setLimit(q, limit)
	return c.get(ctx, restutil.WithQuery("/pages/"+url.PathEscape(pageID)+"/footer-comments", q))
}

// ListFooterCommentsAll follows a page's footer-comment list to completion.
func (c *Client) ListFooterCommentsAll(ctx context.Context, pageID string, limit int) (json.RawMessage, error) {
	q := url.Values{}
	setLimit(q, limit)
	return c.followList(ctx, restutil.WithQuery("/pages/"+url.PathEscape(pageID)+"/footer-comments", q))
}

// GetFooterComment returns a single footer comment by id with its
// storage-format body (GET /footer-comments/{id}?body-format=storage).
func (c *Client) GetFooterComment(ctx context.Context, id string) (json.RawMessage, error) {
	q := url.Values{}
	q.Set("body-format", "storage")
	return c.get(ctx, restutil.WithQuery("/footer-comments/"+url.PathEscape(id), q))
}

// CreateFooterComment adds a footer comment to a page (POST /footer-comments)
// and returns the created comment. The body is sent verbatim under the named
// representation.
func (c *Client) CreateFooterComment(ctx context.Context, pageID, bodyFormat, body string) (json.RawMessage, error) {
	return c.send(ctx, "POST", "/footer-comments", map[string]any{
		"pageId": pageID,
		"body": map[string]string{
			"representation": bodyFormat,
			"value":          body,
		},
	})
}

// UpdateFooterComment replaces a footer comment's body
// (PUT /footer-comments/{id}) and returns the updated comment. Confluence v2
// treats the update as a full replacement, so the caller supplies the next
// version number (the current number plus one).
func (c *Client) UpdateFooterComment(ctx context.Context, id, bodyFormat, body string, version int) (json.RawMessage, error) {
	return c.send(ctx, "PUT", "/footer-comments/"+url.PathEscape(id), map[string]any{
		"version": map[string]int{"number": version},
		"body": map[string]string{
			"representation": bodyFormat,
			"value":          body,
		},
	})
}

// DeleteFooterComment removes a footer comment (DELETE /footer-comments/{id}).
func (c *Client) DeleteFooterComment(ctx context.Context, id string) error {
	_, err := c.send(ctx, "DELETE", "/footer-comments/"+url.PathEscape(id), nil)
	return err
}

// ListLabels returns a page of a page's labels (GET /pages/{id}/labels).
func (c *Client) ListLabels(ctx context.Context, pageID string, limit int) (json.RawMessage, error) {
	q := url.Values{}
	setLimit(q, limit)
	return c.get(ctx, restutil.WithQuery("/pages/"+url.PathEscape(pageID)+"/labels", q))
}

// ListLabelsAll follows a page's label list to completion.
func (c *Client) ListLabelsAll(ctx context.Context, pageID string, limit int) (json.RawMessage, error) {
	q := url.Values{}
	setLimit(q, limit)
	return c.followList(ctx, restutil.WithQuery("/pages/"+url.PathEscape(pageID)+"/labels", q))
}

// AddLabel attaches a label to a page. Confluence v2 has no page-label write
// endpoint, so this uses the v1 content-label surface.
func (c *Client) AddLabel(ctx context.Context, pageID, label string) (json.RawMessage, error) {
	u, err := c.v1URL("/content/"+url.PathEscape(pageID)+"/label", nil)
	if err != nil {
		return nil, err
	}
	return c.send(ctx, "POST", u, []map[string]string{{"name": label}})
}

// RemoveLabel detaches a label from a page via the v1 content-label surface.
func (c *Client) RemoveLabel(ctx context.Context, pageID, label string) error {
	u, err := c.v1URL("/content/"+url.PathEscape(pageID)+"/label/"+url.PathEscape(label), nil)
	if err != nil {
		return err
	}
	_, err = c.send(ctx, "DELETE", u, nil)
	return err
}

// ListAttachments returns a page of a page's attachments
// (GET /pages/{id}/attachments).
func (c *Client) ListAttachments(ctx context.Context, pageID string, limit int) (json.RawMessage, error) {
	q := url.Values{}
	setLimit(q, limit)
	return c.get(ctx, restutil.WithQuery("/pages/"+url.PathEscape(pageID)+"/attachments", q))
}

// ListAttachmentsAll follows a page's attachment list to completion.
func (c *Client) ListAttachmentsAll(ctx context.Context, pageID string, limit int) (json.RawMessage, error) {
	q := url.Values{}
	setLimit(q, limit)
	return c.followList(ctx, restutil.WithQuery("/pages/"+url.PathEscape(pageID)+"/attachments", q))
}

// GetAttachment returns a single attachment's metadata, including the
// downloadLink that locates its binary (GET /attachments/{id}).
func (c *Client) GetAttachment(ctx context.Context, id string) (json.RawMessage, error) {
	return c.get(ctx, "/attachments/"+url.PathEscape(id))
}

// FetchAttachmentData downloads an attachment's binary content. downloadLink
// comes from an Attachment record; it is resolved against the Confluence
// context path by downloadURL.
func (c *Client) FetchAttachmentData(ctx context.Context, downloadLink string) ([]byte, error) {
	u, err := c.downloadURL(downloadLink)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(ctx, "GET", u, nil)
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
}

// downloadURL resolves an attachment downloadLink to an absolute URL. A v2
// downloadLink is rooted at the Confluence context path (.../wiki), not the
// API base, so the context base is the API base minus the trailing /api/v2
// segment. An already-absolute link is returned unchanged. Either form is
// still subject to the httpclient's same-origin check at request time, so an
// absolute link pointing off the configured site is rejected when fetched.
func (c *Client) downloadURL(link string) (string, error) {
	if link == "" {
		return "", apperr.InvalidInput("attachment has no downloadLink")
	}
	u, err := url.Parse(link)
	if err != nil {
		return "", apperr.InvalidInput(fmt.Sprintf("invalid downloadLink %q: %v", link, err))
	}
	if u.IsAbs() {
		return link, nil
	}
	base, err := c.http.APIBase()
	if err != nil {
		return "", err
	}
	ctxBase := strings.TrimSuffix(base, "/api/v2")
	return strings.TrimRight(ctxBase, "/") + "/" + strings.TrimLeft(link, "/"), nil
}

// setLimit records a positive limit as the API limit parameter.
func setLimit(q url.Values, limit int) {
	if limit > 0 {
		q.Set("limit", strconv.Itoa(limit))
	}
}
