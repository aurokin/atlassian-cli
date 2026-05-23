// Package oauth performs the Atlassian OAuth 2.0 (3LO) token operations the
// interactive `auth login --token-style oauth-3lo` flow and the request-time
// refresh path depend on: building the authorize URL (with PKCE), exchanging an
// authorization code for tokens, refreshing an access token, and listing the
// sites a token can reach.
//
// It deliberately bypasses httpclient.Client. That client signs every request
// with a stored credential and redacts only credential-bearing *headers* in its
// --trace output; the OAuth token endpoint instead carries the client_secret,
// the authorization code, and the refresh token in the request *form body*.
// Because those secrets are not in headers, this package owns its own handling:
// it never logs a request (so the form body can never leak), builds error
// messages only from the server's response, and maps an expired/revoked grant
// to a clear re-authenticate apperr rather than a generic failure.
package oauth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/aurokin/atlassian-cli/internal/apperr"
)

// audience is the fixed Atlassian API audience for 3LO authorize/exchange.
const audience = "api.atlassian.com"

// offlineAccessScope is the scope that makes Atlassian return a refresh token.
const offlineAccessScope = "offline_access"

// defaultTimeout bounds an OAuth HTTP call when the caller supplies no client.
const defaultTimeout = 30 * time.Second

// Endpoints holds the base URLs for the three OAuth interactions. The zero
// value resolves to the production Atlassian endpoints via DefaultEndpoints;
// tests point each field at an httptest server.
type Endpoints struct {
	// Authorize is the authorize endpoint the user's browser is sent to.
	Authorize string
	// Token is the token endpoint for code exchange and refresh.
	Token string
	// Resources is the accessible-resources endpoint used to resolve cloud IDs.
	Resources string
}

// DefaultEndpoints returns the production Atlassian OAuth 2.0 (3LO) endpoints.
func DefaultEndpoints() Endpoints {
	return Endpoints{
		Authorize: "https://auth.atlassian.com/authorize",
		Token:     "https://auth.atlassian.com/oauth/token",
		Resources: "https://api.atlassian.com/oauth/token/accessible-resources",
	}
}

// Options configures a Client. Every field is optional: a zero Endpoints uses
// DefaultEndpoints, a nil HTTPClient uses a default-timeout client, and a nil
// Now uses time.Now.
type Options struct {
	Endpoints  Endpoints
	HTTPClient *http.Client
	Now        func() time.Time
}

// Client performs OAuth 2.0 (3LO) token operations for one confidential client
// (one client_id/client_secret pair).
type Client struct {
	clientID     string
	clientSecret string
	endpoints    Endpoints
	httpClient   *http.Client
	now          func() time.Time
}

// New builds a Client for the given confidential-client credentials.
func New(clientID, clientSecret string, opts Options) *Client {
	eps := opts.Endpoints
	def := DefaultEndpoints()
	if eps.Authorize == "" {
		eps.Authorize = def.Authorize
	}
	if eps.Token == "" {
		eps.Token = def.Token
	}
	if eps.Resources == "" {
		eps.Resources = def.Resources
	}
	hc := opts.HTTPClient
	if hc == nil {
		hc = &http.Client{Timeout: defaultTimeout}
	}
	now := opts.Now
	if now == nil {
		now = time.Now
	}
	return &Client{
		clientID:     clientID,
		clientSecret: clientSecret,
		endpoints:    eps,
		httpClient:   hc,
		now:          now,
	}
}

// TokenBundle is the OAuth credential set produced by Exchange and Refresh.
// Expiry is an absolute instant computed from the response's expires_in using
// the client's clock, so callers compare it directly against "now".
//
// ClientSecret is part of the persisted keychain bundle but is never returned
// by Exchange or Refresh (the token endpoint does not echo it); the storage
// layer populates it before marshaling. It is kept on this type so a single
// shape round-trips through the keychain.
type TokenBundle struct {
	ClientSecret string    `json:"client_secret,omitempty"`
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	Expiry       time.Time `json:"expiry"`
}

// Marshal serializes the bundle to the JSON string stored under one site in the
// secrets.Store. The store is string-keyed, so the whole bundle (client_secret
// included) round-trips as a single opaque value; it is written only to the
// keychain or the 0600 fallback file, never to config.json.
func (b TokenBundle) Marshal() (string, error) {
	data, err := json.Marshal(b)
	if err != nil {
		return "", apperr.New("oauth_error", fmt.Sprintf("marshal OAuth token bundle: %v", err))
	}
	return string(data), nil
}

