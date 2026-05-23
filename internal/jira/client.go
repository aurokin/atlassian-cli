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
	"mime/multipart"
	"net/url"
	"strconv"

	"github.com/aurokin/atlassian-cli/internal/apperr"
	"github.com/aurokin/atlassian-cli/internal/httpclient"
	"github.com/aurokin/atlassian-cli/internal/restutil"
)

// productName labels this product in shared structured error messages.
const productName = "Jira"

// Client is a typed Jira API client bound to one authenticated site. It embeds
// restutil.Base for the shared request plumbing (Get/Send/APIBase/SetLimit).
type Client struct {
	restutil.Base
}

// New wraps an authenticated httpclient.Client as a Jira client.
func New(c *httpclient.Client) *Client {
	return &Client{Base: restutil.Base{HTTP: c, Product: productName}}
}

// setLimit records a positive limit as Jira's maxResults page-size parameter.
func setLimit(q url.Values, limit int) {
	restutil.SetLimit(q, "maxResults", limit)
}

// Myself returns the authenticated account (GET /myself).
func (c *Client) Myself(ctx context.Context) (json.RawMessage, error) {
	return c.Get(ctx, "/myself")
}

// SearchUsers finds users matching a query string — an email or display-name
// fragment — via GET /user/search. The response is a JSON array of users.
func (c *Client) SearchUsers(ctx context.Context, query string) (json.RawMessage, error) {
	q := url.Values{}
	q.Set("query", query)
	return c.Get(ctx, restutil.WithQuery("/user/search", q))
}

// GetProject returns a single project by id or key (GET /project/{idOrKey}).
func (c *Client) GetProject(ctx context.Context, idOrKey string) (json.RawMessage, error) {
	return c.Get(ctx, "/project/"+url.PathEscape(idOrKey))
}

// SearchProjects returns a page of projects (GET /project/search).
func (c *Client) SearchProjects(ctx context.Context, limit int) (json.RawMessage, error) {
	q := url.Values{}
	setLimit(q, limit)
	return c.Get(ctx, restutil.WithQuery("/project/search", q))
}

// GetIssue returns a single issue by id or key (GET /issue/{idOrKey}).
//
// fields and expand are passed through verbatim as the Jira `fields` and
// `expand` query parameters when non-empty, letting a caller pull a custom
// field set (for example "*all", "summary,comment", or a specific custom field
// id) or expanded data (such as "changelog" or "renderedFields"). An empty
// fields uses Jira's default navigable field set.
func (c *Client) GetIssue(ctx context.Context, idOrKey, fields, expand string) (json.RawMessage, error) {
	q := url.Values{}
	if fields != "" {
		q.Set("fields", fields)
	}
	if expand != "" {
		q.Set("expand", expand)
	}
	return c.Get(ctx, restutil.WithQuery("/issue/"+url.PathEscape(idOrKey), q))
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
	return c.Get(ctx, restutil.WithQuery("/search/jql", q))
}

// ListComments returns a page of comments on an issue
// (GET /issue/{idOrKey}/comment).
func (c *Client) ListComments(ctx context.Context, issue string, limit int) (json.RawMessage, error) {
	q := url.Values{}
	setLimit(q, limit)
	return c.Get(ctx, restutil.WithQuery("/issue/"+url.PathEscape(issue)+"/comment", q))
}

// GetComment returns a single comment (GET /issue/{idOrKey}/comment/{id}).
func (c *Client) GetComment(ctx context.Context, issue, commentID string) (json.RawMessage, error) {
	return c.Get(ctx, "/issue/"+url.PathEscape(issue)+"/comment/"+url.PathEscape(commentID))
}

// CreateIssue creates an issue (POST /issue) from the given field map and
// returns the raw creation response (id, key, self).
func (c *Client) CreateIssue(ctx context.Context, fields map[string]any) (json.RawMessage, error) {
	return c.Send(ctx, "POST", "/issue", map[string]any{"fields": fields})
}

// EditIssue updates an issue's fields (PUT /issue/{idOrKey}). Jira returns no
// body on success.
func (c *Client) EditIssue(ctx context.Context, idOrKey string, fields map[string]any) error {
	_, err := c.Send(ctx, "PUT", "/issue/"+url.PathEscape(idOrKey),
		map[string]any{"fields": fields})
	return err
}

// GetTransitions returns the transitions available on an issue from its
// current status (GET /issue/{idOrKey}/transitions).
func (c *Client) GetTransitions(ctx context.Context, idOrKey string) (json.RawMessage, error) {
	return c.Get(ctx, "/issue/"+url.PathEscape(idOrKey)+"/transitions")
}

