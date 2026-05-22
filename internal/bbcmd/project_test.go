package bbcmd

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestProjectListHuman(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/workspaces/acme/projects" {
			t.Errorf("path = %q", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"values":[` +
			`{"key":"WID","name":"Widgets"},` +
			`{"key":"GAD","name":"Gadgets"}]}`))
	}))
	defer srv.Close()

	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginBBSite(t, srv.URL)

	out, err := execBB(t, "project", "list", "acme", "--site", "work")
	if err != nil {
		t.Fatalf("project list: %v\n%s", err, out)
	}
	for _, want := range []string{"WID", "Widgets", "GAD", "Gadgets"} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q:\n%s", want, out)
		}
	}
}

func TestProjectViewHumanAndJSON(t *testing.T) {
	body := `{"key":"WID","name":"Widgets","description":"the widget project","is_private":true,"uuid":"{p-1}"}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/workspaces/acme/projects/WID" {
			t.Errorf("path = %q", r.URL.Path)
		}
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()

	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginBBSite(t, srv.URL)

	out, err := execBB(t, "project", "view", "WID", "--workspace", "acme", "--site", "work")
	if err != nil {
		t.Fatalf("project view: %v\n%s", err, out)
	}
	for _, want := range []string{"WID", "Widgets", "private", "the widget project", "{p-1}"} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q:\n%s", want, out)
		}
	}
}

func TestProjectViewRequiresWorkspace(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	_, err := execBB(t, "project", "view", "WID", "--site", "work")
	if err == nil || !strings.Contains(err.Error(), "a workspace is required") {
		t.Fatalf("expected workspace-required error, got %v", err)
	}
}

func TestProjectCreateSendsBodyAndOmitsUnsetPrivate(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want POST", r.Method)
		}
		if r.URL.Path != "/workspaces/acme/projects" {
			t.Errorf("path = %q", r.URL.Path)
		}
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &gotBody)
		_, _ = w.Write([]byte(`{"key":"WID","name":"Widgets"}`))
	}))
	defer srv.Close()

	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginBBSite(t, srv.URL)

	out, err := execBB(t, "project", "create", "WID", "--workspace", "acme", "--site", "work",
		"--name", "Widgets", "--description", "the widgets")
	if err != nil {
		t.Fatalf("project create: %v\n%s", err, out)
	}
	if !strings.Contains(out, "created project WID: Widgets") {
		t.Fatalf("unexpected output:\n%s", out)
	}
	if gotBody["key"] != "WID" || gotBody["name"] != "Widgets" || gotBody["description"] != "the widgets" {
		t.Fatalf("body = %+v", gotBody)
	}
	// --private was not passed, so is_private must be omitted (Bitbucket default).
	if _, ok := gotBody["is_private"]; ok {
		t.Fatalf("is_private should be omitted when --private unset: %+v", gotBody)
	}
}

func TestProjectCreateForwardsPrivateWhenSet(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &gotBody)
		_, _ = w.Write([]byte(`{"key":"WID","name":"Widgets"}`))
	}))
	defer srv.Close()

	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginBBSite(t, srv.URL)

	if _, err := execBB(t, "project", "create", "WID", "--workspace", "acme", "--site", "work",
		"--name", "Widgets", "--private"); err != nil {
		t.Fatalf("project create: %v", err)
	}
	if gotBody["is_private"] != true {
		t.Fatalf("is_private = %v, want true", gotBody["is_private"])
	}
}

func TestProjectCreateRequiresName(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	_, err := execBB(t, "project", "create", "WID", "--workspace", "acme", "--site", "work")
	if err == nil || !strings.Contains(err.Error(), "a name is required") {
		t.Fatalf("expected name-required error, got %v", err)
	}
}
