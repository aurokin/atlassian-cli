// Package jira is a typed client over the Jira Cloud REST v3 API. It wraps the
// shared httpclient.Client and returns raw JSON response bodies: callers render
// them verbatim under --json or decode them through the models in this package
// for human output.
package jira

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/url"
	"strconv"

	"github.com/aurokin/atlassian-cli/internal/apperr"
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

// send marshals payload as a JSON request body, issues method against an
// API-relative path, and returns the raw response body. A nil payload sends no
// body. A non-2xx response surfaces as the structured *apperr.Error from
// httpclient.
func (c *Client) send(ctx context.Context, method, path string, payload any) (json.RawMessage, error) {
	var body io.Reader
	if payload != nil {
		b, err := json.Marshal(payload)
		if err != nil {
			return nil, apperr.New("request_encode_failed",
				"could not encode the Jira API request body: "+err.Error())
		}
		body = bytes.NewReader(b)
	}
	resp, err := c.http.Do(ctx, method, path, body)
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

// CreateIssue creates an issue (POST /issue) from the given field map and
// returns the raw creation response (id, key, self).
func (c *Client) CreateIssue(ctx context.Context, fields map[string]any) (json.RawMessage, error) {
	return c.send(ctx, "POST", "/issue", map[string]any{"fields": fields})
}

// EditIssue updates an issue's fields (PUT /issue/{idOrKey}). Jira returns no
// body on success.
func (c *Client) EditIssue(ctx context.Context, idOrKey string, fields map[string]any) error {
	_, err := c.send(ctx, "PUT", "/issue/"+url.PathEscape(idOrKey),
		map[string]any{"fields": fields})
	return err
}

// GetTransitions returns the transitions available on an issue from its
// current status (GET /issue/{idOrKey}/transitions).
func (c *Client) GetTransitions(ctx context.Context, idOrKey string) (json.RawMessage, error) {
	return c.get(ctx, "/issue/"+url.PathEscape(idOrKey)+"/transitions")
}

// DoTransition applies a transition to an issue
// (POST /issue/{idOrKey}/transitions). Jira returns no body on success.
func (c *Client) DoTransition(ctx context.Context, idOrKey, transitionID string) error {
	_, err := c.send(ctx, "POST", "/issue/"+url.PathEscape(idOrKey)+"/transitions",
		map[string]any{"transition": map[string]string{"id": transitionID}})
	return err
}

// CreateComment adds a comment to an issue (POST /issue/{idOrKey}/comment). The
// body is an ADF document; the created comment is returned.
func (c *Client) CreateComment(ctx context.Context, issue string, body json.RawMessage) (json.RawMessage, error) {
	return c.send(ctx, "POST", "/issue/"+url.PathEscape(issue)+"/comment",
		map[string]any{"body": body})
}

// EditComment replaces a comment's body (PUT /issue/{idOrKey}/comment/{id}).
// The body is an ADF document; the updated comment is returned.
func (c *Client) EditComment(ctx context.Context, issue, commentID string, body json.RawMessage) (json.RawMessage, error) {
	return c.send(ctx, "PUT", "/issue/"+url.PathEscape(issue)+"/comment/"+url.PathEscape(commentID),
		map[string]any{"body": body})
}

// DeleteComment removes a comment (DELETE /issue/{idOrKey}/comment/{id}). Jira
// returns no body on success.
func (c *Client) DeleteComment(ctx context.Context, issue, commentID string) error {
	_, err := c.send(ctx, "DELETE",
		"/issue/"+url.PathEscape(issue)+"/comment/"+url.PathEscape(commentID), nil)
	return err
}

// maxFollowPages caps how many pages an --all request follows, guarding
// against an unbounded loop from a malformed cursor.
const maxFollowPages = 100

// decodeError wraps a pagination decode failure as a structured error.
func decodeError(err error) error {
	return apperr.New("response_decode_failed",
		"could not decode the Jira API response: "+err.Error())
}

// followAll follows a paginated endpoint to completion. fetch issues the
// request for a cursor ("" for the first page); extract pulls a page's raw
// items and the next cursor ("" when there is no next page) from a response.
// It returns every collected item, stopping at maxFollowPages.
func followAll(ctx context.Context,
	fetch func(context.Context, string) (json.RawMessage, error),
	extract func(json.RawMessage) ([]json.RawMessage, string, error),
) ([]json.RawMessage, error) {
	all := []json.RawMessage{}
	cursor := ""
	for page := 0; page < maxFollowPages; page++ {
		raw, err := fetch(ctx, cursor)
		if err != nil {
			return nil, err
		}
		items, next, err := extract(raw)
		if err != nil {
			return nil, err
		}
		all = append(all, items...)
		if next == "" {
			break
		}
		cursor = next
	}
	return all, nil
}

// synthesize assembles an aggregate list body, {"<key>": [<item>, ...]}. Items
// are kept verbatim, so every field each API page returned is preserved.
func synthesize(key string, items []json.RawMessage) (json.RawMessage, error) {
	out, err := json.Marshal(map[string][]json.RawMessage{key: items})
	if err != nil {
		return nil, decodeError(err)
	}
	return out, nil
}

// SearchProjectsAll follows /project/search to completion and returns an
// aggregated {"values": [...]} body. /project/search is offset paginated.
func (c *Client) SearchProjectsAll(ctx context.Context, limit int) (json.RawMessage, error) {
	items, err := followAll(ctx,
		func(ctx context.Context, cursor string) (json.RawMessage, error) {
			q := url.Values{}
			setLimit(q, limit)
			if cursor != "" {
				q.Set("startAt", cursor)
			}
			return c.get(ctx, withQuery("/project/search", q))
		},
		func(raw json.RawMessage) ([]json.RawMessage, string, error) {
			var pg struct {
				Values  []json.RawMessage `json:"values"`
				StartAt int               `json:"startAt"`
				Total   int               `json:"total"`
				IsLast  bool              `json:"isLast"`
			}
			if err := json.Unmarshal(raw, &pg); err != nil {
				return nil, "", decodeError(err)
			}
			next := ""
			if end := pg.StartAt + len(pg.Values); !pg.IsLast && len(pg.Values) > 0 && end < pg.Total {
				next = strconv.Itoa(end)
			}
			return pg.Values, next, nil
		},
	)
	if err != nil {
		return nil, err
	}
	return synthesize("values", items)
}

// SearchIssuesAll follows /search/jql to completion and returns an aggregated
// {"issues": [...]} body. /search/jql is token paginated via nextPageToken.
func (c *Client) SearchIssuesAll(ctx context.Context, jql string, limit int) (json.RawMessage, error) {
	items, err := followAll(ctx,
		func(ctx context.Context, cursor string) (json.RawMessage, error) {
			q := url.Values{}
			q.Set("jql", jql)
			q.Set("fields", "*navigable")
			setLimit(q, limit)
			if cursor != "" {
				q.Set("nextPageToken", cursor)
			}
			return c.get(ctx, withQuery("/search/jql", q))
		},
		func(raw json.RawMessage) ([]json.RawMessage, string, error) {
			var pg struct {
				Issues        []json.RawMessage `json:"issues"`
				NextPageToken string            `json:"nextPageToken"`
			}
			if err := json.Unmarshal(raw, &pg); err != nil {
				return nil, "", decodeError(err)
			}
			return pg.Issues, pg.NextPageToken, nil
		},
	)
	if err != nil {
		return nil, err
	}
	return synthesize("issues", items)
}

// ListCommentsAll follows an issue's comment list to completion and returns an
// aggregated {"comments": [...]} body. The endpoint is offset paginated.
func (c *Client) ListCommentsAll(ctx context.Context, issue string, limit int) (json.RawMessage, error) {
	items, err := followAll(ctx,
		func(ctx context.Context, cursor string) (json.RawMessage, error) {
			q := url.Values{}
			setLimit(q, limit)
			if cursor != "" {
				q.Set("startAt", cursor)
			}
			return c.get(ctx, withQuery("/issue/"+url.PathEscape(issue)+"/comment", q))
		},
		func(raw json.RawMessage) ([]json.RawMessage, string, error) {
			var pg struct {
				Comments []json.RawMessage `json:"comments"`
				StartAt  int               `json:"startAt"`
				Total    int               `json:"total"`
			}
			if err := json.Unmarshal(raw, &pg); err != nil {
				return nil, "", decodeError(err)
			}
			next := ""
			if end := pg.StartAt + len(pg.Comments); len(pg.Comments) > 0 && end < pg.Total {
				next = strconv.Itoa(end)
			}
			return pg.Comments, next, nil
		},
	)
	if err != nil {
		return nil, err
	}
	return synthesize("comments", items)
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
