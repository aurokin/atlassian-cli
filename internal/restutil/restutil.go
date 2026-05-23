// Package restutil holds the product-agnostic helpers shared by the typed
// Jira and Confluence REST clients: query-string assembly, the --all page
// cap, and the generic response decoder with its structured-error wrap.
//
// Only helpers that are byte-for-byte identical across the product clients
// live here. Per-product machinery that merely looks similar — the
// pagination followers, the limit parameter name, the request helpers — stays
// in each client package, where it is coupled to that API's shape.
package restutil

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

// MaxFollowPages caps how many pages an --all request follows, guarding
// against an unbounded loop from a malformed cursor or page token.
const MaxFollowPages = 100

// TruncatedError reports that an --all request reached MaxFollowPages while the
// API still had more pages. Returning it (rather than silently aggregating a
// partial result) keeps --all honest: the caller is told the set is incomplete
// instead of receiving a truncated list that looks whole.
func TruncatedError() error {
	return apperr.New(apperr.CodeResultTruncated,
		"the result has more pages than --all will follow (cap: 100 pages); "+
			"narrow the query or raise --limit to fetch larger pages")
}

// Base is the product-agnostic core of a typed REST client: it wraps an
// authenticated httpclient.Client and provides the request plumbing every
// product client shares — a GET, a JSON-body send, the resolved API base, and
// the limit-parameter setter. Each product embeds a Base and layers its typed
// endpoint methods on top, so the byte-identical request path lives here once.
type Base struct {
	// HTTP is the authenticated, site-bound transport.
	HTTP *httpclient.Client
	// Product names the product in encode-error messages (e.g. "Jira").
	Product string
	// RemapError optionally post-processes a request error before it is
	// returned, given the (possibly nil) response. Bitbucket sets it to upgrade
	// a disabled-capability response to feature_disabled. Nil means identity.
	RemapError func(resp *httpclient.Response, err error) error
}

// APIBase returns the resolved API base URL the client sends requests to.
func (b *Base) APIBase() (string, error) {
	return b.HTTP.APIBase()
}

// Get issues a GET against an API-relative path or absolute (pagination) URL
// and returns the raw body. A non-2xx response surfaces as the structured
// *apperr.Error from httpclient, passed through RemapError when set.
func (b *Base) Get(ctx context.Context, pathOrURL string) (json.RawMessage, error) {
	resp, err := b.HTTP.Do(ctx, "GET", pathOrURL, nil)
	if err != nil {
		return nil, b.remap(resp, err)
	}
	return json.RawMessage(resp.Body), nil
}

// Send marshals payload as a JSON request body, issues method against an
// API-relative path or absolute URL, and returns the raw response body. A nil
// payload sends no body. A non-2xx response surfaces as the structured
// *apperr.Error from httpclient, passed through RemapError when set.
func (b *Base) Send(ctx context.Context, method, pathOrURL string, payload any) (json.RawMessage, error) {
	var body io.Reader
	if payload != nil {
		raw, err := json.Marshal(payload)
		if err != nil {
			return nil, apperr.New(apperr.CodeRequestEncodeFailed,
				"could not encode the "+b.Product+" API request body: "+err.Error())
		}
		body = bytes.NewReader(raw)
	}
	resp, err := b.HTTP.Do(ctx, method, pathOrURL, body)
	if err != nil {
		return nil, b.remap(resp, err)
	}
	return json.RawMessage(resp.Body), nil
}

// Upload sends body with an explicit Content-Type (e.g. a multipart boundary)
// and the X-Atlassian-Token: no-check header attachment-upload endpoints
// require, returning the raw response body. A non-2xx response surfaces as the
// structured *apperr.Error from httpclient, passed through RemapError when set.
func (b *Base) Upload(ctx context.Context, method, pathOrURL, contentType string, body io.Reader) (json.RawMessage, error) {
	resp, err := b.HTTP.DoUpload(ctx, method, pathOrURL, contentType, body)
	if err != nil {
		return nil, b.remap(resp, err)
	}
	return json.RawMessage(resp.Body), nil
}

// remap applies the optional RemapError hook to a non-nil error.
func (b *Base) remap(resp *httpclient.Response, err error) error {
	if b.RemapError == nil {
		return err
	}
	return b.RemapError(resp, err)
}

// SetLimit records a positive limit under the given page-size query parameter
// ("maxResults" for Jira, "limit" for Confluence, "pagelen" for Bitbucket). A
// non-positive limit is left unset. It is a free function (not a Base method)
// so the per-product query-builder helpers, which have no client receiver, can
// share one implementation.
func SetLimit(q url.Values, param string, limit int) {
	if limit > 0 {
		q.Set(param, strconv.Itoa(limit))
	}
}

// FollowAll follows a paginated endpoint to completion, collecting every
// page's items into one slice. It is the single follow loop shared by all
// three products' --all aggregation.
//
// initial is the first cursor: "" for products that build the first request
// from an empty cursor (Jira), or the first request URL for products that page
// by following a link (Confluence, Bitbucket). fetch issues the request for a
// cursor and returns the raw body. extract pulls a page's raw items and the
// next cursor ("" when there is no next page) from a response; it also receives
// the cursor that produced the response, which Confluence uses to thread its
// relative _links.next onto the current URL. Following stops at MaxFollowPages,
// returning TruncatedError rather than a silently partial result.
func FollowAll(
	ctx context.Context,
	initial string,
	fetch func(ctx context.Context, cursor string) (json.RawMessage, error),
	extract func(raw json.RawMessage, cursor string) (items []json.RawMessage, next string, err error),
) ([]json.RawMessage, error) {
	all := []json.RawMessage{}
	cursor := initial
	done := false
	for page := 0; page < MaxFollowPages; page++ {
		raw, err := fetch(ctx, cursor)
		if err != nil {
			return nil, err
		}
		items, next, err := extract(raw, cursor)
		if err != nil {
			return nil, err
		}
		all = append(all, items...)
		if next == "" {
			done = true
			break
		}
		cursor = next
	}
	// Exiting the loop without an empty next means the cap was hit and the
	// aggregate is incomplete.
	if !done {
		return nil, TruncatedError()
	}
	return all, nil
}

// Aggregate assembles a synthesized list body, {"<key>": [<item>, ...]}, from
// items collected by FollowAll. Items are kept verbatim, so every field each
// API page returned is preserved. product names the API in the (effectively
// unreachable) marshal-error message.
func Aggregate(product, key string, items []json.RawMessage) (json.RawMessage, error) {
	out, err := json.Marshal(map[string][]json.RawMessage{key: items})
	if err != nil {
		return nil, DecodeError(product, err)
	}
	return out, nil
}

// WithQuery appends an encoded query string to path when it is non-empty.
func WithQuery(path string, q url.Values) string {
	if len(q) == 0 {
		return path
	}
	return path + "?" + q.Encode()
}

// Decode unmarshals a raw API response body into a model value, wrapping a
// decode failure as a structured error. product names the API in the error
// message (e.g. "Jira", "Confluence").
func Decode[T any](raw json.RawMessage, product string) (T, error) {
	var v T
	if err := json.Unmarshal(raw, &v); err != nil {
		return v, DecodeError(product, err)
	}
	return v, nil
}

// DecodeError wraps a decode or pagination-aggregation failure as a
// structured error naming the product whose response could not be decoded.
func DecodeError(product string, err error) error {
	return apperr.New(apperr.CodeResponseDecodeFailed,
		"could not decode the "+product+" API response: "+err.Error())
}
