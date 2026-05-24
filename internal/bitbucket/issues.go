package bitbucket

import (
	"context"
	"encoding/json"
	"net/url"
	"strconv"

	"github.com/aurokin/atlassian-cli/internal/restutil"
)

// issuesBase returns the issues collection path for a repository.
func issuesBase(workspace, repo string) string {
	return "/repositories/" + url.PathEscape(workspace) + "/" + url.PathEscape(repo) + "/issues"
}

// issuesQuery assembles the list query: a page size and an optional state
// filter. Bitbucket issue states are lower-case (new, open, resolved, …); the
// caller passes the value through verbatim, and an empty state lists all.
func issuesQuery(state string, limit int) url.Values {
	q := url.Values{}
	setLimit(q, limit)
	if state != "" {
		q.Set("state", state)
	}
	return q
}

// ListIssues returns one page of a repository's issues
// (GET /repositories/{ws}/{repo}/issues). A repository with its issue tracker
// disabled surfaces as a feature_disabled error.
func (c *Client) ListIssues(ctx context.Context, workspace, repo, state string, limit int) (json.RawMessage, error) {
	return c.Get(ctx, restutil.WithQuery(issuesBase(workspace, repo), issuesQuery(state, limit)))
}

// ListIssuesAll follows a repository's issue listing to completion and returns
// an aggregated {"values": [...]} body.
func (c *Client) ListIssuesAll(ctx context.Context, workspace, repo, state string, limit int) (json.RawMessage, error) {
	return c.followValues(ctx, restutil.WithQuery(issuesBase(workspace, repo), issuesQuery(state, limit)))
}

// GetIssue returns a single issue by numeric id
// (GET /repositories/{ws}/{repo}/issues/{id}).
func (c *Client) GetIssue(ctx context.Context, workspace, repo string, id int) (json.RawMessage, error) {
	return c.Get(ctx, issuesBase(workspace, repo)+"/"+strconv.Itoa(id))
}

// CreateIssueOptions holds the fields an issue creation accepts. Title is
// required by Bitbucket; Kind and Priority are passed through verbatim so the
// API validates the allowed vocabulary.
type CreateIssueOptions struct {
	Title    string
	Body     string
	Kind     string
	Priority string
}

// CreateIssue files a new issue (POST /repositories/{ws}/{repo}/issues) and
// returns the created issue.
func (c *Client) CreateIssue(ctx context.Context, workspace, repo string, opts CreateIssueOptions) (json.RawMessage, error) {
	body := map[string]any{"title": opts.Title}
	if opts.Body != "" {
		body["content"] = map[string]string{"raw": opts.Body}
	}
	if opts.Kind != "" {
		body["kind"] = opts.Kind
	}
	if opts.Priority != "" {
		body["priority"] = opts.Priority
	}
	return c.Send(ctx, "POST", issuesBase(workspace, repo), body)
}

// UpdateIssueOptions holds the issue fields an update may change. Each field is
// sent only when non-empty, so a caller can transition state alone (e.g.
// State: "resolved") or edit several fields at once. The values are passed
// through verbatim so the API validates the allowed vocabulary.
type UpdateIssueOptions struct {
	Title    string
	Body     string
	State    string
	Kind     string
	Priority string
}

// IsEmpty reports whether no field is set, so the command layer can reject an
// update that would change nothing.
func (o UpdateIssueOptions) IsEmpty() bool {
	return o.Title == "" && o.Body == "" && o.State == "" && o.Kind == "" && o.Priority == ""
}

// UpdateIssue changes one or more fields of an existing issue
// (PUT /repositories/{ws}/{repo}/issues/{id}) and returns the updated issue.
// Setting State transitions the issue (e.g. new → resolved).
func (c *Client) UpdateIssue(ctx context.Context, workspace, repo string, id int, opts UpdateIssueOptions) (json.RawMessage, error) {
	body := map[string]any{}
	if opts.Title != "" {
		body["title"] = opts.Title
	}
	if opts.Body != "" {
		body["content"] = map[string]string{"raw": opts.Body}
	}
	if opts.State != "" {
		body["state"] = opts.State
	}
	if opts.Kind != "" {
		body["kind"] = opts.Kind
	}
	if opts.Priority != "" {
		body["priority"] = opts.Priority
	}
	return c.Send(ctx, "PUT", issuesBase(workspace, repo)+"/"+strconv.Itoa(id), body)
}
