package git

import (
	"context"
	"testing"
)

func TestParseRemoteURL(t *testing.T) {
	cases := []struct {
		name      string
		raw       string
		wantHost  string
		wantWS    string
		wantRepo  string
		wantError bool
	}{
		{"https", "https://bitbucket.org/acme/widgets.git", "bitbucket.org", "acme", "widgets", false},
		{"https no .git", "https://bitbucket.org/acme/widgets", "bitbucket.org", "acme", "widgets", false},
		{"https with user", "https://x-token-auth@bitbucket.org/acme/widgets.git", "bitbucket.org", "acme", "widgets", false},
		{"scp", "git@bitbucket.org:acme/widgets.git", "bitbucket.org", "acme", "widgets", false},
		{"github https", "https://github.com/acme/widgets.git", "github.com", "acme", "widgets", false},
		{"empty", "", "", "", "", true},
		{"no path", "https://bitbucket.org/acme", "", "", "", true},
		{"garbage", "not-a-url", "", "", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseRemoteURL(tc.raw)
			if tc.wantError {
				if err == nil {
					t.Fatalf("ParseRemoteURL(%q) = %+v, want error", tc.raw, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseRemoteURL(%q): %v", tc.raw, err)
			}
			if got.Host != tc.wantHost || got.Workspace != tc.wantWS || got.Repo != tc.wantRepo {
				t.Fatalf("ParseRemoteURL(%q) = %+v", tc.raw, got)
			}
		})
	}
}

// withRunner swaps the package git runner for the duration of a test.
func withRunner(t *testing.T, fn func(ctx context.Context, dir string, args ...string) (string, error)) {
	t.Helper()
	orig := runner
	runner = fn
	t.Cleanup(func() { runner = orig })
}

func TestInferBitbucketRepoFromOrigin(t *testing.T) {
	withRunner(t, func(_ context.Context, _ string, args ...string) (string, error) {
		switch {
		case len(args) >= 1 && args[0] == "branch":
			return "main", nil
		case len(args) >= 2 && args[0] == "config":
			return "", context.Canceled // no configured upstream → fall through to remotes
		case len(args) == 1 && args[0] == "remote":
			return "origin\nupstream", nil
		case len(args) >= 2 && args[0] == "remote" && args[1] == "get-url":
			return "git@bitbucket.org:acme/widgets.git", nil
		}
		return "", context.Canceled
	})

	got, ok := InferBitbucketRepo(context.Background(), ".")
	if !ok {
		t.Fatal("InferBitbucketRepo did not resolve")
	}
	if got.Workspace != "acme" || got.Repo != "widgets" {
		t.Fatalf("inferred = %+v", got)
	}
}

func TestInferBitbucketRepoUsesBranchUpstream(t *testing.T) {
	var gotRemote string
	withRunner(t, func(_ context.Context, _ string, args ...string) (string, error) {
		switch {
		case len(args) >= 1 && args[0] == "branch":
			return "feature", nil
		case len(args) >= 2 && args[0] == "config":
			return "fork", nil
		case len(args) >= 2 && args[0] == "remote" && args[1] == "get-url":
			gotRemote = args[2]
			return "https://bitbucket.org/acme/widgets", nil
		}
		return "", context.Canceled
	})

	got, ok := InferBitbucketRepo(context.Background(), ".")
	if !ok {
		t.Fatal("InferBitbucketRepo did not resolve")
	}
	if gotRemote != "fork" {
		t.Fatalf("get-url remote = %q, want fork", gotRemote)
	}
	if got.Workspace != "acme" || got.Repo != "widgets" {
		t.Fatalf("inferred = %+v", got)
	}
}

func TestInferBitbucketRepoRejectsNonBitbucket(t *testing.T) {
	withRunner(t, func(_ context.Context, _ string, args ...string) (string, error) {
		switch {
		case len(args) >= 1 && args[0] == "branch":
			return "", context.Canceled
		case len(args) == 1 && args[0] == "remote":
			return "origin", nil
		case len(args) >= 2 && args[0] == "remote" && args[1] == "get-url":
			return "git@github.com:acme/widgets.git", nil
		}
		return "", context.Canceled
	})

	if _, ok := InferBitbucketRepo(context.Background(), "."); ok {
		t.Fatal("InferBitbucketRepo should reject a non-Bitbucket remote")
	}
}

func TestInferBitbucketRepoNoRepo(t *testing.T) {
	withRunner(t, func(_ context.Context, _ string, _ ...string) (string, error) {
		return "", context.Canceled // every git call fails → not a repo
	})

	if _, ok := InferBitbucketRepo(context.Background(), "."); ok {
		t.Fatal("InferBitbucketRepo should return false when git fails")
	}
}
