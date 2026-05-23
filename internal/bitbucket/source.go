package bitbucket

import (
	"context"
	"encoding/json"
	"net/url"
	"strings"

	"github.com/aurokin/atlassian-cli/internal/restutil"
)

// srcBase returns the source-browsing collection path for a repository.
func srcBase(workspace, repo string) string {
	return "/repositories/" + url.PathEscape(workspace) + "/" + url.PathEscape(repo) + "/src"
}

// escapePathSegments percent-escapes each segment of a slash-separated path
// while preserving the "/" separators, so a ref like "feature/x" or a nested
// file path survives URL construction intact.
func escapePathSegments(s string) string {
	parts := strings.Split(s, "/")
	for i, p := range parts {
		parts[i] = url.PathEscape(p)
	}
	return strings.Join(parts, "/")
}

// srcPath builds the /src request path for a ref and repo-relative path. An
// empty ref yields the bare /src endpoint, which Bitbucket redirects to the
// default branch's root.
func srcPath(workspace, repo, ref, path string) string {
	p := srcBase(workspace, repo)
	if ref == "" {
		return p
	}
	p += "/" + escapePathSegments(ref)
	if path != "" {
		p += "/" + escapePathSegments(path)
	}
	return p
}

// ListSource returns one page of a directory listing at ref/path
// (GET .../src/{ref}/{path}). The response is a paginated set of entries, each
// a file or directory. An empty ref lists the default branch's root.
func (c *Client) ListSource(ctx context.Context, workspace, repo, ref, path string, limit int) (json.RawMessage, error) {
	q := url.Values{}
	setLimit(q, limit)
	return c.Get(ctx, restutil.WithQuery(srcPath(workspace, repo, ref, path), q))
}

// ListSourceAll follows a directory listing to completion and returns an
// aggregated {"values": [...]} body.
func (c *Client) ListSourceAll(ctx context.Context, workspace, repo, ref, path string, limit int) (json.RawMessage, error) {
	q := url.Values{}
	setLimit(q, limit)
	return c.followValues(ctx, restutil.WithQuery(srcPath(workspace, repo, ref, path), q))
}

// GetFileContent returns the raw bytes of a file at ref/path
// (GET .../src/{ref}/{path}). The endpoint returns the file verbatim, so this
// accepts any content type rather than the JSON default.
func (c *Client) GetFileContent(ctx context.Context, workspace, repo, ref, path string) ([]byte, error) {
	return c.GetAccepting(ctx, srcPath(workspace, repo, ref, path), "*/*")
}
