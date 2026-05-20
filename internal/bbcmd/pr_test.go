package bbcmd

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPRListHumanAndStateQuery(t *testing.T) {
	var gotState, gotPagelen string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repositories/acme/widgets/pullrequests" {
			t.Errorf("path = %q", r.URL.Path)
		}
		gotState = r.URL.Query().Get("state")
		gotPagelen = r.URL.Query().Get("pagelen")
		_, _ = w.Write([]byte(`{"values":[` +
			`{"id":7,"title":"Add widget","state":"OPEN"},` +
			`{"id":4,"title":"Fix gadget","state":"OPEN"}]}`))
	}))
	defer srv.Close()

	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginBBSite(t, srv.URL)

	out, err := execBB(t, "pr", "list", "--repo", "acme/widgets", "--site", "work", "--state", "open", "--limit", "25")
	if err != nil {
		t.Fatalf("pr list: %v\n%s", err, out)
	}
	if gotState != "OPEN" {
		t.Fatalf("state query = %q, want OPEN", gotState)
	}
	if gotPagelen != "25" {
		t.Fatalf("pagelen query = %q, want 25", gotPagelen)
	}
	for _, want := range []string{"#7", "Add widget", "#4", "Fix gadget", "OPEN"} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q:\n%s", want, out)
		}
	}
}

func TestPRListStateAllOmitsFilter(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("state"); got != "" {
			t.Errorf("state query = %q, want empty for ALL", got)
		}
		_, _ = w.Write([]byte(`{"values":[]}`))
	}))
	defer srv.Close()

	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginBBSite(t, srv.URL)

	out, err := execBB(t, "pr", "list", "--repo", "acme/widgets", "--site", "work", "--state", "ALL")
	if err != nil {
		t.Fatalf("pr list: %v\n%s", err, out)
	}
	if !strings.Contains(out, "No pull requests found.") {
		t.Fatalf("expected empty message:\n%s", out)
	}
}

func TestPRListInvalidState(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	_, err := execBB(t, "pr", "list", "--repo", "acme/widgets", "--site", "work", "--state", "bogus")
	if err == nil || !strings.Contains(err.Error(), "invalid --state") {
		t.Fatalf("expected invalid state error, got %v", err)
	}
}

func TestPRViewHumanAndJSON(t *testing.T) {
	body := `{"id":7,"title":"Add widget","state":"OPEN",` +
		`"author":{"display_name":"Auro"},` +
		`"source":{"branch":{"name":"feature"}},"destination":{"branch":{"name":"main"}}}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repositories/acme/widgets/pullrequests/7" {
			t.Errorf("path = %q", r.URL.Path)
		}
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()

	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginBBSite(t, srv.URL)

	out, err := execBB(t, "pr", "view", "7", "--repo", "acme/widgets", "--site", "work")
	if err != nil {
		t.Fatalf("pr view: %v\n%s", err, out)
	}
	for _, want := range []string{"#7", "Add widget", "OPEN", "Auro", "feature → main"} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q:\n%s", want, out)
		}
	}

	jsonOut, err := execBB(t, "pr", "view", "7", "--repo", "acme/widgets", "--site", "work", "--jq", ".id")
	if err != nil {
		t.Fatalf("pr view --jq: %v\n%s", err, jsonOut)
	}
	if strings.TrimSpace(jsonOut) != "7" {
		t.Fatalf("jq output = %q", jsonOut)
	}
}

func TestPRViewInvalidID(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	_, err := execBB(t, "pr", "view", "abc", "--repo", "acme/widgets", "--site", "work")
	if err == nil || !strings.Contains(err.Error(), "invalid pull request id") {
		t.Fatalf("expected invalid id error, got %v", err)
	}
}

func TestPRCreateSendsBodyAndReportsResult(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want POST", r.Method)
		}
		if r.URL.Path != "/repositories/acme/widgets/pullrequests" {
			t.Errorf("path = %q", r.URL.Path)
		}
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &gotBody)
		_, _ = w.Write([]byte(`{"id":12,"title":"Add widget","state":"OPEN"}`))
	}))
	defer srv.Close()

	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginBBSite(t, srv.URL)

	out, err := execBB(t, "pr", "create", "--repo", "acme/widgets", "--site", "work",
		"--title", "Add widget", "--source", "feature", "--destination", "main",
		"--description", "does things", "--draft")
	if err != nil {
		t.Fatalf("pr create: %v\n%s", err, out)
	}
	if !strings.Contains(out, "created pull request #12: Add widget") {
		t.Fatalf("unexpected output:\n%s", out)
	}
	if gotBody["title"] != "Add widget" || gotBody["description"] != "does things" || gotBody["draft"] != true {
		t.Fatalf("request body = %+v", gotBody)
	}
	src, _ := gotBody["source"].(map[string]any)
	branch, _ := src["branch"].(map[string]any)
	if branch["name"] != "feature" {
		t.Fatalf("source branch = %+v", gotBody["source"])
	}
}

func TestPRCreateRequiresFields(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cases := []struct {
		name string
		args []string
		want string
	}{
		{"no title", []string{"pr", "create", "--repo", "acme/widgets", "--site", "work", "--source", "f", "--destination", "main"}, "a title is required"},
		{"no source", []string{"pr", "create", "--repo", "acme/widgets", "--site", "work", "--title", "t", "--destination", "main"}, "a source branch is required"},
		{"no destination", []string{"pr", "create", "--repo", "acme/widgets", "--site", "work", "--title", "t", "--source", "f"}, "a destination branch is required"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := execBB(t, tc.args...)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want substring %q", err, tc.want)
			}
		})
	}
}

func TestPRListAllFollowsPagination(t *testing.T) {
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Query().Get("page") {
		case "", "1":
			_, _ = w.Write([]byte(`{"values":[{"id":1,"title":"one","state":"OPEN"}],` +
				`"next":"` + srv.URL + `/repositories/acme/widgets/pullrequests?page=2"}`))
		case "2":
			_, _ = w.Write([]byte(`{"values":[{"id":2,"title":"two","state":"OPEN"}]}`))
		default:
			t.Errorf("unexpected page %q", r.URL.Query().Get("page"))
		}
	}))
	defer srv.Close()

	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginBBSite(t, srv.URL)

	out, err := execBB(t, "pr", "list", "--repo", "acme/widgets", "--site", "work", "--all")
	if err != nil {
		t.Fatalf("pr list --all: %v\n%s", err, out)
	}
	for _, want := range []string{"#1", "one", "#2", "two"} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q:\n%s", want, out)
		}
	}
}

func TestNormalizePRState(t *testing.T) {
	cases := map[string]string{"open": "OPEN", "MERGED": "MERGED", "all": "", "": ""}
	for in, want := range cases {
		got, err := normalizePRState(in)
		if err != nil {
			t.Fatalf("normalizePRState(%q): %v", in, err)
		}
		if got != want {
			t.Fatalf("normalizePRState(%q) = %q, want %q", in, got, want)
		}
	}
	if _, err := normalizePRState("nope"); err == nil {
		t.Fatalf("expected error for invalid state")
	}
}
