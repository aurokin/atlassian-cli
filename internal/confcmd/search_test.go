package confcmd

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aurokin/atlassian-cli/internal/apperr"
)

func TestSearchCQLHumanOutput(t *testing.T) {
	var gotCQL, gotLimit string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/api/search" {
			t.Errorf("path = %q, want /rest/api/search", r.URL.Path)
		}
		gotCQL = r.URL.Query().Get("cql")
		gotLimit = r.URL.Query().Get("limit")
		_, _ = w.Write([]byte(`{"results":[{"content":{"id":"10","type":"page","title":"Home"}}]}`))
	}))
	defer srv.Close()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginConfSite(t, srv.URL)

	const cql = "type = page AND space = DEV"
	out, err := execConf(t, "search", "cql", cql, "--limit", "6", "--site", "work")
	if err != nil {
		t.Fatalf("search cql: %v", err)
	}
	if gotCQL != cql {
		t.Errorf("search cql sent cql %q, want %q", gotCQL, cql)
	}
	if gotLimit != "6" {
		t.Errorf("search cql sent limit %q, want 6", gotLimit)
	}
	for _, want := range []string{"10", "page", "Home"} {
		if !strings.Contains(out, want) {
			t.Errorf("search cql output missing %q:\n%s", want, out)
		}
	}
}

func TestSearchCQLEmpty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"results":[]}`))
	}))
	defer srv.Close()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginConfSite(t, srv.URL)

	out, err := execConf(t, "search", "cql", "type = page", "--site", "work")
	if err != nil {
		t.Fatalf("search cql: %v", err)
	}
	if !strings.Contains(out, "No results found") {
		t.Fatalf("empty search output:\n%s", out)
	}
}

func TestSearchCQLJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"results":[{"content":{"id":"10"}}]}`))
	}))
	defer srv.Close()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginConfSite(t, srv.URL)

	out, err := execConf(t, "search", "cql", "type = page", "--site", "work", "--json")
	if err != nil {
		t.Fatalf("search cql --json: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("search cql --json output is not valid JSON: %v\n%s", err, out)
	}
	if _, ok := got["results"]; !ok {
		t.Fatalf("unexpected search JSON: %v", got)
	}
}

func TestSearchCQLMapsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"message":"You do not have permission."}`))
	}))
	defer srv.Close()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginConfSite(t, srv.URL)

	_, err := execConf(t, "search", "cql", "type = page", "--site", "work")
	if err == nil {
		t.Fatal("search cql against a 403 endpoint returned no error")
	}
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Code != apperr.CodeForbidden {
		t.Fatalf("error = %v, want a forbidden *apperr.Error", err)
	}
}

func TestSearchCQLRequiresExactlyOneArg(t *testing.T) {
	if _, err := execConf(t, "search", "cql", "--site", "work"); err == nil {
		t.Fatal("search cql with no query returned no error")
	}
}

func TestSearchCQLJQFiltersOutput(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"results":[{"content":{"title":"Home"}},{"content":{"title":"Docs"}}]}`))
	}))
	defer srv.Close()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginConfSite(t, srv.URL)

	// --jq is plumbed through to the shared renderer; it filters the raw API
	// response and prints each result on its own line.
	out, err := execConf(t, "search", "cql", "type = page", "--site", "work",
		"--jq", ".results[].content.title")
	if err != nil {
		t.Fatalf("search cql --jq: %v", err)
	}
	if out != "\"Home\"\n\"Docs\"\n" {
		t.Fatalf("--jq output = %q, want each title on its own line", out)
	}
}

func TestSearchTextBuildsCQL(t *testing.T) {
	var gotCQL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotCQL = r.URL.Query().Get("cql")
		_, _ = w.Write([]byte(`{"results":[{"content":{"id":"10","type":"page","title":"Home"}}]}`))
	}))
	defer srv.Close()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginConfSite(t, srv.URL)

	out, err := execConf(t, "search", "text", "release notes",
		"--space", "DEV", "--type", "page", "--site", "work")
	if err != nil {
		t.Fatalf("search text: %v\n%s", err, out)
	}
	want := `text ~ "release notes" and type = "page" and space = "DEV"`
	if gotCQL != want {
		t.Fatalf("cql = %q, want %q", gotCQL, want)
	}
	if !strings.Contains(out, "Home") {
		t.Fatalf("output missing result:\n%s", out)
	}
}

func TestSearchTextEscapesQuotes(t *testing.T) {
	var gotCQL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotCQL = r.URL.Query().Get("cql")
		_, _ = w.Write([]byte(`{"results":[]}`))
	}))
	defer srv.Close()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginConfSite(t, srv.URL)

	if _, err := execConf(t, "search", "text", `say "hi"`, "--site", "work"); err != nil {
		t.Fatalf("search text: %v", err)
	}
	want := `text ~ "say \"hi\""`
	if gotCQL != want {
		t.Fatalf("cql = %q, want %q", gotCQL, want)
	}
}

func TestSearchTextBuildCQLNoFlags(t *testing.T) {
	if got := buildTextCQL("hello", "", ""); got != `text ~ "hello"` {
		t.Errorf("buildTextCQL = %q", got)
	}
}

func TestCQLEscapeBackslashAndQuote(t *testing.T) {
	// A trailing backslash must be doubled so it cannot escape the closing
	// quote; quotes are escaped after.
	if got := cqlEscape(`foo\`); got != `foo\\` {
		t.Errorf(`cqlEscape(foo\) = %q, want foo\\`, got)
	}
	if got := cqlEscape(`a"b`); got != `a\"b` {
		t.Errorf(`cqlEscape(a"b) = %q`, got)
	}
	if got := buildTextCQL(`x\`, "", ""); got != `text ~ "x\\"` {
		t.Errorf(`buildTextCQL(x\) = %q`, got)
	}
}
