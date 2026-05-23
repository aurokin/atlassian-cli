package bbcmd

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/aurokin/atlassian-cli/internal/bitbucket"
)

func TestRepoViewHuman(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repositories/acme/widgets" {
			t.Errorf("path = %q", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"full_name":"acme/widgets","name":"widgets","is_private":true,` +
			`"mainbranch":{"name":"main"},"project":{"key":"WID","name":"Widgets"},` +
			`"description":"the widget repo"}`))
	}))
	defer srv.Close()

	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginBBSite(t, srv.URL)

	out, err := execBB(t, "repo", "view", "acme/widgets", "--site", "work")
	if err != nil {
		t.Fatalf("repo view: %v\n%s", err, out)
	}
	for _, want := range []string{"acme/widgets", "private", "WID — Widgets", "main", "the widget repo"} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q:\n%s", want, out)
		}
	}
}

func TestRepoViewViaRepoFlag(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repositories/acme/widgets" {
			t.Errorf("path = %q", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"full_name":"acme/widgets","is_private":false}`))
	}))
	defer srv.Close()

	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginBBSite(t, srv.URL)

	out, err := execBB(t, "repo", "view", "--repo", "widgets", "--workspace", "acme", "--site", "work")
	if err != nil {
		t.Fatalf("repo view: %v\n%s", err, out)
	}
	if !strings.Contains(out, "public") {
		t.Fatalf("expected public visibility:\n%s", out)
	}
}

func TestRepoViewJSONIsRawAPIBody(t *testing.T) {
	body := `{"full_name":"acme/widgets","is_private":true,"project":{"key":"WID"}}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()

	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginBBSite(t, srv.URL)

	out, err := execBB(t, "repo", "view", "acme/widgets", "--site", "work", "--json")
	if err != nil {
		t.Fatalf("repo view --json: %v\n%s", err, out)
	}
	// Raw API field names (e.g. is_private, project.key) are preserved, not
	// re-shaped into legacy bb's custom payload (which used private/project_key).
	for _, want := range []string{`"full_name"`, `"is_private"`, `"project"`, `"key"`} {
		if !strings.Contains(out, want) {
			t.Fatalf("JSON output missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "project_key") {
		t.Fatalf("output should not carry legacy bb field project_key:\n%s", out)
	}
}

func TestRepoViewJQ(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"full_name":"acme/widgets","is_private":true}`))
	}))
	defer srv.Close()

	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginBBSite(t, srv.URL)

	out, err := execBB(t, "repo", "view", "acme/widgets", "--site", "work", "--jq", ".full_name")
	if err != nil {
		t.Fatalf("repo view --jq: %v\n%s", err, out)
	}
	if strings.TrimSpace(out) != `"acme/widgets"` {
		t.Fatalf("jq output = %q", out)
	}
}

func TestRepoListAllFollowsPagination(t *testing.T) {
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repositories/acme" {
			t.Errorf("path = %q", r.URL.Path)
		}
		switch r.URL.Query().Get("page") {
		case "", "1":
			_, _ = w.Write([]byte(`{"values":[{"full_name":"acme/a","is_private":false}],` +
				`"next":"` + srv.URL + `/repositories/acme?page=2"}`))
		case "2":
			_, _ = w.Write([]byte(`{"values":[{"full_name":"acme/b","is_private":true}]}`))
		default:
			t.Errorf("unexpected page %q", r.URL.Query().Get("page"))
		}
	}))
	defer srv.Close()

	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginBBSite(t, srv.URL)

	out, err := execBB(t, "repo", "list", "acme", "--site", "work", "--all")
	if err != nil {
		t.Fatalf("repo list --all: %v\n%s", err, out)
	}
	for _, want := range []string{"acme/a", "public", "acme/b", "private"} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q:\n%s", want, out)
		}
	}
}

func TestRepoTargetErrors(t *testing.T) {
	stubInferDisabled(t)
	cases := []struct {
		name      string
		args      []string
		repoFlag  string
		workspace string
		wantErr   string
	}{
		{"no target", nil, "", "", "a repository is required"},
		{"bare repo no workspace", nil, "widgets", "", "has no workspace"},
		{"conflicting workspace", nil, "acme/widgets", "other", "conflicts"},
		{"empty side", nil, "acme/", "", "expected <workspace>/<repo>"},
		{"too many slashes", nil, "acme/team/widgets", "", "expected <workspace>/<repo>"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := resolveRepoTarget(tc.args, tc.repoFlag, tc.workspace)
			if err == nil {
				t.Fatalf("expected error")
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error = %q, want substring %q", err.Error(), tc.wantErr)
			}
		})
	}
}

