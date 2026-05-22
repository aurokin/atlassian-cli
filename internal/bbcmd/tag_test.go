package bbcmd

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTagListHuman(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repositories/acme/widgets/refs/tags" {
			t.Errorf("path = %q", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"values":[` +
			`{"name":"v1.0.0","target":{"hash":"abcdef1234567890"}},` +
			`{"name":"v0.9.0","target":{"hash":"0123456789abcdef"}}]}`))
	}))
	defer srv.Close()

	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginBBSite(t, srv.URL)

	out, err := execBB(t, "tag", "list", "--repo", "acme/widgets", "--site", "work")
	if err != nil {
		t.Fatalf("tag list: %v\n%s", err, out)
	}
	for _, want := range []string{"v1.0.0", "abcdef123456", "v0.9.0"} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q:\n%s", want, out)
		}
	}
}

func TestTagViewHumanAndJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repositories/acme/widgets/refs/tags/v1.0.0" {
			t.Errorf("path = %q", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"name":"v1.0.0","message":"first release","date":"2026-01-01T00:00:00Z","target":{"hash":"abcdef1234567890"}}`))
	}))
	defer srv.Close()

	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginBBSite(t, srv.URL)

	out, err := execBB(t, "tag", "view", "v1.0.0", "--repo", "acme/widgets", "--site", "work")
	if err != nil {
		t.Fatalf("tag view: %v\n%s", err, out)
	}
	for _, want := range []string{"v1.0.0", "first release", "abcdef123456"} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q:\n%s", want, out)
		}
	}

	jsonOut, err := execBB(t, "tag", "view", "v1.0.0", "--repo", "acme/widgets", "--site", "work", "--jq", ".name")
	if err != nil {
		t.Fatalf("tag view --jq: %v\n%s", err, jsonOut)
	}
	if strings.TrimSpace(jsonOut) != `"v1.0.0"` {
		t.Fatalf("jq output = %q", jsonOut)
	}
}

func TestTagCreateSendsBody(t *testing.T) {
	var gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want POST", r.Method)
		}
		buf, _ := io.ReadAll(r.Body)
		gotBody = string(buf)
		_, _ = w.Write([]byte(`{"name":"v2.0.0"}`))
	}))
	defer srv.Close()

	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginBBSite(t, srv.URL)

	out, err := execBB(t, "tag", "create", "--repo", "acme/widgets", "--site", "work",
		"--name", "v2.0.0", "--target", "abc123", "--message", "ship it")
	if err != nil {
		t.Fatalf("tag create: %v\n%s", err, out)
	}
	if !strings.Contains(out, "created tag v2.0.0") {
		t.Fatalf("unexpected output:\n%s", out)
	}
	for _, want := range []string{`"name":"v2.0.0"`, `"hash":"abc123"`, `"message":"ship it"`} {
		if !strings.Contains(gotBody, want) {
			t.Fatalf("request body missing %q: %s", want, gotBody)
		}
	}
}

func TestTagCreateRequiresNameAndTarget(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	_, err := execBB(t, "tag", "create", "--repo", "acme/widgets", "--site", "work", "--target", "abc123")
	if err == nil || !strings.Contains(err.Error(), "a tag name is required") {
		t.Fatalf("expected name-required error, got %v", err)
	}
	_, err = execBB(t, "tag", "create", "--repo", "acme/widgets", "--site", "work", "--name", "v2.0.0")
	if err == nil || !strings.Contains(err.Error(), "a tag target is required") {
		t.Fatalf("expected target-required error, got %v", err)
	}
}

func TestTagDelete(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("method = %q, want DELETE", r.Method)
		}
		if r.URL.Path != "/repositories/acme/widgets/refs/tags/v0.1.0" {
			t.Errorf("path = %q", r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginBBSite(t, srv.URL)

	out, err := execBB(t, "tag", "delete", "v0.1.0", "--repo", "acme/widgets", "--site", "work")
	if err != nil {
		t.Fatalf("tag delete: %v\n%s", err, out)
	}
	if !strings.Contains(out, "deleted tag v0.1.0") {
		t.Fatalf("unexpected output:\n%s", out)
	}
}
