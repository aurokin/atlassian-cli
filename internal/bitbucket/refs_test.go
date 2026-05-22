package bitbucket

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestListBranches(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repositories/acme/widgets/refs/branches" {
			t.Errorf("path = %q", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"values":[{"name":"main","target":{"hash":"abcdef1234567890"}}]}`))
	}))
	defer srv.Close()

	raw, err := newTestClient(srv).ListBranches(context.Background(), "acme", "widgets", 0)
	if err != nil {
		t.Fatalf("ListBranches: %v", err)
	}
	page, err := Decode[BranchPage](raw)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if len(page.Values) != 1 || page.Values[0].Name != "main" {
		t.Fatalf("values = %+v", page.Values)
	}
	if page.Values[0].Target == nil || page.Values[0].Target.Hash != "abcdef1234567890" {
		t.Fatalf("target = %+v", page.Values[0].Target)
	}
}

func TestGetBranch(t *testing.T) {
	// A branch name with a slash must be percent-escaped so it stays a single
	// path segment; assert on the raw escaped path the server received.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.EscapedPath(); got != "/repositories/acme/widgets/refs/branches/feature%2Fx" {
			t.Errorf("escaped path = %q", got)
		}
		_, _ = w.Write([]byte(`{"name":"feature/x","target":{"hash":"deadbeef"}}`))
	}))
	defer srv.Close()

	raw, err := newTestClient(srv).GetBranch(context.Background(), "acme", "widgets", "feature/x")
	if err != nil {
		t.Fatalf("GetBranch: %v", err)
	}
	branch, err := Decode[Branch](raw)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if branch.Name != "feature/x" {
		t.Fatalf("branch = %+v", branch)
	}
}

func TestCreateBranchSendsTargetHash(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want POST", r.Method)
		}
		if r.URL.Path != "/repositories/acme/widgets/refs/branches" {
			t.Errorf("path = %q", r.URL.Path)
		}
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &gotBody)
		_, _ = w.Write([]byte(`{"name":"hotfix","target":{"hash":"abc123"}}`))
	}))
	defer srv.Close()

	_, err := newTestClient(srv).CreateBranch(context.Background(), "acme", "widgets",
		CreateBranchOptions{Name: "hotfix", Target: "abc123"})
	if err != nil {
		t.Fatalf("CreateBranch: %v", err)
	}
	if gotBody["name"] != "hotfix" {
		t.Fatalf("body name = %v", gotBody["name"])
	}
	target, ok := gotBody["target"].(map[string]any)
	if !ok || target["hash"] != "abc123" {
		t.Fatalf("body target = %v", gotBody["target"])
	}
}

func TestDeleteBranch(t *testing.T) {
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

	if err := newTestClient(srv).DeleteBranch(context.Background(), "acme", "widgets", "stale"); err != nil {
		t.Fatalf("DeleteBranch: %v", err)
	}
}

func TestListTags(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repositories/acme/widgets/refs/tags" {
			t.Errorf("path = %q", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"values":[{"name":"v1.0.0","target":{"hash":"abcdef1234567890"}}]}`))
	}))
	defer srv.Close()

	raw, err := newTestClient(srv).ListTags(context.Background(), "acme", "widgets", 0)
	if err != nil {
		t.Fatalf("ListTags: %v", err)
	}
	page, err := Decode[TagPage](raw)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if len(page.Values) != 1 || page.Values[0].Name != "v1.0.0" {
		t.Fatalf("values = %+v", page.Values)
	}
}

func TestGetTag(t *testing.T) {
	srv := serveJSON(t, "/repositories/acme/widgets/refs/tags/v1.0.0",
		`{"name":"v1.0.0","message":"release","target":{"hash":"deadbeef"}}`)
	defer srv.Close()

	raw, err := newTestClient(srv).GetTag(context.Background(), "acme", "widgets", "v1.0.0")
	if err != nil {
		t.Fatalf("GetTag: %v", err)
	}
	tag, err := Decode[Tag](raw)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if tag.Name != "v1.0.0" || tag.Message != "release" {
		t.Fatalf("tag = %+v", tag)
	}
}

func TestCreateTagSendsTargetAndMessage(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repositories/acme/widgets/refs/tags" {
			t.Errorf("path = %q", r.URL.Path)
		}
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &gotBody)
		_, _ = w.Write([]byte(`{"name":"v2.0.0"}`))
	}))
	defer srv.Close()

	_, err := newTestClient(srv).CreateTag(context.Background(), "acme", "widgets",
		CreateTagOptions{Name: "v2.0.0", Target: "abc123", Message: "ship it"})
	if err != nil {
		t.Fatalf("CreateTag: %v", err)
	}
	if gotBody["name"] != "v2.0.0" || gotBody["message"] != "ship it" {
		t.Fatalf("body = %+v", gotBody)
	}
	target, ok := gotBody["target"].(map[string]any)
	if !ok || target["hash"] != "abc123" {
		t.Fatalf("body target = %v", gotBody["target"])
	}
}

func TestCreateTagOmitsEmptyMessage(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &gotBody)
		_, _ = w.Write([]byte(`{"name":"v2.0.0"}`))
	}))
	defer srv.Close()

	if _, err := newTestClient(srv).CreateTag(context.Background(), "acme", "widgets",
		CreateTagOptions{Name: "v2.0.0", Target: "abc123"}); err != nil {
		t.Fatalf("CreateTag: %v", err)
	}
	if _, ok := gotBody["message"]; ok {
		t.Fatalf("message should be omitted when empty: %+v", gotBody)
	}
}

func TestDeleteTag(t *testing.T) {
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

	if err := newTestClient(srv).DeleteTag(context.Background(), "acme", "widgets", "v0.1.0"); err != nil {
		t.Fatalf("DeleteTag: %v", err)
	}
}
