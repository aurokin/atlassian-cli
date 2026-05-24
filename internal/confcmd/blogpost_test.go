package confcmd

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestBlogpostListHuman(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/blogposts" {
			t.Errorf("path = %q", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"results":[{"id":"5","status":"current","title":"Hello World"}]}`))
	}))
	defer srv.Close()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginConfSite(t, srv.URL)

	out, err := execConf(t, "blogpost", "list", "--site", "work")
	if err != nil {
		t.Fatalf("blogpost list: %v\n%s", err, out)
	}
	for _, want := range []string{"5", "current", "Hello World"} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q:\n%s", want, out)
		}
	}
}

func TestBlogpostListResolvesSpace(t *testing.T) {
	var gotSpace string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/spaces":
			_, _ = w.Write([]byte(`{"results":[{"id":"789","key":"DEV","name":"Dev"}]}`))
		case "/blogposts":
			gotSpace = r.URL.Query().Get("space-id")
			_, _ = w.Write([]byte(`{"results":[]}`))
		default:
			t.Errorf("unexpected path %q", r.URL.Path)
		}
	}))
	defer srv.Close()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginConfSite(t, srv.URL)

	if _, err := execConf(t, "blogpost", "list", "--space", "DEV", "--site", "work"); err != nil {
		t.Fatalf("blogpost list --space: %v", err)
	}
	if gotSpace != "789" {
		t.Errorf("space-id = %q, want resolved 789", gotSpace)
	}
}

func TestBlogpostViewHuman(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/blogposts/5" {
			t.Errorf("path = %q", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"id":"5","title":"Hello","status":"current","spaceId":"789","version":{"number":2}}`))
	}))
	defer srv.Close()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginConfSite(t, srv.URL)

	out, err := execConf(t, "blogpost", "view", "5", "--site", "work")
	if err != nil {
		t.Fatalf("blogpost view: %v\n%s", err, out)
	}
	for _, want := range []string{"Hello", "current", "789"} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q:\n%s", want, out)
		}
	}
}

func TestBlogpostCreateSendsBody(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/spaces":
			_, _ = w.Write([]byte(`{"results":[{"id":"789","key":"DEV","name":"Dev"}]}`))
		case "/blogposts":
			raw, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(raw, &gotBody)
			_, _ = w.Write([]byte(`{"id":"5","title":"Hello"}`))
		default:
			t.Errorf("unexpected path %q", r.URL.Path)
		}
	}))
	defer srv.Close()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginConfSite(t, srv.URL)

	out, err := execConf(t, "blogpost", "create", "--space", "DEV", "--title", "Hello",
		"--body", "<p>hi</p>", "--body-format", "storage", "--site", "work")
	if err != nil {
		t.Fatalf("blogpost create: %v\n%s", err, out)
	}
	if gotBody["spaceId"] != "789" || gotBody["title"] != "Hello" {
		t.Fatalf("body = %+v", gotBody)
	}
	if !strings.Contains(out, "created blogpost 5") {
		t.Fatalf("unexpected output:\n%s", out)
	}
}

func TestBlogpostCreateRequiresFlags(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	_, err := execConf(t, "blogpost", "create", "--space", "DEV", "--site", "work")
	if err == nil || !strings.Contains(err.Error(), "blogpost create requires") {
		t.Fatalf("expected required-flags error, got %v", err)
	}
}

func TestBlogpostEditTitleOnlyPreservesBody(t *testing.T) {
	var put map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/blogposts/5" {
			t.Errorf("path = %q", r.URL.Path)
		}
		switch r.Method {
		case http.MethodGet:
			_, _ = w.Write([]byte(`{"id":"5","title":"Old","status":"current","version":{"number":2},` +
				`"body":{"storage":{"representation":"storage","value":"<p>keep</p>"}}}`))
		case http.MethodPut:
			raw, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(raw, &put)
			_, _ = w.Write([]byte(`{"id":"5","version":{"number":3}}`))
		}
	}))
	defer srv.Close()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginConfSite(t, srv.URL)

	out, err := execConf(t, "blogpost", "edit", "5", "--title", "New", "--site", "work")
	if err != nil {
		t.Fatalf("blogpost edit: %v\n%s", err, out)
	}
	if put["title"] != "New" {
		t.Fatalf("PUT title = %v, want New", put["title"])
	}
	body, _ := put["body"].(map[string]any)
	if body["value"] != "<p>keep</p>" || body["representation"] != "storage" {
		t.Fatalf("title-only edit did not preserve body: %+v", put["body"])
	}
	if !strings.Contains(out, "updated blogpost 5 to version 3") {
		t.Fatalf("unexpected output:\n%s", out)
	}
}

func TestBlogpostEditRequiresAChange(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	_, err := execConf(t, "blogpost", "edit", "5", "--site", "work")
	if err == nil || !strings.Contains(err.Error(), "blogpost edit requires at least one change") {
		t.Fatalf("expected no-change error, got %v", err)
	}
}