// ParseBundle deserializes a stored bundle string produced by Marshal.
func ParseBundle(data string) (TokenBundle, error) {
	var b TokenBundle
	if err := json.Unmarshal([]byte(data), &b); err != nil {
		return TokenBundle{}, apperr.New("oauth_error", "stored OAuth token bundle is not valid JSON")
	}
	return b, nil
}

// Expired reports whether the access token is at or past its expiry as of now.
// A zero expiry is treated as expired so an unknown-expiry bundle refreshes
// rather than being trusted indefinitely.
func (b TokenBundle) Expired(now time.Time) bool {
	if b.Expiry.IsZero() {
		return true
	}
	return !now.Before(b.Expiry)
}

// AuthorizeParams are the per-flow inputs to an authorize URL.
type AuthorizeParams struct {
	// RedirectURI must byte-for-byte match the callback registered on the app.
	RedirectURI string
	// Scopes is the full scope set to request. Callers should pass it through
	// EnsureOfflineAccess so a refresh token is granted.
	Scopes []string
	// State is an opaque anti-CSRF value the callback handler must echo back.
	State string
	// CodeChallenge is the PKCE S256 challenge derived from the verifier.
	CodeChallenge string
	// Prompt, when non-empty, sets the OAuth `prompt` parameter (for example
	// "consent" to always re-show the consent screen). Left empty, the
	// parameter is omitted, which lets Atlassian skip consent the user has
	// already granted. The caller owns this policy.
	Prompt string
}

// AuthorizeURL builds the browser URL that starts the consent flow. It requests
// the authorization-code grant with PKCE (S256) and the fixed api.atlassian.com
// audience. Scopes are sent verbatim — use EnsureOfflineAccess first.
func (c *Client) AuthorizeURL(p AuthorizeParams) string {
	q := url.Values{}
	q.Set("audience", audience)
	q.Set("client_id", c.clientID)
	q.Set("scope", strings.Join(p.Scopes, " "))
	q.Set("redirect_uri", p.RedirectURI)
	q.Set("state", p.State)
	q.Set("response_type", "code")
	if p.Prompt != "" {
		q.Set("prompt", p.Prompt)
	}
	q.Set("code_challenge", p.CodeChallenge)
	q.Set("code_challenge_method", "S256")
	return c.endpoints.Authorize + "?" + q.Encode()
}

// Exchange swaps an authorization code for a token bundle. codeVerifier is the
// PKCE verifier whose challenge was sent to AuthorizeURL; redirectURI must match
// the value used there.
func (c *Client) Exchange(ctx context.Context, code, codeVerifier, redirectURI string) (TokenBundle, error) {
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("client_id", c.clientID)
	form.Set("client_secret", c.clientSecret)
	form.Set("code", code)
	form.Set("redirect_uri", redirectURI)
	form.Set("code_verifier", codeVerifier)
	return c.postToken(ctx, form)
}

// Refresh exchanges a refresh token for a new bundle. Atlassian rotates the
// refresh token on every refresh, so the returned bundle carries a *new*
// refresh token that the caller must persist.
func (c *Client) Refresh(ctx context.Context, refreshToken string) (TokenBundle, error) {
	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("client_id", c.clientID)
	form.Set("client_secret", c.clientSecret)
	form.Set("refresh_token", refreshToken)
	return c.postToken(ctx, form)
}

// tokenResponse is the success shape of the token endpoint.
type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
	TokenType    string `json:"token_type"`
}

// postToken POSTs a form to the token endpoint and parses the result. The form
// carries the client_secret/code/refresh_token, so it is never logged; errors
// are built only from the response.
func (c *Client) postToken(ctx context.Context, form url.Values) (TokenBundle, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoints.Token, strings.NewReader(form.Encode()))
	if err != nil {
		return TokenBundle{}, apperr.InvalidInput(fmt.Sprintf("build token request: %v", err))
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		// err can embed the request URL but never the form body.
		return TokenBundle{}, apperr.New("oauth_request_failed", fmt.Sprintf("OAuth token request failed: %v", err))
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return TokenBundle{}, apperr.New("oauth_request_failed", fmt.Sprintf("read OAuth token response: %v", err))
	}

	if resp.StatusCode >= 400 {
		return TokenBundle{}, c.tokenError(resp.StatusCode, body)
	}

	var tr tokenResponse
	if err := json.Unmarshal(body, &tr); err != nil {
		return TokenBundle{}, apperr.New("oauth_error", "OAuth token response was not valid JSON")
	}
	if tr.AccessToken == "" {
		return TokenBundle{}, apperr.New("oauth_error", "OAuth token response did not include an access token")
	}
	return TokenBundle{
		AccessToken:  tr.AccessToken,
		RefreshToken: tr.RefreshToken,
		Expiry:       c.now().Add(time.Duration(tr.ExpiresIn) * time.Second),
	}, nil
}

