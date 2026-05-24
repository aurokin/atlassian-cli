package bbcmd

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestBranchListHuman(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repositories/acme/widgets/refs/branches" {
			t.Errorf("path = %q", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"values":[` +
			`{"name":"main","target":{"hash":"abcdef1234567890"}},` +
			`{"name":"develop","target":{"hash":"0123456789abcdef"}}]}`))
	}))
	defer srv.Close()

	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginBBSite(t, srv.URL)

	out, err := execBB(t, "branch", "list", "--repo", "acme/widgets", "--site", "work")
	if err != nil {
		t.Fatalf("branch list: %v\n%s", err, out)
	}
	for _, want := range []string{"main", "abcdef123456", "develop", "0123456789ab"} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q:\n%s", want, out)
		}
	}
}

func TestBranchViewHumanAndJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repositories/acme/widgets/refs/branches/main" {
			t.Errorf("path = %q", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"name":"main","target":{"hash":"abcdef1234567890"}}`))
	}))
	defer srv.Close()

	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginBBSite(t, srv.URL)

	out, err := execBB(t, "branch", "view", "main", "--repo", "acme/widgets", "--site", "work")
	if err != nil {
		t.Fatalf("branch view: %v\n%s", err, out)
	}
	for _, want := range []string{"main", "abcdef123456"} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q:\n%s", want, out)
		}
	}

	jsonOut, err := execBB(t, "branch", "view", "main", "--repo", "acme/widgets", "--site", "work", "--jq", ".name")
	if err != nil {
		t.Fatalf("branch view --jq: %v\n%s", err, jsonOut)
	}
	if strings.TrimSpace(jsonOut) != `"main"` {
		t.Fatalf("jq output = %q", jsonOut)
	}
}

func TestBranchCreateSendsBody(t *testing.T) {
	var gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want POST", r.Method)
		}
		buf, _ := io.ReadAll(r.Body)
		gotBody = string(buf)
		_, _ = w.Write([]byte(`{"name":"hotfix"}`))
	}))
	defer srv.Close()

	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginBBSite(t, srv.URL)

	out, err := execBB(t, "branch", "create", "--repo", "acme/widgets", "--site", "work",
		"--name", "hotfix", "--target", "abc123")
	if err != nil {
		t.Fatalf("branch create: %v\n%s", err, out)
	}
	if !strings.Contains(out, "created branch hotfix") {
		t.Fatalf("unexpected output:\n%s", out)
	}
	if !strings.Contains(gotBody, `"name":"hotfix"`) || !strings.Contains(gotBody, `"hash":"abc123"`) {
		t.Fatalf("request body = %q", gotBody)
	}
}

func TestBranchCreateRequiresNameAndTarget(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	_, err := execBB(t, "branch", "create", "--repo", "acme/widgets", "--site", "work", "--target", "abc123")
	if err == nil || !strings.Contains(err.Error(), "a branch name is required") {
		t.Fatalf("expected name-required error, got %v", err)
	}
	_, err = execBB(t, "branch", "create", "--repo", "acme/widgets", "--site", "work", "--name", "hotfix")
	if err == nil || !strings.Contains(err.Error(), "a branch target is required") {
		t.Fatalf("expected target-required error, got %v", err)
	}
}

func TestBranchDelete(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("method = %q, want DELETE", r.Method)
		}
		if r.URL.Path != "/repositories/acme/widgets/refs/branches/stale" {
			t.Errorf("path = %q", r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginBBSite(t, srv.URL)

	out, err := execBB(t, "branch", "delete", "stale", "--repo", "acme/widgets", "--site", "work")
	if err != nil {
		t.Fatalf("branch delete: %v\n%s", err, out)
	}
	if !strings.Contains(out, "deleted branch stale") {
		t.Fatalf("unexpected output:\n%s", out)
	}
}

func TestBranchDeleteJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginBBSite(t, srv.URL)

	out, err := execBB(t, "branch", "delete", "stale", "--repo", "acme/widgets", "--site", "work", "--json")
	if err != nil {
		t.Fatalf("branch delete --json: %v\n%s", err, out)
	}
	for _, want := range []string{`"resource"`, `"branch"`, `"id"`, `"stale"`, `"deleted"`, `true`} {
		if !strings.Contains(out, want) {
			t.Fatalf("JSON output missing %q:\n%s", want, out)
		}
	}
}