// DoTransition applies a transition to an issue
// (POST /issue/{idOrKey}/transitions). Jira returns no body on success.
func (c *Client) DoTransition(ctx context.Context, idOrKey, transitionID string) error {
	_, err := c.Send(ctx, "POST", "/issue/"+url.PathEscape(idOrKey)+"/transitions",
		map[string]any{"transition": map[string]string{"id": transitionID}})
	return err
}

// CreateComment adds a comment to an issue (POST /issue/{idOrKey}/comment). The
// body is an ADF document; the created comment is returned.
func (c *Client) CreateComment(ctx context.Context, issue string, body json.RawMessage) (json.RawMessage, error) {
	return c.Send(ctx, "POST", "/issue/"+url.PathEscape(issue)+"/comment",
		map[string]any{"body": body})
}

// EditComment replaces a comment's body (PUT /issue/{idOrKey}/comment/{id}).
// The body is an ADF document; the updated comment is returned.
func (c *Client) EditComment(ctx context.Context, issue, commentID string, body json.RawMessage) (json.RawMessage, error) {
	return c.Send(ctx, "PUT", "/issue/"+url.PathEscape(issue)+"/comment/"+url.PathEscape(commentID),
		map[string]any{"body": body})
}

// DeleteComment removes a comment (DELETE /issue/{idOrKey}/comment/{id}). Jira
// returns no body on success.
func (c *Client) DeleteComment(ctx context.Context, issue, commentID string) error {
	_, err := c.Send(ctx, "DELETE",
		"/issue/"+url.PathEscape(issue)+"/comment/"+url.PathEscape(commentID), nil)
	return err
}

// AssignIssue sets or clears the issue's assignee
// (PUT /issue/{idOrKey}/assignee). A nil accountID sends
// `{"accountId": null}`, which Jira treats as unassigned. Jira returns no
// body on success.
func (c *Client) AssignIssue(ctx context.Context, idOrKey string, accountID *string) error {
	var v any
	if accountID != nil {
		v = *accountID
	}
	_, err := c.Send(ctx, "PUT", "/issue/"+url.PathEscape(idOrKey)+"/assignee",
		map[string]any{"accountId": v})
	return err
}

// AddWatcher adds accountID as a watcher of the issue
// (POST /issue/{idOrKey}/watchers). The request body is the bare account id
// as a JSON string; an empty accountID sends no body, which Jira treats as
// "the authenticated user". Jira returns no body on success.
func (c *Client) AddWatcher(ctx context.Context, idOrKey, accountID string) error {
	var payload any
	if accountID != "" {
		payload = accountID
	}
	_, err := c.Send(ctx, "POST", "/issue/"+url.PathEscape(idOrKey)+"/watchers", payload)
	return err
}

// RemoveWatcher removes accountID from the issue's watchers
// (DELETE /issue/{idOrKey}/watchers?accountId=<id>). accountID is required;
// Jira does not infer the caller for the DELETE form. Returns no body on
// success.
func (c *Client) RemoveWatcher(ctx context.Context, idOrKey, accountID string) error {
	q := url.Values{}
	q.Set("accountId", accountID)
	_, err := c.Send(ctx, "DELETE",
		restutil.WithQuery("/issue/"+url.PathEscape(idOrKey)+"/watchers", q), nil)
	return err
}

// ListWatchers returns the issue's watcher list
// (GET /issue/{idOrKey}/watchers).
func (c *Client) ListWatchers(ctx context.Context, idOrKey string) (json.RawMessage, error) {
	return c.Get(ctx, "/issue/"+url.PathEscape(idOrKey)+"/watchers")
}

// CreateIssueLink creates a directional link between two issues
// (POST /issueLink). inward and outward are issue keys; linkType is the name
// of the link type (e.g. "Blocks"). Jira returns no body on success.
func (c *Client) CreateIssueLink(ctx context.Context, inward, outward, linkType string) error {
	_, err := c.Send(ctx, "POST", "/issueLink", map[string]any{
		"type":         map[string]string{"name": linkType},
		"inwardIssue":  map[string]string{"key": inward},
		"outwardIssue": map[string]string{"key": outward},
	})
	return err
}

// ListIssueLinkTypes returns the issue link types available on the site
// (GET /issueLinkType).
func (c *Client) ListIssueLinkTypes(ctx context.Context) (json.RawMessage, error) {
	return c.Get(ctx, "/issueLinkType")
}

