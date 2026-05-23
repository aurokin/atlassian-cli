// Package bitbucket is a typed client over the Bitbucket Cloud REST 2.0 API.
// It wraps the shared httpclient.Client and returns raw JSON response bodies:
// callers render them verbatim under --json or decode them through the models
// in this package for human output. The shape mirrors the internal/jira and
// internal/conf clients so all three products sit on one foundation.
package bitbucket

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/aurokin/atlassian-cli/internal/apperr"
	"github.com/aurokin/atlassian-cli/internal/httpclient"
	"github.com/aurokin/atlassian-cli/internal/restutil"
)

// productName labels this product in shared structured error messages.
const productName = "Bitbucket"

// Client is a typed Bitbucket API client bound to one authenticated site.
type Client struct {
	http *httpclient.Client
}

// New wraps an authenticated httpclient.Client as a Bitbucket client.
func New(c *httpclient.Client) *Client {
	return &Client{http: c}
}

// APIBase returns the resolved Bitbucket API base URL the client sends
// requests to.
func (c *Client) APIBase() (string, error) {
	return c.http.APIBase()
}

// get issues a GET against an API-relative path (or an absolute pagination
// URL) and returns the raw body. A non-2xx response surfaces as a structured
// *apperr.Error, upgraded to feature_disabled where Bitbucket signals a
// switched-off capability.
func (c *Client) get(ctx context.Context, path string) (json.RawMessage, error) {
	resp, err := c.http.Do(ctx, "GET", path, nil)
	if err != nil {
		return nil, remapError(resp, err)
	}
	return json.RawMessage(resp.Body), nil
}

// send marshals payload as a JSON request body, issues method against an
// API-relative path, and returns the raw response body. A nil payload sends no
// body.
func (c *Client) send(ctx context.Context, method, path string, payload any) (json.RawMessage, error) {
	var body io.Reader
	if payload != nil {
		b, err := json.Marshal(payload)
		if err != nil {
			return nil, apperr.New("request_encode_failed",
				"could not encode the Bitbucket API request body: "+err.Error())
		}
		body = bytes.NewReader(b)
	}
	resp, err := c.http.Do(ctx, method, path, body)
	if err != nil {
		return nil, remapError(resp, err)
	}
	return json.RawMessage(resp.Body), nil
}

// CurrentUser returns the authenticated account (GET /user).
func (c *Client) CurrentUser(ctx context.Context) (json.RawMessage, error) {
	return c.get(ctx, "/user")
}

// GetRepository returns a single repository (GET /repositories/{ws}/{repo}).
func (c *Client) GetRepository(ctx context.Context, workspace, repo string) (json.RawMessage, error) {
	return c.get(ctx, "/repositories/"+url.PathEscape(workspace)+"/"+url.PathEscape(repo))
}

// ListRepositories returns one page of a workspace's repositories
// (GET /repositories/{workspace}).
func (c *Client) ListRepositories(ctx context.Context, workspace string, limit int) (json.RawMessage, error) {
	q := url.Values{}
	setLimit(q, limit)
	return c.get(ctx, restutil.WithQuery("/repositories/"+url.PathEscape(workspace), q))
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
	all := []json.RawMessage{}
	next := firstPath
	for page := 0; page < restutil.MaxFollowPages && next != ""; page++ {
		raw, err := c.get(ctx, next)
		if err != nil {
			return nil, err
		}
		var pg struct {
			Values []json.RawMessage `json:"values"`
			Next   string            `json:"next"`
		}
		if err := json.Unmarshal(raw, &pg); err != nil {
			return nil, decodeError(err)
		}
		all = append(all, pg.Values...)
		next = pg.Next
	}
	// A non-empty cursor here means the loop stopped at the page cap, not
	// because the API ran out of pages — the aggregate is incomplete.
	if next != "" {
		return nil, restutil.TruncatedError()
	}
	out, err := json.Marshal(map[string][]json.RawMessage{"values": all})
	if err != nil {
		return nil, decodeError(err)
	}
	return out, nil
}

// remapError upgrades a generic transport error to a Bitbucket-specific one
// where the response body signals it. Today it recognizes a disabled
// repository capability (issue tracker / wiki) and re-codes it as
// feature_disabled so an agent can distinguish "enable the feature" from
// "the resource is missing or hidden". Any other error is returned unchanged.
func remapError(resp *httpclient.Response, err error) error {
	var ae *apperr.Error
	if resp == nil || !errors.As(err, &ae) {
		return err
	}
	if !featureDisabledSignal(resp.Status, resp.Body) {
		return err
	}
	fd := apperr.FeatureDisabled(ae.Message)
	fd.Status = ae.Status
	fd.Product = ae.Product
	fd.Site = ae.Site
	fd.TokenStyle = ae.TokenStyle
	fd.APIBaseURL = ae.APIBaseURL
	fd.Next = "Enable the feature in the repository settings, or target a repository that has it enabled."
	return fd
}

// featureDisabledSignal reports whether a non-2xx Bitbucket response indicates
// a switched-off repository capability rather than a genuinely missing or
// hidden resource. Bitbucket reports a disabled issue tracker or wiki as a 404
// (or 403) whose message names the feature; the check is intentionally narrow
// to avoid recoding ordinary not-found responses.
func featureDisabledSignal(status int, body []byte) bool {
	if status != http.StatusNotFound && status != http.StatusForbidden {
		return false
	}
	msg := strings.ToLower(errorMessage(body))
	// Bitbucket phrases a disabled capability as "Repository has no issue
	// tracker." / "Repository has no wiki." Matching the full "has no <feature>"
	// phrase (rather than the bare feature word) avoids re-coding an ordinary
	// not-found for a repository that merely happens to be named "wiki".
	return strings.Contains(msg, "has no issue tracker") ||
		strings.Contains(msg, "has no wiki")
}

// errorMessage pulls the human message out of a Bitbucket error body
// ({"error":{"message"|"detail"}} or a top-level "message"), falling back to
// the trimmed raw body.
func errorMessage(body []byte) string {
	trimmed := strings.TrimSpace(string(body))
	if trimmed == "" {
		return ""
	}
	var shaped struct {
		Message string `json:"message"`
		Error   struct {
			Message string `json:"message"`
			Detail  string `json:"detail"`
		} `json:"error"`
	}
	if json.Unmarshal(body, &shaped) == nil {
		switch {
		case shaped.Error.Message != "":
			return shaped.Error.Message
		case shaped.Error.Detail != "":
			return shaped.Error.Detail
		case shaped.Message != "":
			return shaped.Message
		}
	}
	return trimmed
}

// decodeError wraps a pagination-aggregation or decode failure as a structured
// error.
func decodeError(err error) error {
	return restutil.DecodeError(productName, err)
}

// setLimit records a positive limit as the Bitbucket pagelen parameter.
func setLimit(q url.Values, limit int) {
	if limit > 0 {
		q.Set("pagelen", strconv.Itoa(limit))
	}
}
