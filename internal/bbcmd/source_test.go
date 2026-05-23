package bbcmd

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSrcListsDirectory(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repositories/acme/widgets/src/main/internal" {
			t.Errorf("path = %q", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"values":[` +
			`{"path":"internal/cli","type":"commit_directory"},` +
			`{"path":"internal/main.go","type":"commit_file","size":120}]}`))
	}))
	defer srv.Close()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginBBSite(t, srv.URL)

	out, err := execBB(t, "src", "internal", "--repo", "acme/widgets", "--site", "work", "--ref", "main")
	if err != nil {
		t.Fatalf("src: %v\n%s", err, out)
	}
	for _, want := range []string{"dir", "internal/cli", "file", "120", "internal/main.go"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestSrcResolvesDefaultBranchWhenRefOmitted(t *testing.T) {
	var hitRepo, hitSrc bool
	var srcPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/repositories/acme/widgets":
			hitRepo = true
			_, _ = w.Write([]byte(`{"full_name":"acme/widgets","mainbranch":{"name":"develop"}}`))
		case strings.HasPrefix(r.URL.Path, "/repositories/acme/widgets/src"):
			hitSrc = true
			srcPath = r.URL.Path
			_, _ = w.Write([]byte(`{"values":[]}`))
		default:
			t.Errorf("unexpected path %q", r.URL.Path)
		}
	}))
	defer srv.Close()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginBBSite(t, srv.URL)

	out, err := execBB(t, "src", "--repo", "acme/widgets", "--site", "work")
	if err != nil {
		t.Fatalf("src: %v\n%s", err, out)
	}
	if !hitRepo || !hitSrc {
		t.Fatalf("expected repo + src calls; hitRepo=%v hitSrc=%v", hitRepo, hitSrc)
	}
	if srcPath != "/repositories/acme/widgets/src/develop" {
		t.Errorf("src path = %q, want default branch develop", srcPath)
	}
}

func TestFilePrintsContent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repositories/acme/widgets/src/main/go.mod" {
			t.Errorf("path = %q", r.URL.Path)
		}
		_, _ = w.Write([]byte("module example\n\ngo 1.26\n"))
	}))
	defer srv.Close()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginBBSite(t, srv.URL)

	out, err := execBB(t, "file", "go.mod", "--repo", "acme/widgets", "--site", "work", "--ref", "main")
	if err != nil {
		t.Fatalf("file: %v\n%s", err, out)
	}
	if out != "module example\n\ngo 1.26\n" {
		t.Errorf("file output = %q", out)
	}
}
