// Package git infers a Bitbucket repository target from the local git
// checkout. It shells out to the git binary and parses the configured remote
// URL; it never makes a network call. Inference is best-effort: every failure
// path (no git binary, not a repo, no usable remote, a non-Bitbucket host)
// returns ok=false rather than an error, so callers can fall back to explicit
// flags.
package git

import (
	"context"
	"fmt"
	"net/url"
	"os/exec"
	"strings"
)

// bitbucketHost is the git remote host that identifies a Bitbucket Cloud
// repository.
const bitbucketHost = "bitbucket.org"

// RemoteTarget is a workspace/repo parsed from a git remote URL.
type RemoteTarget struct {
	Host      string
	Workspace string
	Repo      string
}

// runner runs a git subcommand in dir and returns its trimmed stdout. It is a
// package variable so tests can stub git invocation without a real repository.
var runner = func(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// InferBitbucketRepo resolves dir's git remote into a Bitbucket workspace/repo.
// It returns ok=false (and no error) whenever inference is not possible — no
// git binary, dir is not a repository, no usable remote, an unparseable remote
// URL, or a remote whose host is not Bitbucket Cloud.
func InferBitbucketRepo(ctx context.Context, dir string) (RemoteTarget, bool) {
	remote, ok := detectRemoteName(ctx, dir)
	if !ok {
		return RemoteTarget{}, false
	}
	cloneURL, err := runner(ctx, dir, "remote", "get-url", remote)
	if err != nil || cloneURL == "" {
		return RemoteTarget{}, false
	}
	parsed, err := ParseRemoteURL(cloneURL)
	if err != nil || parsed.Host != bitbucketHost {
		return RemoteTarget{}, false
	}
	return parsed, true
}

// detectRemoteName picks the remote to infer from: the upstream of the current
// branch when set, otherwise "origin", otherwise the first configured remote.
func detectRemoteName(ctx context.Context, dir string) (string, bool) {
	if branch, err := runner(ctx, dir, "branch", "--show-current"); err == nil && branch != "" {
		if remote, err := runner(ctx, dir, "config", "--get", "branch."+branch+".remote"); err == nil && remote != "" {
			return remote, true
		}
	}
	remotes, err := runner(ctx, dir, "remote")
	if err != nil {
		return "", false
	}
	names := strings.Fields(remotes)
	if len(names) == 0 {
		return "", false
	}
	for _, name := range names {
		if name == "origin" {
			return "origin", true
		}
	}
	return names[0], true
}

// ParseRemoteURL parses an https(s) or scp-style git remote URL into its host,
// workspace, and repository slug.
func ParseRemoteURL(raw string) (RemoteTarget, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return RemoteTarget{}, fmt.Errorf("remote URL is empty")
	}
	if strings.Contains(raw, "://") {
		u, err := url.Parse(raw)
		if err != nil {
			return RemoteTarget{}, fmt.Errorf("parse remote URL %q: %w", raw, err)
		}
		return remoteFromParts(u.Hostname(), u.Path)
	}
	// scp-style: git@host:workspace/repo.git
	if at := strings.LastIndex(raw, "@"); at != -1 {
		if colon := strings.LastIndex(raw, ":"); colon > at {
			return remoteFromParts(raw[at+1:colon], raw[colon+1:])
		}
	}
	return RemoteTarget{}, fmt.Errorf("unsupported remote URL format %q", raw)
}

// remoteFromParts assembles a RemoteTarget from a host and the repository path,
// stripping a leading slash and a trailing ".git".
func remoteFromParts(host, path string) (RemoteTarget, error) {
	host = strings.TrimSpace(host)
	if host == "" {
		return RemoteTarget{}, fmt.Errorf("remote host is empty")
	}
	path = strings.TrimPrefix(path, "/")
	path = strings.TrimSuffix(path, ".git")
	parts := strings.Split(path, "/")
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		return RemoteTarget{}, fmt.Errorf("remote path %q does not look like a Bitbucket repository", path)
	}
	return RemoteTarget{Host: host, Workspace: parts[0], Repo: parts[1]}, nil
}
