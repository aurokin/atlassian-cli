package bbcmd

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWorkspaceListHumanAndRole(t *testing.T) {
	var gotRole string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/workspaces" {
			t.Errorf("path = %q", r.URL.Path)
		}
		gotRole = r.URL.Query().Get("role")
		_, _ = w.Write([]byte(`{"values":[` +
			`{"slug":"acme","name":"Acme Inc"},` +
			`{"slug":"beta","name":"Beta Co"}]}`))
	}))
	defer srv.Close()

	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginBBSite(t, srv.URL)

	out, err := execBB(t, "workspace", "list", "--site", "work")
	if err != nil {
		t.Fatalf("workspace list: %v\n%s", err, out)
	}
	if gotRole != "member" {
		t.Fatalf("role query = %q, want member", gotRole)
	}
	for _, want := range []string{"acme", "Acme Inc", "beta", "Beta Co"} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q:\n%s", want, out)
		}
	}
}

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

func TestWorkspaceListAllFollowsPagination(t *testing.T) {
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Query().Get("page") {
		case "", "1":
			_, _ = w.Write([]byte(`{"values":[{"slug":"a"}],"next":"` + srv.URL + `/workspaces?page=2"}`))
		case "2":
			_, _ = w.Write([]byte(`{"values":[{"slug":"b"}]}`))
		default:
			t.Errorf("unexpected page %q", r.URL.Query().Get("page"))
		}
	}))
	defer srv.Close()

	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginBBSite(t, srv.URL)

	out, err := execBB(t, "workspace", "list", "--site", "work", "--all")
	if err != nil {
		t.Fatalf("workspace list --all: %v\n%s", err, out)
	}
	if !strings.Contains(out, "a") || !strings.Contains(out, "b") {
		t.Fatalf("output missing aggregated workspaces:\n%s", out)
	}
}