// ListFields returns the global catalog of issue fields (GET /field), the
// source for discovering field ids/types accepted by create/edit and the
// issue-view --fields selector. The response is a JSON array.
func (c *Client) ListFields(ctx context.Context) (json.RawMessage, error) {
	return c.Get(ctx, "/field")
}

// ListWorklogs returns a page of an issue's worklog entries
// (GET /issue/{idOrKey}/worklog).
func (c *Client) ListWorklogs(ctx context.Context, idOrKey string, limit int) (json.RawMessage, error) {
	q := url.Values{}
	setLimit(q, limit)
	return c.Get(ctx, restutil.WithQuery("/issue/"+url.PathEscape(idOrKey)+"/worklog", q))
}

// AddWorklog appends a worklog entry to an issue
// (POST /issue/{idOrKey}/worklog). timeSpent is passed through verbatim, so
// callers supply the Jira duration form (e.g. "3h 30m"). commentADF is the
// ADF document for the optional comment; a nil RawMessage omits it.
func (c *Client) AddWorklog(ctx context.Context, idOrKey, timeSpent string, commentADF json.RawMessage) (json.RawMessage, error) {
	payload := map[string]any{"timeSpent": timeSpent}
	if commentADF != nil {
		payload["comment"] = commentADF
	}
	return c.Send(ctx, "POST", "/issue/"+url.PathEscape(idOrKey)+"/worklog", payload)
}

// ListIssueAttachments returns the issue with only its attachment field
// populated. Jira has no standalone attachment-list endpoint; an issue's
// attachments are carried in fields.attachment, so this is GetIssue scoped to
// that one field.
func (c *Client) ListIssueAttachments(ctx context.Context, idOrKey string) (json.RawMessage, error) {
	return c.GetIssue(ctx, idOrKey, "attachment", "")
}

// GetAttachment returns a single attachment's metadata, including the absolute
// content URL that locates its binary (GET /attachment/{id}).
func (c *Client) GetAttachment(ctx context.Context, id string) (json.RawMessage, error) {
	return c.Get(ctx, "/attachment/"+url.PathEscape(id))
}

// FetchAttachmentData downloads an attachment's binary content from the
// absolute content URL on its metadata. The same-origin check still applies, so
// a content URL pointing off the configured site is rejected at request time.
func (c *Client) FetchAttachmentData(ctx context.Context, contentURL string) ([]byte, error) {
	if contentURL == "" {
		return nil, apperr.InvalidInput("attachment has no content URL")
	}
	// An attachment is binary, so accept any content type rather than the JSON
	// default a normal API GET sends.
	resp, err := c.HTTP.DoAccepting(ctx, "GET", contentURL, nil, "*/*")
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
}

// AddAttachment uploads a file to an issue (POST /issue/{idOrKey}/attachments)
// as multipart/form-data under the "file" part. filename is the name recorded
// on the attachment; r supplies its bytes. The response is the JSON array of
// created attachment records.
func (c *Client) AddAttachment(ctx context.Context, idOrKey, filename string, r io.Reader) (json.RawMessage, error) {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormFile("file", filename)
	if err != nil {
		return nil, apperr.New(apperr.CodeRequestEncodeFailed,
			"could not build the attachment upload body: "+err.Error())
	}
	if _, err := io.Copy(fw, r); err != nil {
		return nil, apperr.New(apperr.CodeRequestEncodeFailed,
			"could not read the attachment file: "+err.Error())
	}
	if err := mw.Close(); err != nil {
		return nil, apperr.New(apperr.CodeRequestEncodeFailed,
			"could not finalize the attachment upload body: "+err.Error())
	}
	path := "/issue/" + url.PathEscape(idOrKey) + "/attachments"
	return c.Upload(ctx, "POST", path, mw.FormDataContentType(), &buf)
}

// decodeError wraps a pagination decode failure as a structured error.
func decodeError(err error) error {
	return restutil.DecodeError(productName, err)
}

// offsetCursor parses a startAt cursor ("" means 0), the offset we sent for the
// current page. Deriving the next offset from what we requested — rather than
// the startAt the response echoes back — keeps --all correct even when a server
// omits or misreports startAt.
func offsetCursor(cursor string) int {
	if cursor == "" {
		return 0
	}
	n, _ := strconv.Atoi(cursor)
	return n
}

// offsetNext computes the next startAt cursor for an offset-paginated endpoint:
// the empty string when this page is the last, otherwise the locally-derived
// end offset. count is this page's item count and total the reported total.
func offsetNext(sent, count, total int) string {
	if count == 0 || sent+count >= total {
		return ""
	}
	return strconv.Itoa(sent + count)
}

