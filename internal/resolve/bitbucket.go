package resolve

import (
	"net/url"
	"regexp"
	"strings"

	"github.com/aurokin/atlassian-cli/internal/apperr"
)

// Bitbucket resolves Bitbucket Cloud repository, pull request, issue, and
// commit URLs, plus the bare "workspace/repo" form.
var Bitbucket Parser = bitbucketParser{}

// bitbucketSlugRe matches a single workspace or repository slug: an
// alphanumeric start followed by alphanumerics, dots, hyphens, or underscores.
var bitbucketSlugRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

// bitbucketRepoRe matches the bare "workspace/repo" form.
var bitbucketRepoRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*/[A-Za-z0-9][A-Za-z0-9._-]*$`)

var bitbucketDigitsRe = regexp.MustCompile(`^[0-9]+$`)

type bitbucketParser struct{}

// Parse recognizes a bare "workspace/repo" target and a Bitbucket web URL for a
// repository, pull request, issue, or commit.
func (bitbucketParser) Parse(input string) (Resource, bool) {
	if bitbucketRepoRe.MatchString(input) {
		ws, repo, _ := strings.Cut(input, "/")
		return Resource{
			Kind:    KindBitbucketRepo,
			Product: productBitbucket,
			Input:   input,
			Key:     ws + "/" + repo,
		}, true
	}
	return parseBitbucketURL(input)
}

func parseBitbucketURL(input string) (Resource, bool) {
	u, ok := parseHTTPSiteURL(input)
	if !ok {
		return Resource{}, false
	}
	segs := pathSegments(u.Path)
	// Every Bitbucket resource is rooted at /{workspace}/{repo}.
	if len(segs) < 2 || !bitbucketSlugRe.MatchString(segs[0]) || !bitbucketSlugRe.MatchString(segs[1]) {
		return Resource{}, false
	}
	key := segs[0] + "/" + segs[1]
	base := Resource{Product: productBitbucket, Input: input, SiteHost: u.Host, Key: key}

	if len(segs) == 2 {
		base.Kind = KindBitbucketRepo
		return base, true
	}

	switch segs[2] {
	case "pull-requests":
		if len(segs) >= 4 && bitbucketDigitsRe.MatchString(segs[3]) {
			base.Kind = KindBitbucketPullRequest
			base.ID = segs[3]
			return base, true
		}
	case "issues":
		if len(segs) >= 4 && bitbucketDigitsRe.MatchString(segs[3]) {
			base.Kind = KindBitbucketIssue
			base.ID = segs[3]
			return base, true
		}
	case "commits":
		if len(segs) >= 4 && segs[3] != "" {
			base.Kind = KindBitbucketCommit
			base.ID = segs[3]
			return base, true
		}
	}

	// A repository sub-page we do not model specifically (src, branches, …)
	// still resolves to its repository.
	base.Kind = KindBitbucketRepo
	return base, true
}

// CanonicalURL builds the bitbucket.org web URL for a resolved resource. The
// base may be the API host (api.bitbucket.org/2.0) for a bare key, which is
// mapped to the web host.
func (bitbucketParser) CanonicalURL(baseURL string, r Resource) (string, error) {
	if r.Key == "" {
		return "", apperr.InvalidInput("cannot build a Bitbucket URL without a workspace/repo")
	}
	web := bitbucketWebBase(baseURL)
	repoURL := web + "/" + r.Key
	switch r.Kind {
	case KindBitbucketRepo:
		return repoURL, nil
	case KindBitbucketPullRequest:
		if r.ID == "" {
			return "", apperr.InvalidInput("cannot build a pull request URL without an id")
		}
		return repoURL + "/pull-requests/" + r.ID, nil
	case KindBitbucketIssue:
		if r.ID == "" {
			return "", apperr.InvalidInput("cannot build an issue URL without an id")
		}
		return repoURL + "/issues/" + r.ID, nil
	case KindBitbucketCommit:
		if r.ID == "" {
			return "", apperr.InvalidInput("cannot build a commit URL without a hash")
		}
		return repoURL + "/commits/" + r.ID, nil
	default:
		return "", apperr.InvalidInput("cannot build a Bitbucket URL for an unknown resource kind")
	}
}

// bitbucketWebBase maps a configured base URL to the Bitbucket Cloud web host.
// The API base (api.bitbucket.org/2.0) becomes bitbucket.org; any other host is
// kept (with its path dropped) so a URL input's own host is honored.
func bitbucketWebBase(baseURL string) string {
	u, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil || u.Host == "" {
		return strings.TrimRight(baseURL, "/")
	}
	host := u.Host
	if host == "api.bitbucket.org" {
		host = "bitbucket.org"
	}
	scheme := u.Scheme
	if scheme == "" {
		scheme = "https"
	}
	return scheme + "://" + host
}
