// Package httpclient resolves Atlassian API URLs and executes signed HTTP
// requests, mapping non-2xx responses to structured apperr.Error values.
package httpclient

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/aurokin/atlassian-cli/internal/apperr"
	"github.com/aurokin/atlassian-cli/internal/auth"
)

// Supported product identifiers.
const (
	ProductJira       = "jira"
	ProductConfluence = "confluence"
	ProductBitbucket  = "bitbucket"
)

// defaultBitbucketAPIBase is the Bitbucket Cloud REST base used when a site
// profile does not configure its own base URL. Tests point a profile's base
// URL at a local server instead.
const defaultBitbucketAPIBase = "https://api.bitbucket.org/2.0"

// defaultTimeout bounds a request when the caller does not supply its own
// *http.Client.
const defaultTimeout = 30 * time.Second

// Target describes where and how a product's API is reached. It is derived
// from a configured site profile by the command layer.
type Target struct {
	Product    string          // ProductJira, ProductConfluence, or ProductBitbucket
	TokenStyle auth.TokenStyle // selects URL resolution and signing
	SiteName   string          // configured profile name, used in diagnostics
	BaseURL    string          // configured site/instance URL
	CloudID    string          // required for auth.StyleCloudScoped
}

// APIBase computes the effective API base URL for the target.
//
// Cloud styles append the product's well-known API path. For Confluence the
// "/wiki" segment is added only if the configured URL does not already carry
// it, so both "https://x.atlassian.net" and "https://x.atlassian.net/wiki"
// resolve correctly. Data Center uses the configured URL verbatim because
// Data Center API paths are not pinned in Phase 1.
func (t Target) APIBase() (string, error) {
	if t.Product != ProductJira && t.Product != ProductConfluence && t.Product != ProductBitbucket {
		return "", apperr.InvalidInput(fmt.Sprintf("unknown product %q", t.Product))
	}
	// Strip any userinfo so a credential embedded in the configured URL can
	// never survive into the API base, which is also surfaced in diagnostics
	// (apperr.APIBaseURL) and persisted to config.
	site := stripUserinfo(strings.TrimRight(t.BaseURL, "/"))

	// Bitbucket Cloud exposes one fixed REST host for every account, so the
	// API base is taken verbatim from the configured site URL (used as the
	// override seam in tests) and falls back to the well-known Cloud base.
	// The signing path is still Basic auth (StyleCloudClassic).
	if t.Product == ProductBitbucket {
		if site == "" {
			return defaultBitbucketAPIBase, nil
		}
		return site, nil
	}

	switch t.TokenStyle {
	case auth.StyleCloudClassic:
		if site == "" {
			return "", apperr.InvalidInput("cloud-classic requires a base URL")
		}
		switch t.Product {
		case ProductJira:
			return site + "/rest/api/3", nil
		case ProductConfluence:
			if !strings.HasSuffix(site, "/wiki") {
				site += "/wiki"
			}
			return site + "/api/v2", nil
		}

	case auth.StyleCloudScoped, auth.StyleOAuth3LO:
		// OAuth 3LO reaches the same api.atlassian.com gateway as a scoped API
		// token; only the Authorization scheme differs (Bearer vs Basic), which
		// auth.Credential.Sign handles. The base URL resolution is identical.
		if t.CloudID == "" {
			return "", apperr.InvalidInput(fmt.Sprintf("%s requires a cloud_id", t.TokenStyle))
		}
		switch t.Product {
		case ProductJira:
			return "https://api.atlassian.com/ex/jira/" + t.CloudID + "/rest/api/3", nil
		case ProductConfluence:
			return "https://api.atlassian.com/ex/confluence/" + t.CloudID + "/wiki/api/v2", nil
		}

	case auth.StyleDataCenterPAT:
		if site == "" {
			return "", apperr.InvalidInput("data-center-pat requires a base URL")
		}
		return site, nil
	}

	return "", apperr.InvalidInput(fmt.Sprintf("unknown token style %q", t.TokenStyle))
}

