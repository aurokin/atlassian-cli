package resolve

import (
	"net/url"
	"regexp"
	"slices"
	"strings"

	"github.com/aurokin/atlassian-cli/internal/apperr"
)

// Jira resolves Jira issue and project keys and URLs.
var Jira Parser = jiraParser{}

var (
	jiraIssueKeyRe   = regexp.MustCompile(`^[A-Z][A-Z0-9]+-[0-9]+$`)
	jiraProjectKeyRe = regexp.MustCompile(`^[A-Z][A-Z0-9]+$`)
)

type jiraParser struct{}

// Parse recognizes a bare issue key (PROJ-123), a bare project key (PROJ), a
// /browse/<KEY> URL, and a /jira/.../projects/<KEY> URL (with an optional
// issue hint).
func (jiraParser) Parse(input string) (Resource, bool) {
	if jiraIssueKeyRe.MatchString(input) {
		return Resource{Kind: KindJiraIssue, Product: productJira, Input: input, Key: input}, true
	}
	if jiraProjectKeyRe.MatchString(input) {
		return Resource{Kind: KindJiraProject, Product: productJira, Input: input, Key: input}, true
	}
	return parseJiraURL(input)
}

func parseJiraURL(input string) (Resource, bool) {
	u, ok := parseHTTPSiteURL(input)
	if !ok {
		return Resource{}, false
	}
	segs := pathSegments(u.Path)

	// /browse/<KEY> — KEY-123 is an issue, KEY is a project.
	if len(segs) >= 2 && segs[0] == "browse" {
		switch token := segs[1]; {
		case jiraIssueKeyRe.MatchString(token):
			return jiraURLResource(KindJiraIssue, u, input, token), true
		case jiraProjectKeyRe.MatchString(token):
			return jiraURLResource(KindJiraProject, u, input, token), true
		}
	}

	// /jira/.../projects/<KEY>[/...] — a project, unless an issue hint is
	// present. A "jira" prefix segment is required so an unrelated path that
	// merely contains "projects/<KEY>" does not resolve as a Jira project.
	for i, s := range segs {
		if s != "projects" || i+1 >= len(segs) || !slices.Contains(segs[:i], "jira") {
			continue
		}
		key := segs[i+1]
		if !jiraProjectKeyRe.MatchString(key) {
			continue
		}
		if iss := jiraIssueHint(u, segs); iss != "" {
			return jiraURLResource(KindJiraIssue, u, input, iss), true
		}
		return jiraURLResource(KindJiraProject, u, input, key), true
	}
	return Resource{}, false
}

func jiraURLResource(kind ResourceKind, u *url.URL, input, key string) Resource {
	return Resource{Kind: kind, Product: productJira, Input: input, SiteHost: u.Host, Key: key}
}

// jiraIssueHint extracts an issue key from a project-scoped Jira URL, via the
// selectedIssue query parameter or an "issues/<KEY-123>" path segment.
func jiraIssueHint(u *url.URL, segs []string) string {
	if q := u.Query().Get("selectedIssue"); jiraIssueKeyRe.MatchString(q) {
		return q
	}
	for i, s := range segs {
		if s == "issues" && i+1 < len(segs) && jiraIssueKeyRe.MatchString(segs[i+1]) {
			return segs[i+1]
		}
	}
	return ""
}

// CanonicalURL builds the stable /browse/<KEY> URL, which Jira serves for both
// issues and projects.
func (jiraParser) CanonicalURL(baseURL string, r Resource) (string, error) {
	if r.Key == "" {
		return "", apperr.InvalidInput("cannot build a Jira URL without a key")
	}
	return strings.TrimRight(baseURL, "/") + "/browse/" + r.Key, nil
}

// parseHTTPSiteURL parses input as an absolute http(s) URL. It returns false
// for a non-URL, a non-http(s) scheme, a missing host, or an embedded
// credential (userinfo), none of which belong in a resource locator.
func parseHTTPSiteURL(input string) (*url.URL, bool) {
	u, err := url.Parse(input)
	if err != nil || u.Host == "" || u.User != nil {
		return nil, false
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, false
	}
	return u, true
}

// pathSegments splits a URL path into its non-empty segments.
func pathSegments(p string) []string {
	var out []string
	for _, s := range strings.Split(p, "/") {
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}
