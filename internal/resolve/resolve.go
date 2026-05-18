// Package resolve turns a user-supplied Atlassian URL or bare key into a
// structured Resource. Resolution is pure string parsing: no network call, no
// clock, and no environment access happen here.
package resolve

import (
	"fmt"
	"strings"

	"github.com/aurokin/atlassian-cli/internal/apperr"
)

// ResourceKind classifies a resolved Atlassian resource.
type ResourceKind string

const (
	KindJiraIssue       ResourceKind = "jira_issue"
	KindJiraProject     ResourceKind = "jira_project"
	KindConfluencePage  ResourceKind = "confluence_page"
	KindConfluenceSpace ResourceKind = "confluence_space"
)

// Product identifiers recorded on a resolved Resource.
const (
	productJira       = "jira"
	productConfluence = "confluence"
)

// Resource is a resolved Atlassian resource. It is safe to render as JSON.
type Resource struct {
	Kind     ResourceKind `json:"kind"`
	Product  string       `json:"product"`
	Input    string       `json:"input"`
	SiteHost string       `json:"site_host,omitempty"` // host, when the input was a URL
	Key      string       `json:"key,omitempty"`       // issue / project / space key
	ID       string       `json:"id,omitempty"`        // numeric page id
	Title    string       `json:"title,omitempty"`     // URL slug, best-effort
}

// Parser resolves an input string for one product and builds canonical URLs
// for the resources it recognizes.
type Parser interface {
	// Parse resolves a trimmed, non-empty input string into a Resource. The
	// boolean is false when the input matches no form this parser recognizes.
	Parse(input string) (Resource, bool)
	// CanonicalURL builds the canonical browser URL for a resolved Resource,
	// rooted at baseURL (the configured site URL).
	CanonicalURL(baseURL string, r Resource) (string, error)
}

// ParserFor returns the resolver for an Atlassian product. It accepts the
// appinfo product values "jira" and "confluence"; any other value yields a
// structured error.
func ParserFor(product string) (Parser, error) {
	switch product {
	case productJira:
		return Jira, nil
	case productConfluence:
		return Confluence, nil
	default:
		return nil, apperr.InvalidInput(fmt.Sprintf("unknown product %q", product))
	}
}

// Resolve trims input and resolves it with p. An empty input, or one that the
// parser does not recognize, yields a structured "unresolved" error.
func Resolve(p Parser, input string) (Resource, error) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return Resource{}, apperr.New("unresolved", "no input to resolve")
	}
	r, ok := p.Parse(trimmed)
	if !ok {
		return Resource{}, apperr.New("unresolved",
			fmt.Sprintf("could not resolve %q to a known resource", trimmed))
	}
	return r, nil
}
