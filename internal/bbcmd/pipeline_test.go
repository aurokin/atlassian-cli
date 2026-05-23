package bbcmd

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPipelineListHumanAndQuery(t *testing.T) {
	var gotStatus, gotSort string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repositories/acme/widgets/pipelines/" {
			t.Errorf("path = %q", r.URL.Path)
		}
		gotStatus = r.URL.Query().Get("status")
		gotSort = r.URL.Query().Get("sort")
		_, _ = w.Write([]byte(`{"values":[` +
			`{"build_number":42,"state":{"name":"COMPLETED","result":{"name":"SUCCESSFUL"}},"target":{"ref_type":"branch","ref_name":"main"}},` +
			`{"build_number":41,"state":{"name":"IN_PROGRESS"},"target":{"ref_type":"branch","ref_name":"dev"}}]}`))
	}))
	defer srv.Close()

	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginBBSite(t, srv.URL)

	out, err := execBB(t, "pipeline", "list", "--repo", "acme/widgets", "--site", "work", "--status", "completed", "--limit", "5")
	if err != nil {
		t.Fatalf("pipeline list: %v\n%s", err, out)
	}
	if gotStatus != "COMPLETED" {
		t.Fatalf("status query = %q, want COMPLETED", gotStatus)
	}
	if gotSort != "-created_on" {
		t.Fatalf("sort query = %q, want -created_on", gotSort)
	}
	for _, want := range []string{"#42", "COMPLETED (SUCCESSFUL)", "branch:main", "#41", "IN_PROGRESS"} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q:\n%s", want, out)
		}
	}
}

func TestPipelineViewByUUID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repositories/acme/widgets/pipelines/{abc-123}" {
			t.Errorf("path = %q", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"uuid":"{abc-123}","build_number":7,"state":{"name":"COMPLETED"}}`))
	}))
	defer srv.Close()

	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginBBSite(t, srv.URL)

	// A bare UUID is normalized to brace-wrapped form.
	out, err := execBB(t, "pipeline", "view", "abc-123", "--repo", "acme/widgets", "--site", "work")
	if err != nil {
		t.Fatalf("pipeline view: %v\n%s", err, out)
	}
	for _, want := range []string{"#7", "{abc-123}", "COMPLETED"} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q:\n%s", want, out)
		}
	}
}

func TestPipelineViewByBuildNumber(t *testing.T) {
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repositories/acme/widgets/pipelines/" {
			t.Errorf("path = %q", r.URL.Path)
		}
		switch r.URL.Query().Get("page") {
		case "", "1":
			_, _ = w.Write([]byte(`{"values":[{"build_number":50,"uuid":"{x}"}],` +
				`"next":"` + srv.URL + `/repositories/acme/widgets/pipelines/?page=2"}`))
		case "2":
			_, _ = w.Write([]byte(`{"values":[{"build_number":42,"uuid":"{target}","state":{"name":"COMPLETED"}}]}`))
		default:
			t.Errorf("unexpected page %q", r.URL.Query().Get("page"))
		}
	}))
	defer srv.Close()

	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginBBSite(t, srv.URL)

	out, err := execBB(t, "pipeline", "view", "42", "--repo", "acme/widgets", "--site", "work", "--jq", ".uuid")
	if err != nil {
		t.Fatalf("pipeline view by number: %v\n%s", err, out)
	}
	if strings.TrimSpace(out) != `"{target}"` {
		t.Fatalf("jq output = %q", out)
	}
}

func TestPipelineViewBuildNumberNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"values":[{"build_number":1}]}`))
	}))
	defer srv.Close()

	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginBBSite(t, srv.URL)

	_, err := execBB(t, "pipeline", "view", "999", "--repo", "acme/widgets", "--site", "work")
	if err == nil || !strings.Contains(err.Error(), "#999 was not found") {
		t.Fatalf("expected not-found error, got %v", err)
	}
}

func TestPipelineRunSendsTargetBody(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want POST", r.Method)
		}
		if r.URL.Path != "/repositories/acme/widgets/pipelines" {
			t.Errorf("path = %q (trigger must omit trailing slash)", r.URL.Path)
		}
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &gotBody)
		_, _ = w.Write([]byte(`{"build_number":99,"state":{"name":"PENDING"}}`))
	}))
	defer srv.Close()

	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginBBSite(t, srv.URL)

	out, err := execBB(t, "pipeline", "run", "--repo", "acme/widgets", "--site", "work", "--ref", "main")
	if err != nil {
		t.Fatalf("pipeline run: %v\n%s", err, out)
	}
	if !strings.Contains(out, "triggered pipeline #99 on branch main") {
		t.Fatalf("unexpected output:\n%s", out)
	}
	target, _ := gotBody["target"].(map[string]any)
	if target["type"] != "pipeline_ref_target" || target["ref_type"] != "branch" || target["ref_name"] != "main" {
		t.Fatalf("target body = %+v", gotBody["target"])
	}
}