// ResolveURL turns a user-supplied path or absolute URL into a request URL.
//
// A relative path is appended to the API base; a leading slash is cosmetic.
// An absolute URL is allowed only when its scheme and host match the
// configured site or the Atlassian API gateway for this target, so a request
// can never be retargeted to another host or downgraded to a non-matching
// scheme (for example plaintext http that would expose the signed token).
func (t Target) ResolveURL(ref string) (string, error) {
	base, err := t.APIBase()
	if err != nil {
		return "", err
	}
	u, err := url.Parse(ref)
	if err != nil {
		return "", apperr.InvalidInput(fmt.Sprintf("invalid request path %q: %v", ref, err))
	}
	if u.IsAbs() {
		origins, err := t.allowedOrigins()
		if err != nil {
			return "", err
		}
		cand := originOf(u)
		if !originAllowed(origins, cand) {
			return "", apperr.New(apperr.CodeUntrustedURL, fmt.Sprintf(
				"absolute URL %s://%s is not the configured site or Atlassian API gateway for site %q",
				cand.scheme, cand.host, t.SiteName))
		}
		// Requests are authenticated via the Authorization header, so URL
		// userinfo is never needed; drop it so a credential can never travel
		// in the URL or be echoed into an error message.
		u.User = nil
		return u.String(), nil
	}
	joined := strings.TrimRight(base, "/") + "/" + strings.TrimLeft(ref, "/")
	return stripUserinfo(joined), nil
}

// stripUserinfo removes any embedded userinfo from a URL string. A string that
// does not parse, or that carries no userinfo, is returned unchanged.
func stripUserinfo(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.User == nil {
		return raw
	}
	u.User = nil
	return u.String()
}

// origin is a normalized scheme+host pair. Both fields are lower-cased so
// comparisons are case-insensitive, matching how DNS and URL schemes behave.
type origin struct {
	scheme string
	host   string
}

// originOf normalizes the scheme and host of a parsed URL.
func originOf(u *url.URL) origin {
	return origin{scheme: strings.ToLower(u.Scheme), host: strings.ToLower(u.Host)}
}

// allowedOrigins lists the scheme+host origins an absolute request URL may
// target: the resolved API base and the configured site URL.
func (t Target) allowedOrigins() ([]origin, error) {
	base, err := t.APIBase()
	if err != nil {
		return nil, err
	}
	var origins []origin
	for _, raw := range []string{base, t.BaseURL} {
		if raw == "" {
			continue
		}
		if u, err := url.Parse(raw); err == nil && u.Host != "" {
			origins = append(origins, originOf(u))
		}
	}
	return origins, nil
}

// originAllowed reports whether cand exactly matches one of the origins.
func originAllowed(origins []origin, cand origin) bool {
	for _, o := range origins {
		if o == cand {
			return true
		}
	}
	return false
}

// Response is the outcome of an executed request.
type Response struct {
	Status int
	Header http.Header
	Body   []byte
}

// CredentialProvider yields the credential to sign a request with, resolved at
// request time. Static styles return a fixed credential; oauth-3lo returns a
// freshly refreshed access token (see internal/cli). It receives the request's
// context so a refresh can be cancelled or time out with the request.
type CredentialProvider func(ctx context.Context) (auth.Credential, error)

// Client executes signed requests against one Target.
type Client struct {
	target      Target
	credentials CredentialProvider
	http        *http.Client
	trace       io.Writer // when non-nil, request/response diagnostics are written here
}

// New builds a Client that signs every request with a fixed credential. If hc
// is nil a client with a default timeout is used.
func New(target Target, cred auth.Credential, hc *http.Client) *Client {
	return NewWithProvider(target, func(context.Context) (auth.Credential, error) { return cred, nil }, hc)
}

