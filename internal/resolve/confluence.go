package resolve

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/aurokin/atlassian-cli/internal/apperr"
)

// Confluence resolves Confluence page and space URLs and bare page ids.
var Confluence Parser = confluenceParser{}

var (
	confluenceIDRe       = regexp.MustCompile(`^[0-9]+$`)
	confluenceSpaceKeyRe = regexp.MustCompile(`^~?[A-Za-z0-9._-]+$`)
)

type confluenceParser struct{}

// Parse recognizes a bare numeric page id (123456), a
// /wiki/spaces/<SPACEKEY>/pages/<id>/<slug> page URL, and a
// /wiki/spaces/<SPACEKEY>[/overview] space URL.
func (confluenceParser) Parse(input string) (Resource, bool) {
	if confluenceIDRe.MatchString(input) {
		return Resource{Kind: KindConfluencePage, Product: productConfluence, Input: input, ID: input}, true
	}
	return parseConfluenceURL(input)
}

func parseConfluenceURL(input string) (Resource, bool) {
	u, err := url.Parse(input)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return Resource{}, false
	}
	segs := pathSegments(u.Path)

	// /wiki/spaces/<SPACEKEY>[/...] — a page when a pages/<id> segment
	// follows, a space when nothing or only "overview" follows.
	for i, s := range segs {
		if s != "spaces" || i+1 >= len(segs) {
			continue
		}
		spaceKey := segs[i+1]
		if !confluenceSpaceKeyRe.MatchString(spaceKey) {
			continue
		}
		rest := segs[i+2:]

		if len(rest) >= 2 && rest[0] == "pages" && confluenceIDRe.MatchString(rest[1]) {
			r := Resource{
				Kind:     KindConfluencePage,
				Product:  productConfluence,
				Input:    input,
				SiteHost: u.Host,
				Key:      spaceKey,
				ID:       rest[1],
			}
			if len(rest) >= 3 {
				// Confluence renders spaces in a page slug as "+".
				r.Title = strings.ReplaceAll(rest[2], "+", " ")
			}
			return r, true
		}

		if len(rest) == 0 || (len(rest) == 1 && rest[0] == "overview") {
			return Resource{
				Kind:     KindConfluenceSpace,
				Product:  productConfluence,
				Input:    input,
				SiteHost: u.Host,
				Key:      spaceKey,
			}, true
		}
	}
	return Resource{}, false
}

// CanonicalURL builds the stable browser URL for a Confluence resource. A page
// with a known space key gets the /spaces/<key>/pages/<id> form; a bare page id
// falls back to the legacy viewpage.action form, which Confluence redirects.
func (confluenceParser) CanonicalURL(baseURL string, r Resource) (string, error) {
	wiki := confluenceWikiBase(baseURL)
	switch r.Kind {
	case KindConfluenceSpace:
		if r.Key == "" {
			return "", apperr.InvalidInput("cannot build a Confluence space URL without a space key")
		}
		return wiki + "/spaces/" + r.Key, nil
	case KindConfluencePage:
		if r.ID == "" {
			return "", apperr.InvalidInput("cannot build a Confluence page URL without a page id")
		}
		if r.Key != "" {
			return wiki + "/spaces/" + r.Key + "/pages/" + r.ID, nil
		}
		return wiki + "/pages/viewpage.action?pageId=" + r.ID, nil
	default:
		return "", apperr.InvalidInput(fmt.Sprintf("cannot build a Confluence URL for kind %q", r.Kind))
	}
}

// confluenceWikiBase normalizes a site root to a /wiki-rooted base, adding the
// "/wiki" segment only when absent — the same rule as httpclient.APIBase.
func confluenceWikiBase(baseURL string) string {
	base := strings.TrimRight(baseURL, "/")
	if !strings.HasSuffix(base, "/wiki") {
		base += "/wiki"
	}
	return base
}