func TestPipelineRunDefaultsRefTypeInMessage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"build_number":5,"state":{"name":"PENDING"}}`))
	}))
	defer srv.Close()

	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginBBSite(t, srv.URL)

	// An explicitly blank --ref-type still reports (and sends) "branch".
	out, err := execBB(t, "pipeline", "run", "--repo", "acme/widgets", "--site", "work",
		"--ref", "main", "--ref-type", "")
	if err != nil {
		t.Fatalf("pipeline run: %v\n%s", err, out)
	}
	if !strings.Contains(out, "triggered pipeline #5 on branch main") {
		t.Fatalf("unexpected output:\n%s", out)
	}
}

func TestPipelineRunRequiresRef(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	_, err := execBB(t, "pipeline", "run", "--repo", "acme/widgets", "--site", "work")
	if err == nil || !strings.Contains(err.Error(), "a ref is required") {
		t.Fatalf("expected ref-required error, got %v", err)
	}
}

func TestPipelineStopByUUID(t *testing.T) {
	var gotMethod, gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath = r.Method, r.URL.Path
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginBBSite(t, srv.URL)

	out, err := execBB(t, "pipeline", "stop", "{abc-123}", "--repo", "acme/widgets", "--site", "work")
	if err != nil {
		t.Fatalf("pipeline stop: %v\n%s", err, out)
	}
	if gotMethod != http.MethodPost || gotPath != "/repositories/acme/widgets/pipelines/{abc-123}/stopPipeline" {
		t.Errorf("request = %s %s", gotMethod, gotPath)
	}
	if !strings.Contains(out, "stopped pipeline {abc-123}") {
		t.Errorf("output missing confirmation:\n%s", out)
	}
}

func TestPipelineStopByBuildNumberResolvesUUID(t *testing.T) {
	var stopPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/repositories/acme/widgets/pipelines/" && r.Method == http.MethodGet:
			_, _ = w.Write([]byte(`{"values":[{"build_number":42,"uuid":"{resolved}"}]}`))
		case strings.HasSuffix(r.URL.Path, "/stopPipeline"):
			stopPath = r.URL.Path
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer srv.Close()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginBBSite(t, srv.URL)

	out, err := execBB(t, "pipeline", "stop", "42", "--repo", "acme/widgets", "--site", "work", "--json")
	if err != nil {
		t.Fatalf("pipeline stop 42: %v\n%s", err, out)
	}
	if stopPath != "/repositories/acme/widgets/pipelines/{resolved}/stopPipeline" {
		t.Errorf("stop path = %q, want resolved uuid", stopPath)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, out)
	}
	if got["uuid"] != "{resolved}" || got["stopped"] != true {
		t.Fatalf("unexpected json: %s", out)
	}
}

func TestPipelineStepsList(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repositories/acme/widgets/pipelines/{p1}/steps/" {
			t.Errorf("path = %q", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"values":[{"uuid":"{s1}","name":"Build","state":{"name":"COMPLETED","result":{"name":"SUCCESSFUL"}}}]}`))
	}))
	defer srv.Close()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginBBSite(t, srv.URL)

	out, err := execBB(t, "pipeline", "steps", "{p1}", "--repo", "acme/widgets", "--site", "work")
	if err != nil {
		t.Fatalf("pipeline steps: %v\n%s", err, out)
	}
	for _, want := range []string{"{s1}", "Build", "SUCCESSFUL"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestPipelineLogWritesRawLog(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repositories/acme/widgets/pipelines/{p1}/steps/{s1}/log" {
			t.Errorf("path = %q", r.URL.Path)
		}
		_, _ = w.Write([]byte("+ make test\nPASS\n"))
	}))
	defer srv.Close()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginBBSite(t, srv.URL)

	out, err := execBB(t, "pipeline", "log", "{p1}", "{s1}", "--repo", "acme/widgets", "--site", "work")
	if err != nil {
		t.Fatalf("pipeline log: %v\n%s", err, out)
	}
	if out != "+ make test\nPASS\n" {
		t.Errorf("log output = %q", out)
	}
}
