// Package httpclient resolves Atlassian API URLs and executes signed HTTP
// requests, mapping non-2xx responses to structured apperr.Error values.
package httpclient

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
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

	case auth.StyleCloudScoped:
		if t.CloudID == "" {
			return "", apperr.InvalidInput("cloud-scoped requires a cloud_id")
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
			return "", apperr.New("untrusted_url", fmt.Sprintf(
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

// Client executes signed requests against one Target.
type Client struct {
	target Target
	cred   auth.Credential
	http   *http.Client
}

// New builds a Client. If hc is nil a client with a default timeout is used.
func New(target Target, cred auth.Credential, hc *http.Client) *Client {
	if hc == nil {
		hc = &http.Client{Timeout: defaultTimeout}
	}
	return &Client{target: target, cred: cred, http: hc}
}

// APIBase returns the resolved API base URL the client sends requests against.
func (c *Client) APIBase() (string, error) {
	return c.target.APIBase()
}

// Do resolves pathOrURL, signs the request, executes it, and reads the body.
// A non-2xx status is returned as a structured *apperr.Error alongside the
// populated Response.
func (c *Client) Do(ctx context.Context, method, pathOrURL string, body io.Reader) (*Response, error) {
	target, err := c.target.ResolveURL(pathOrURL)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, method, target, body)
	if err != nil {
		return nil, apperr.InvalidInput(fmt.Sprintf("build request: %v", err))
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if err := c.cred.Sign(req); err != nil {
		return nil, err
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, apperr.New("request_failed", fmt.Sprintf("request to %s failed: %v", target, err))
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, apperr.New("request_failed", fmt.Sprintf("read response body: %v", err))
	}
	out := &Response{Status: resp.StatusCode, Header: resp.Header, Body: raw}

	if resp.StatusCode >= 400 {
		return out, c.classify(out)
	}
	return out, nil
}

// classify maps a non-2xx Response to a structured *apperr.Error, enriched
// with target context.
func (c *Client) classify(resp *Response) *apperr.Error {
	msg := extractMessage(resp.Body)
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
	default:
		e = apperr.New("http_error", orDefault(msg, fmt.Sprintf("request failed with HTTP %d", resp.Status)))
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

// extractMessage makes a best-effort attempt to pull a human message out of a
// Jira-, Confluence-, or Bitbucket-shaped error body. Jira/Confluence put the
// text at top level (message / errorMessages); Bitbucket nests it under an
// "error" object ({"error":{"message":...,"detail":...}}).
func extractMessage(body []byte) string {
	var shaped struct {
		Message       string   `json:"message"`
		ErrorMessages []string `json:"errorMessages"`
		Error         struct {
			Message string `json:"message"`
			Detail  string `json:"detail"`
		} `json:"error"`
	}
	if json.Unmarshal(body, &shaped) == nil {
		if shaped.Message != "" {
			return shaped.Message
		}
		if len(shaped.ErrorMessages) > 0 {
			return strings.Join(shaped.ErrorMessages, "; ")
		}
		if shaped.Error.Message != "" {
			return shaped.Error.Message
		}
		if shaped.Error.Detail != "" {
			return shaped.Error.Detail
		}
	}
	return ""
}

func orDefault(s, fallback string) string {
	if s != "" {
		return s
	}
	return fallback
}