// SearchProjectsAll follows /project/search to completion and returns an
// aggregated {"values": [...]} body. /project/search is offset paginated.
func (c *Client) SearchProjectsAll(ctx context.Context, limit int) (json.RawMessage, error) {
	items, err := restutil.FollowAll(ctx, "",
		func(ctx context.Context, cursor string) (json.RawMessage, error) {
			q := url.Values{}
			setLimit(q, limit)
			if cursor != "" {
				q.Set("startAt", cursor)
			}
			return c.Get(ctx, restutil.WithQuery("/project/search", q))
		},
		func(raw json.RawMessage, cursor string) ([]json.RawMessage, string, error) {
			var pg struct {
				Values []json.RawMessage `json:"values"`
				Total  int               `json:"total"`
				IsLast bool              `json:"isLast"`
			}
			if err := json.Unmarshal(raw, &pg); err != nil {
				return nil, "", decodeError(err)
			}
			if pg.IsLast {
				return pg.Values, "", nil
			}
			return pg.Values, offsetNext(offsetCursor(cursor), len(pg.Values), pg.Total), nil
		},
	)
	if err != nil {
		return nil, err
	}
	return restutil.Aggregate(productName, "values", items)
}

// SearchIssuesAll follows /search/jql to completion and returns an aggregated
// {"issues": [...]} body. /search/jql is token paginated via nextPageToken.
func (c *Client) SearchIssuesAll(ctx context.Context, jql string, limit int) (json.RawMessage, error) {
	items, err := restutil.FollowAll(ctx, "",
		func(ctx context.Context, cursor string) (json.RawMessage, error) {
			q := url.Values{}
			q.Set("jql", jql)
			q.Set("fields", "*navigable")
			setLimit(q, limit)
			if cursor != "" {
				q.Set("nextPageToken", cursor)
			}
			return c.Get(ctx, restutil.WithQuery("/search/jql", q))
		},
		func(raw json.RawMessage, _ string) ([]json.RawMessage, string, error) {
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
	return restutil.Aggregate(productName, "issues", items)
}

// ListCommentsAll follows an issue's comment list to completion and returns an
// aggregated {"comments": [...]} body. The endpoint is offset paginated.
func (c *Client) ListCommentsAll(ctx context.Context, issue string, limit int) (json.RawMessage, error) {
	items, err := restutil.FollowAll(ctx, "",
		func(ctx context.Context, cursor string) (json.RawMessage, error) {
			q := url.Values{}
			setLimit(q, limit)
			if cursor != "" {
				q.Set("startAt", cursor)
			}
			return c.Get(ctx, restutil.WithQuery("/issue/"+url.PathEscape(issue)+"/comment", q))
		},
		func(raw json.RawMessage, cursor string) ([]json.RawMessage, string, error) {
			var pg struct {
				Comments []json.RawMessage `json:"comments"`
				Total    int               `json:"total"`
			}
			if err := json.Unmarshal(raw, &pg); err != nil {
				return nil, "", decodeError(err)
			}
			return pg.Comments, offsetNext(offsetCursor(cursor), len(pg.Comments), pg.Total), nil
		},
	)
	if err != nil {
		return nil, err
	}
	return restutil.Aggregate(productName, "comments", items)
}

// ListWorklogsAll follows an issue's worklog list to completion and returns
// an aggregated {"worklogs": [...]} body. The endpoint is offset paginated.
func (c *Client) ListWorklogsAll(ctx context.Context, idOrKey string, limit int) (json.RawMessage, error) {
	items, err := restutil.FollowAll(ctx, "",
		func(ctx context.Context, cursor string) (json.RawMessage, error) {
			q := url.Values{}
			setLimit(q, limit)
			if cursor != "" {
				q.Set("startAt", cursor)
			}
			return c.Get(ctx, restutil.WithQuery("/issue/"+url.PathEscape(idOrKey)+"/worklog", q))
		},
		func(raw json.RawMessage, cursor string) ([]json.RawMessage, string, error) {
			var pg struct {
				Worklogs []json.RawMessage `json:"worklogs"`
				Total    int               `json:"total"`
			}
			if err := json.Unmarshal(raw, &pg); err != nil {
				return nil, "", decodeError(err)
			}
			return pg.Worklogs, offsetNext(offsetCursor(cursor), len(pg.Worklogs), pg.Total), nil
		},
	)
	if err != nil {
		return nil, err
	}
	return restutil.Aggregate(productName, "worklogs", items)
}
