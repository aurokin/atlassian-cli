package confcmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPageLabelListHumanOutput(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/pages/10/labels" {
			t.Errorf("path = %q, want /pages/10/labels", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"results":[{"id":"1","name":"needs-review","prefix":"global"},` +
			`{"id":"2","name":"draft","prefix":"global"}]}`))
	}))
	defer srv.Close()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginConfSite(t, srv.URL)

	out, err := execConf(t, "page", "label", "list", "10", "--site", "work")
	if err != nil {
		t.Fatalf("page label list: %v", err)
	}
	for _, want := range []string{"needs-review", "draft", "global"} {
		if !strings.Contains(out, want) {
			t.Errorf("label list output missing %q:\n%s", want, out)
		}
	}
}

func TestPageLabelListJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"results":[{"id":"1","name":"needs-review"}]}`))
	}))
	defer srv.Close()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginConfSite(t, srv.URL)

	out, err := execConf(t, "page", "label", "list", "10", "--site", "work", "--json")
	if err != nil {
		t.Fatalf("page label list --json: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("label list --json output is not valid JSON: %v\n%s", err, out)
	}
	if _, ok := got["results"]; !ok {
		t.Fatalf("unexpected label list JSON: %v", got)
	}
}

func TestPageLabelListAllFollowsPages(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/pages/10/labels" {
			t.Errorf("path = %q, want /pages/10/labels", r.URL.Path)
		}
		switch r.URL.Query().Get("cursor") {
		case "":
			_, _ = w.Write([]byte(`{"results":[{"id":"1","name":"a"}],` +
				`"_links":{"next":"/pages/10/labels?cursor=c2"}}`))
		case "c2":
			_, _ = w.Write([]byte(`{"results":[{"id":"2","name":"b"}]}`))
		default:
			t.Errorf("unexpected cursor %q", r.URL.Query().Get("cursor"))
		}
	}))
	defer srv.Close()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginConfSite(t, srv.URL)

	out, err := execConf(t, "page", "label", "list", "10", "--all", "--site", "work", "--json")
	if err != nil {
		t.Fatalf("page label list --all: %v", err)
	}
	assertResultCount(t, "page label list --all", out, 2)
}

func TestPageLabelAddHumanOutput(t *testing.T) {
	var gotBody []map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/api/content/10/label" {
			t.Errorf("path = %q, want /rest/api/content/10/label", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want POST", r.Method)
		}
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_, _ = w.Write([]byte(`{"results":[{"id":"1","name":"needs-review","prefix":"global"}]}`))
	}))
	defer srv.Close()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginConfSite(t, srv.URL)

	out, err := execConf(t, "page", "label", "add", "10", "needs-review", "--site", "work")
	if err != nil {
		t.Fatalf("page label add: %v", err)
	}
	if len(gotBody) != 1 || gotBody[0]["name"] != "needs-review" {
		t.Errorf("add sent body %v, want [{name: needs-review}]", gotBody)
	}
	if !strings.Contains(out, "added label needs-review to page 10") {
		t.Errorf("add output missing 'added label needs-review to page 10':\n%s", out)
	}
}

func TestPageLabelRemoveHumanOutput(t *testing.T) {
	var gotMethod string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		if r.URL.Path != "/rest/api/content/10/label/needs-review" {
			t.Errorf("path = %q, want /rest/api/content/10/label/needs-review", r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginConfSite(t, srv.URL)

	out, err := execConf(t, "page", "label", "remove", "10", "needs-review", "--site", "work")
	if err != nil {
		t.Fatalf("page label remove: %v", err)
	}
	if gotMethod != http.MethodDelete {
		t.Errorf("remove used method %q, want DELETE", gotMethod)
	}
	if !strings.Contains(out, "removed label needs-review from page 10") {
		t.Errorf("remove output missing 'removed label needs-review from page 10':\n%s", out)
	}
}
