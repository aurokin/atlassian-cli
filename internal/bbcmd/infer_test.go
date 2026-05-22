package bbcmd

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestRepoTargetInferenceDrivesRequest confirms that when no target is supplied
// a command falls back to git-checkout inference and acts on the inferred
// workspace/repo.
func TestRepoTargetInferenceDrivesRequest(t *testing.T) {
	stubInfer(t, "acme", "widgets")

	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = w.Write([]byte(`{"hash":"abc123","message":"hi"}`))
	}))
	defer srv.Close()

	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginBBSite(t, srv.URL)

	// No --repo / --workspace: the target must come from inference.
	out, err := execBB(t, "commit", "view", "abc123", "--site", "work")
	if err != nil {
		t.Fatalf("commit view (inferred): %v\n%s", err, out)
	}
	if gotPath != "/repositories/acme/widgets/commit/abc123" {
		t.Fatalf("request path = %q, want inferred acme/widgets", gotPath)
	}
}

// TestExplicitWorkspaceSkipsInference confirms that passing --workspace without
// a repository does not silently fall back to inference; the missing repository
// is reported instead.
func TestExplicitWorkspaceSkipsInference(t *testing.T) {
	stubInfer(t, "acme", "widgets")
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	_, err := execBB(t, "commit", "view", "abc123", "--workspace", "other", "--site", "work")
	if err == nil || !strings.Contains(err.Error(), "a repository is required") {
		t.Fatalf("expected repo-required error with explicit --workspace, got %v", err)
	}
}