// NewWithProvider builds a Client that resolves its credential per request via
// provider. This is the seam oauth-3lo uses to refresh an expired access token
// lazily on the request path (which already carries a context), so neither the
// Client API nor any call site needs a refresh-aware signature.
func NewWithProvider(target Target, provider CredentialProvider, hc *http.Client) *Client {
	if hc == nil {
		hc = &http.Client{Timeout: defaultTimeout}
	}
	return &Client{target: target, credentials: provider, http: hc}
}

// EnableTrace turns on verbose request tracing, writing one line per request
// and response to w (typically os.Stderr, backing the global --trace flag).
// Credential-bearing headers are redacted. Passing nil disables tracing.
func (c *Client) EnableTrace(w io.Writer) {
	c.trace = w
}

// APIBase returns the resolved API base URL the client sends requests against.
func (c *Client) APIBase() (string, error) {
	return c.target.APIBase()
}

// Do resolves pathOrURL, signs the request, executes it, and reads the body.
// A non-2xx status is returned as a structured *apperr.Error alongside the
// populated Response.
func (c *Client) Do(ctx context.Context, method, pathOrURL string, body io.Reader) (*Response, error) {
	return c.DoAccepting(ctx, method, pathOrURL, body, "application/json")
}

// DoAccepting is Do with an explicit Accept header. Binary payloads — an
// attachment download, say — pass "*/*" rather than the JSON default, since a
// JSON Accept is wrong for a file. Behavior is otherwise identical to Do.
func (c *Client) DoAccepting(ctx context.Context, method, pathOrURL string, body io.Reader, accept string) (*Response, error) {
	return c.do(ctx, method, pathOrURL, body, accept, "application/json", nil)
}

// DoUpload signs and sends a request whose body carries the given Content-Type
// (e.g. a multipart/form-data boundary) rather than the JSON default, plus the
// X-Atlassian-Token: no-check header that Atlassian's attachment-upload
// endpoints require to bypass their XSRF check. It is the upload counterpart to
// Do; behavior is otherwise identical.
func (c *Client) DoUpload(ctx context.Context, method, pathOrURL, contentType string, body io.Reader) (*Response, error) {
	return c.do(ctx, method, pathOrURL, body, "application/json", contentType,
		map[string]string{"X-Atlassian-Token": "no-check"})
}

// do is the shared request core behind Do/DoAccepting/DoUpload. It resolves
// pathOrURL, sets the Accept header and (when body is non-nil) the given
// Content-Type, applies any extra headers, signs the request, executes it, and
// reads the body. A non-2xx status is returned as a structured *apperr.Error
// alongside the populated Response.
func (c *Client) do(ctx context.Context, method, pathOrURL string, body io.Reader, accept, contentType string, extra map[string]string) (*Response, error) {
	target, err := c.target.ResolveURL(pathOrURL)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, method, target, body)
	if err != nil {
		return nil, apperr.InvalidInput(fmt.Sprintf("build request: %v", err))
	}
	req.Header.Set("Accept", accept)
	if body != nil {
		req.Header.Set("Content-Type", contentType)
	}
	for k, v := range extra {
		req.Header.Set(k, v)
	}
	cred, err := c.credentials(ctx)
	if err != nil {
		return nil, err
	}
	if err := cred.Sign(req); err != nil {
		return nil, err
	}

	c.traceRequest(req)
	start := time.Now()
	resp, err := c.http.Do(req)
	if err != nil {
		c.traceFailure(time.Since(start), err)
		return nil, transportError(target, err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		c.traceFailure(time.Since(start), err)
		return nil, transportError(target, err)
	}
	out := &Response{Status: resp.StatusCode, Header: resp.Header, Body: raw}
	c.traceResponse(out.Status, time.Since(start), len(raw))

	if resp.StatusCode >= 400 {
		return out, c.classify(out)
	}
	return out, nil
}

// sensitiveHeaders are request headers whose values must never be written to
// the trace, since they carry credentials.
var sensitiveHeaders = map[string]bool{
	"Authorization":       true,
	"Proxy-Authorization": true,
	"Cookie":              true,
}

