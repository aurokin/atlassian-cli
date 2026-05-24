package bitbucket

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetWorkspace(t *testing.T) {
	srv := serveJSON(t, "/workspaces/acme", `{"slug":"acme","name":"Acme"}`)
	defer srv.Close()

	raw, err := newTestClient(srv).GetWorkspace(context.Background(), "acme")
	if err != nil {
		t.Fatalf("GetWorkspace: %v", err)
	}
	ws, err := Decode[Workspace](raw)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if ws.Slug != "acme" || ws.Name != "Acme" {
		t.Fatalf("ws = %+v", ws)
	}
}

func TestListAndGetProject(t *testing.T) {
	srv := serveJSON(t, "/workspaces/acme/projects/WID", `{"key":"WID","name":"Widgets"}`)
	defer srv.Close()

	raw, err := newTestClient(srv).GetProject(context.Background(), "acme", "WID")
	if err != nil {
		t.Fatalf("GetProject: %v", err)
	}
	p, err := Decode[Project](raw)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if p.Key != "WID" {
		t.Fatalf("project = %+v", p)
	}
}

func TestCreateProjectBody(t *testing.T) {
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

	private := true
	_, err := newTestClient(srv).CreateProject(context.Background(), "acme", "WID",
		CreateProjectOptions{Name: "Widgets", IsPrivate: &private})
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	if gotBody["key"] != "WID" || gotBody["name"] != "Widgets" || gotBody["is_private"] != true {
		t.Fatalf("body = %+v", gotBody)
	}

	// A nil IsPrivate omits the field entirely.
	gotBody = nil
	if _, err := newTestClient(srv).CreateProject(context.Background(), "acme", "WID",
		CreateProjectOptions{Name: "Widgets"}); err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	if _, ok := gotBody["is_private"]; ok {
		t.Fatalf("is_private should be omitted when nil: %+v", gotBody)
	}
}

func TestDeleteProject(t *testing.T) {
	var gotMethod, gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath = r.Method, r.URL.Path
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	if err := newTestClient(srv).DeleteProject(context.Background(), "acme", "WID"); err != nil {
		t.Fatalf("DeleteProject: %v", err)
	}
	if gotMethod != http.MethodDelete || gotPath != "/workspaces/acme/projects/WID" {
		t.Errorf("request = %s %s", gotMethod, gotPath)
	}
}
