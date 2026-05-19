package confcmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// The --all flag follows pagination to completion; each test serves two pages
// via the Confluence _links.next cursor and asserts the command emitted the
// aggregated result.

func TestSpaceListAllFollowsPages(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Query().Get("cursor") {
		case "":
			_, _ = w.Write([]byte(`{"results":[{"id":"1","key":"DEV"}],` +
				`"_links":{"next":"/spaces?cursor=c2"}}`))
		case "c2":
			_, _ = w.Write([]byte(`{"results":[{"id":"2","key":"OPS"}]}`))
		default:
			t.Errorf("unexpected cursor %q", r.URL.Query().Get("cursor"))
		}
	}))
	defer srv.Close()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginConfSite(t, srv.URL)

	out, err := execConf(t, "space", "list", "--all", "--site", "work", "--json")
	if err != nil {
		t.Fatalf("space list --all: %v", err)
	}
	assertResultCount(t, "space list --all", out, 2)
}

func TestPageListAllFollowsPages(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/spaces":
			_, _ = w.Write([]byte(`{"results":[{"id":"1","key":"DEV"}]}`))
		case "/pages":
			switch r.URL.Query().Get("cursor") {
			case "":
				_, _ = w.Write([]byte(`{"results":[{"id":"10"}],"_links":{"next":"/pages?cursor=c2"}}`))
			case "c2":
				_, _ = w.Write([]byte(`{"results":[{"id":"11"}]}`))
			default:
				t.Errorf("unexpected cursor %q", r.URL.Query().Get("cursor"))
			}
		default:
			t.Errorf("unexpected path %q", r.URL.Path)
		}
	}))
	defer srv.Close()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginConfSite(t, srv.URL)

	out, err := execConf(t, "page", "list", "--space", "DEV", "--all", "--site", "work", "--json")
	if err != nil {
		t.Fatalf("page list --all: %v", err)
	}
	assertResultCount(t, "page list --all", out, 2)
}

func TestPageChildrenAllFollowsPages(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/pages/10/children" {
			t.Errorf("path = %q, want /pages/10/children", r.URL.Path)
		}
		switch r.URL.Query().Get("cursor") {
		case "":
			_, _ = w.Write([]byte(`{"results":[{"id":"11"}],` +
				`"_links":{"next":"/pages/10/children?cursor=c2"}}`))
		case "c2":
			_, _ = w.Write([]byte(`{"results":[{"id":"12"}]}`))
		default:
			t.Errorf("unexpected cursor %q", r.URL.Query().Get("cursor"))
		}
	}))
	defer srv.Close()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginConfSite(t, srv.URL)

	out, err := execConf(t, "page", "children", "10", "--all", "--site", "work", "--json")
	if err != nil {
		t.Fatalf("page children --all: %v", err)
	}
	assertResultCount(t, "page children --all", out, 2)
}

func TestPageCommentListAllFollowsPages(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/pages/10/footer-comments" {
			t.Errorf("path = %q, want /pages/10/footer-comments", r.URL.Path)
		}
		switch r.URL.Query().Get("cursor") {
		case "":
			_, _ = w.Write([]byte(`{"results":[{"id":"c1"}],` +
				`"_links":{"next":"/pages/10/footer-comments?cursor=c2"}}`))
		case "c2":
			_, _ = w.Write([]byte(`{"results":[{"id":"c2"}]}`))
		default:
			t.Errorf("unexpected cursor %q", r.URL.Query().Get("cursor"))
		}
	}))
	defer srv.Close()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginConfSite(t, srv.URL)

	out, err := execConf(t, "page", "comment", "list", "10", "--all", "--site", "work", "--json")
	if err != nil {
		t.Fatalf("page comment list --all: %v", err)
	}
	assertResultCount(t, "page comment list --all", out, 2)
}

func TestAttachmentListAllFollowsPages(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/pages/10/attachments" {
			t.Errorf("path = %q, want /pages/10/attachments", r.URL.Path)
		}
		switch r.URL.Query().Get("cursor") {
		case "":
			_, _ = w.Write([]byte(`{"results":[{"id":"a1"}],` +
				`"_links":{"next":"/pages/10/attachments?cursor=c2"}}`))
		case "c2":
			_, _ = w.Write([]byte(`{"results":[{"id":"a2"}]}`))
		default:
			t.Errorf("unexpected cursor %q", r.URL.Query().Get("cursor"))
		}
	}))
	defer srv.Close()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginConfSite(t, srv.URL)

	out, err := execConf(t, "attachment", "list", "10", "--all", "--site", "work", "--json")
	if err != nil {
		t.Fatalf("attachment list --all: %v", err)
	}
	assertResultCount(t, "attachment list --all", out, 2)
}

func TestSearchCQLAllFollowsPages(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Query().Get("cursor") {
		case "":
			_, _ = w.Write([]byte(`{"results":[{"content":{"id":"1"}}],` +
				`"_links":{"next":"/rest/api/search?cursor=c2"}}`))
		case "c2":
			_, _ = w.Write([]byte(`{"results":[{"content":{"id":"2"}}]}`))
		default:
			t.Errorf("unexpected cursor %q", r.URL.Query().Get("cursor"))
		}
	}))
	defer srv.Close()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginConfSite(t, srv.URL)

	out, err := execConf(t, "search", "cql", "type = page", "--all", "--site", "work", "--json")
	if err != nil {
		t.Fatalf("search cql --all: %v", err)
	}
	assertResultCount(t, "search cql --all", out, 2)
}

// assertResultCount checks that out is a JSON object whose "results" array has
// the expected length.
func assertResultCount(t *testing.T, label, out string, want int) {
	t.Helper()
	var got struct {
		Results []json.RawMessage `json:"results"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("%s output is not valid JSON: %v\n%s", label, err, out)
	}
	if len(got.Results) != want {
		t.Fatalf("%s aggregated %d results, want %d", label, len(got.Results), want)
	}
}
