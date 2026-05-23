package bbcmd

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWorkspaceViewHumanAndJSON(t *testing.T) {
	body := `{"slug":"acme","name":"Acme Inc","is_private":true,"uuid":"{w-1}","created_on":"2020-01-01T00:00:00Z"}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/workspaces/acme" {
			t.Errorf("path = %q", r.URL.Path)
		}
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()

	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginBBSite(t, srv.URL)

	out, err := execBB(t, "workspace", "view", "acme", "--site", "work")
	if err != nil {
		t.Fatalf("workspace view: %v\n%s", err, out)
	}
	for _, want := range []string{"acme", "Acme Inc", "private", "{w-1}"} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q:\n%s", want, out)
		}
	}

	// Via --workspace flag and raw JSON.
	jsonOut, err := execBB(t, "workspace", "view", "--workspace", "acme", "--site", "work", "--jq", ".slug")
	if err != nil {
		t.Fatalf("workspace view --jq: %v\n%s", err, jsonOut)
	}
	if strings.TrimSpace(jsonOut) != `"acme"` {
		t.Fatalf("jq output = %q", jsonOut)
	}
}

func TestWorkspaceViewRequiresWorkspace(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	_, err := execBB(t, "workspace", "view", "--site", "work")
	if err == nil || !strings.Contains(err.Error(), "a workspace is required") {
		t.Fatalf("expected workspace-required error, got %v", err)
	}
}