// oauthErrorBody is the RFC 6749 error shape returned by the token endpoint.
type oauthErrorBody struct {
	Err     string `json:"error"`
	ErrDesc string `json:"error_description"`
}

// tokenError maps a non-2xx token response to a structured apperr. An expired
// or revoked grant (invalid_grant / invalid_token) becomes a re-authenticate
// error so the caller can tell the user to run `auth login` again, rather than
// a generic failure. The OAuth error code/description come from the response
// and never contain the request's secrets.
func (c *Client) tokenError(status int, body []byte) *apperr.Error {
	var oe oauthErrorBody
	_ = json.Unmarshal(body, &oe)
	switch oe.Err {
	case "invalid_grant", "invalid_token":
		e := apperr.Unauthorized(orDefault(oe.ErrDesc, "the OAuth grant is expired or has been revoked"))
		e.Next = "Re-run `auth login --token-style oauth-3lo` to re-authorize."
		return e
	}
	msg := oe.Err
	if oe.ErrDesc != "" {
		if msg != "" {
			msg += ": " + oe.ErrDesc
		} else {
			msg = oe.ErrDesc
		}
	}
	e := apperr.New("oauth_error", orDefault(msg, fmt.Sprintf("OAuth token request failed with HTTP %d", status)))
	e.Status = status
	return e
}

// Resource is one site (Jira/Confluence tenant) an access token can reach. The
// ID is the cloud ID used to address the api.atlassian.com gateway.
type Resource struct {
	ID        string   `json:"id"`
	URL       string   `json:"url"`
	Name      string   `json:"name"`
	Scopes    []string `json:"scopes"`
	AvatarURL string   `json:"avatarUrl"`
}

// AccessibleResources lists the sites the access token is authorized for. The
// token rides in the Authorization header (not a logged body); the result is
// used to resolve the cloud ID for a configured site URL.
func (c *Client) AccessibleResources(ctx context.Context, accessToken string) ([]Resource, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.endpoints.Resources, nil)
	if err != nil {
		return nil, apperr.InvalidInput(fmt.Sprintf("build accessible-resources request: %v", err))
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, apperr.New("oauth_request_failed", fmt.Sprintf("accessible-resources request failed: %v", err))
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, apperr.New("oauth_request_failed", fmt.Sprintf("read accessible-resources response: %v", err))
	}
	if resp.StatusCode == http.StatusUnauthorized {
		e := apperr.Unauthorized("the OAuth access token was rejected by accessible-resources")
		e.Next = "Re-run `auth login --token-style oauth-3lo` to re-authorize."
		return nil, e
	}
	if resp.StatusCode >= 400 {
		e := apperr.New("oauth_error", fmt.Sprintf("accessible-resources request failed with HTTP %d", resp.StatusCode))
		e.Status = resp.StatusCode
		return nil, e
	}
	var resources []Resource
	if err := json.Unmarshal(body, &resources); err != nil {
		return nil, apperr.New("oauth_error", "accessible-resources response was not valid JSON")
	}
	return resources, nil
}

// EnsureOfflineAccess returns scopes with offline_access appended if it is not
// already present (case-sensitive, matching Atlassian's scope strings).
// offline_access is what makes Atlassian return a refresh token, so the flow
// always requests it regardless of the user's configured scopes.
func EnsureOfflineAccess(scopes []string) []string {
	for _, s := range scopes {
		if s == offlineAccessScope {
			return scopes
		}
	}
	out := make([]string, 0, len(scopes)+1)
	out = append(out, scopes...)
	out = append(out, offlineAccessScope)
	return out
}

// GenerateState returns a cryptographically random, URL-safe anti-CSRF state.
func GenerateState() (string, error) {
	return randomURLSafe(24)
}

// PKCE holds a generated PKCE verifier/challenge pair (S256).
type PKCE struct {
	Verifier  string
	Challenge string
}

// GeneratePKCE produces a fresh PKCE pair: a high-entropy verifier and its
// SHA-256 (S256) challenge, both base64url-encoded without padding per RFC 7636.
func GeneratePKCE() (PKCE, error) {
	verifier, err := randomURLSafe(32)
	if err != nil {
		return PKCE{}, err
	}
	sum := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(sum[:])
	return PKCE{Verifier: verifier, Challenge: challenge}, nil
}

// randomURLSafe returns n random bytes encoded as base64url without padding.
func randomURLSafe(n int) (string, error) {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", apperr.New("oauth_error", fmt.Sprintf("generate random value: %v", err))
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func orDefault(s, fallback string) string {
	if s != "" {
		return s
	}
	return fallback
}
