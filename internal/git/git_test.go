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
		{"https trailing slash", "https://bitbucket.org/acme/widgets.git/", "bitbucket.org", "acme", "widgets", false},
		{"scp trailing slash", "git@bitbucket.org:acme/widgets.git/", "bitbucket.org", "acme", "widgets", false},
		{"ssh with port", "ssh://git@bitbucket.org:22/acme/widgets.git", "bitbucket.org", "acme", "widgets", false},
		{"altssh", "ssh://git@altssh.bitbucket.org:443/acme/widgets.git", "altssh.bitbucket.org", "acme", "widgets", false},
		{"mixed-case host", "git@Bitbucket.org:acme/widgets.git", "bitbucket.org", "acme", "widgets", false},
		{"only slashes", "https://bitbucket.org/", "", "", "", true},
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

// TestInferBitbucketRepoRemoteSelection covers which remote name inference
// hands to "git remote get-url" for each upstream configuration.
func TestInferBitbucketRepoRemoteSelection(t *testing.T) {
	cases := []struct {
		name string
		// branch is what "branch --show-current" returns.
		branch string
		// upstream is what "config --get branch.<branch>.remote" returns; an
		// empty value makes the stub fail the call, as git does when unset.
		upstream   string
		remotes    string
		wantRemote string
	}{
		{"branch upstream", "feature", "fork", "origin\nfork", "fork"},
		{"local-branch upstream falls back to origin", "feature", ".", "origin\nfork", "origin"},
		{"no upstream falls back to origin", "main", "", "upstream\norigin", "origin"},
		{"no upstream no origin uses first remote", "main", "", "upstream\nfork", "upstream"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var gotRemote string
			withRunner(t, func(_ context.Context, _ string, args ...string) (string, error) {
				switch {
				case len(args) >= 1 && args[0] == "branch":
					return tc.branch, nil
				case len(args) >= 2 && args[0] == "config":
					if tc.upstream == "" {
						return "", context.Canceled
					}
					return tc.upstream, nil
				case len(args) == 1 && args[0] == "remote":
					return tc.remotes, nil
				case len(args) >= 3 && args[0] == "remote" && args[1] == "get-url":
					gotRemote = args[2]
					return "https://bitbucket.org/acme/widgets", nil
				}
				return "", context.Canceled
			})

			got, ok := InferBitbucketRepo(context.Background(), ".")
			if !ok {
				t.Fatal("InferBitbucketRepo did not resolve")
			}
			if gotRemote != tc.wantRemote {
				t.Fatalf("get-url remote = %q, want %q", gotRemote, tc.wantRemote)
			}
			if got.Workspace != "acme" || got.Repo != "widgets" {
				t.Fatalf("inferred = %+v", got)
			}
		})
	}
}

func TestInferBitbucketRepoHosts(t *testing.T) {
	cases := []struct {
		name     string
		cloneURL string
		wantOK   bool
	}{
		{"scp bitbucket", "git@bitbucket.org:acme/widgets.git", true},
		{"altssh", "ssh://git@altssh.bitbucket.org:443/acme/widgets.git", true},
		{"mixed-case host", "git@Bitbucket.org:acme/widgets.git", true},
		{"https trailing slash", "https://bitbucket.org/acme/widgets.git/", true},
		{"github", "git@github.com:acme/widgets.git", false},
		{"lookalike suffix", "https://notbitbucket.org/acme/widgets.git", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			withRunner(t, func(_ context.Context, _ string, args ...string) (string, error) {
				switch {
				case len(args) >= 1 && args[0] == "branch":
					return "", context.Canceled
				case len(args) == 1 && args[0] == "remote":
					return "origin", nil
				case len(args) >= 2 && args[0] == "remote" && args[1] == "get-url":
					return tc.cloneURL, nil
				}
				return "", context.Canceled
			})

			got, ok := InferBitbucketRepo(context.Background(), ".")
			if ok != tc.wantOK {
				t.Fatalf("InferBitbucketRepo(%q) ok = %v, want %v", tc.cloneURL, ok, tc.wantOK)
			}
			if tc.wantOK && (got.Workspace != "acme" || got.Repo != "widgets") {
				t.Fatalf("inferred = %+v", got)
			}
		})
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