// traceRequest writes the request line and headers to the trace writer with
// credential-bearing header values redacted. It is a no-op when tracing is off.
func (c *Client) traceRequest(req *http.Request) {
	if c.trace == nil {
		return
	}
	fmt.Fprintf(c.trace, "[trace] > %s %s\n", req.Method, req.URL.String())
	keys := make([]string, 0, len(req.Header))
	for k := range req.Header {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		value := strings.Join(req.Header.Values(k), ", ")
		if sensitiveHeaders[http.CanonicalHeaderKey(k)] {
			value = "[redacted]"
		}
		fmt.Fprintf(c.trace, "[trace] > %s: %s\n", k, value)
	}
}

// traceResponse writes the response status, elapsed time, and body size. It is
// a no-op when tracing is off.
func (c *Client) traceResponse(status int, elapsed time.Duration, size int) {
	if c.trace == nil {
		return
	}
	fmt.Fprintf(c.trace, "[trace] < %d (%s, %d bytes)\n", status, elapsed.Round(time.Millisecond), size)
}

// traceFailure writes a transport-level failure (no HTTP response) with the
// elapsed time. It is a no-op when tracing is off.
func (c *Client) traceFailure(elapsed time.Duration, err error) {
	if c.trace == nil {
		return
	}
	fmt.Fprintf(c.trace, "[trace] < error after %s: %v\n", elapsed.Round(time.Millisecond), err)
}

// transportError classifies a transport-level failure (no usable HTTP
// response): a deadline or client timeout becomes a retryable timeout
// category, everything else a generic request_failed.
func transportError(target string, err error) *apperr.Error {
	if errors.Is(err, context.DeadlineExceeded) || isTimeout(err) {
		e := apperr.New(apperr.CodeTimeout,
			fmt.Sprintf("request to %s timed out: %v", target, err))
		e.Next = "Retry the request, or raise the client timeout if the operation is legitimately slow."
		return e
	}
	return apperr.New(apperr.CodeRequestFailed, fmt.Sprintf("request to %s failed: %v", target, err))
}

// isTimeout reports whether err is (or wraps) a net.Error that timed out.
func isTimeout(err error) bool {
	var nerr net.Error
	return errors.As(err, &nerr) && nerr.Timeout()
}

// classify maps a non-2xx Response to a structured *apperr.Error, enriched
// with target context.
func (c *Client) classify(resp *Response) *apperr.Error {
	msg := apperr.MessageFromBody(resp.Body)
	var e *apperr.Error
	switch resp.Status {
	case http.StatusUnauthorized:
		e = apperr.Unauthorized(orDefault(msg, "authentication failed"))
		e.Next = "Verify the token value, the token style, and the base URL match this credential."
	case http.StatusForbidden:
		e = apperr.Forbidden(orDefault(msg, "the authenticated account lacks permission for this resource"))
		e.Next = "Use an account or token that holds the required permission or scope."
	case http.StatusNotFound:
		e = apperr.NotFoundOrNotVisible(orDefault(msg, "resource not found or not visible to this account"))
	case http.StatusTooManyRequests:
		e = apperr.RateLimited(orDefault(msg, "rate limited by Atlassian"))
		if ra := resp.Header.Get("Retry-After"); ra != "" {
			e.Next = "Retry after " + ra + " seconds."
		}
	case http.StatusGone:
		e = apperr.New(apperr.CodeGone,
			orDefault(msg, "this API endpoint has been removed"))
		e.Status = resp.Status
		e.Next = "Upgrade the CLI; this endpoint is no longer served by Atlassian."
	default:
		e = apperr.New(apperr.CodeHTTPError, orDefault(msg, fmt.Sprintf("request failed with HTTP %d", resp.Status)))
		e.Status = resp.Status
	}
	e.Product = c.target.Product
	e.Site = c.target.SiteName
	e.TokenStyle = string(c.target.TokenStyle)
	if base, err := c.target.APIBase(); err == nil {
		e.APIBaseURL = base
	}
	return e
}

func orDefault(s, fallback string) string {
	if s != "" {
		return s
	}
	return fallback
}
