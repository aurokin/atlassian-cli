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
	"encoding/json"
	"net/url"

	"github.com/aurokin/atlassian-cli/internal/apperr"
)

// MaxFollowPages caps how many pages an --all request follows, guarding
// against an unbounded loop from a malformed cursor or page token.
const MaxFollowPages = 100

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
	return apperr.New("response_decode_failed",
		"could not decode the "+product+" API response: "+err.Error())
}