func TestRepoListAllDefaultsToMaxPageSize(t *testing.T) {
	var gotPagelen string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPagelen = r.URL.Query().Get("pagelen")
		// One page, no "next": the follow completes after a single request.
		_, _ = w.Write([]byte(`{"values":[{"full_name":"acme/widgets"}]}`))
	}))
	defer srv.Close()

	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginBBSite(t, srv.URL)

	// --all with no --limit must request the API max page size.
	if _, err := execBB(t, "repo", "list", "--workspace", "acme", "--site", "work", "--all"); err != nil {
		t.Fatalf("repo list --all: %v", err)
	}
	if gotPagelen != strconv.Itoa(bitbucket.MaxPageLen) {
		t.Fatalf("pagelen query = %q, want %d", gotPagelen, bitbucket.MaxPageLen)
	}
}

func TestResolveWorkspace(t *testing.T) {
	t.Run("conflicting positional and flag rejected", func(t *testing.T) {
		_, err := resolveWorkspace([]string{"acme"}, "other")
		if err == nil || !strings.Contains(err.Error(), "conflicts") {
			t.Fatalf("expected conflict error, got %v", err)
		}
	})
	t.Run("matching positional and flag ok", func(t *testing.T) {
		ws, err := resolveWorkspace([]string{"acme"}, "acme")
		if err != nil || ws != "acme" {
			t.Fatalf("ws = %q, err = %v", ws, err)
		}
	})
	t.Run("missing workspace with no inference", func(t *testing.T) {
		orig := inferRepoTarget
		inferRepoTarget = func() (repoTarget, bool) { return repoTarget{}, false }
		t.Cleanup(func() { inferRepoTarget = orig })
		if _, err := resolveWorkspace(nil, ""); err == nil {
			t.Fatalf("expected error")
		}
	})
	t.Run("falls back to inferred workspace", func(t *testing.T) {
		orig := inferRepoTarget
		inferRepoTarget = func() (repoTarget, bool) {
			return repoTarget{Workspace: "inferred-ws", Repo: "widgets"}, true
		}
		t.Cleanup(func() { inferRepoTarget = orig })
		ws, err := resolveWorkspace(nil, "")
		if err != nil || ws != "inferred-ws" {
			t.Fatalf("ws = %q, err = %v; want inferred-ws", ws, err)
		}
	})
	t.Run("explicit flag wins over inference", func(t *testing.T) {
		orig := inferRepoTarget
		inferRepoTarget = func() (repoTarget, bool) {
			return repoTarget{Workspace: "inferred-ws"}, true
		}
		t.Cleanup(func() { inferRepoTarget = orig })
		ws, err := resolveWorkspace(nil, "explicit")
		if err != nil || ws != "explicit" {
			t.Fatalf("ws = %q, err = %v; want explicit", ws, err)
		}
	})
	t.Run("slash rejected", func(t *testing.T) {
		if _, err := resolveWorkspace([]string{"acme/widgets"}, ""); err == nil {
			t.Fatalf("expected error")
		}
	})
}

func TestAllPageSize(t *testing.T) {
	if got := allPageSize(0); got != bitbucket.MaxPageLen {
		t.Errorf("allPageSize(0) = %d, want %d", got, bitbucket.MaxPageLen)
	}
	if got := allPageSize(-1); got != bitbucket.MaxPageLen {
		t.Errorf("allPageSize(-1) = %d, want %d", got, bitbucket.MaxPageLen)
	}
	if got := allPageSize(25); got != 25 {
		t.Errorf("allPageSize(25) = %d, want 25", got)
	}
}

func TestRepoTargetResolves(t *testing.T) {
	cases := []struct {
		name          string
		args          []string
		repoFlag      string
		workspace     string
		wantWorkspace string
		wantRepo      string
	}{
		{"positional qualified", []string{"acme/widgets"}, "", "", "acme", "widgets"},
		{"repo flag qualified", nil, "acme/widgets", "", "acme", "widgets"},
		{"bare repo with workspace", nil, "widgets", "acme", "acme", "widgets"},
		{"positional wins over flag", []string{"acme/widgets"}, "other/thing", "", "acme", "widgets"},
		{"matching workspace ok", nil, "acme/widgets", "acme", "acme", "widgets"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := resolveRepoTarget(tc.args, tc.repoFlag, tc.workspace)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.Workspace != tc.wantWorkspace || got.Repo != tc.wantRepo {
				t.Fatalf("target = %+v, want %s/%s", got, tc.wantWorkspace, tc.wantRepo)
			}
		})
	}
}
