package bbcmd

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDeploymentListHuman(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repositories/acme/widgets/deployments/" {
			t.Errorf("path = %q", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"values":[` +
			`{"uuid":"{d-1}","state":{"name":"COMPLETED"},"environment":{"name":"Production"}},` +
			`{"uuid":"{d-2}","state":{"name":"IN_PROGRESS"},"environment":{"name":"Staging"}}]}`))
	}))
	defer srv.Close()

	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginBBSite(t, srv.URL)

	out, err := execBB(t, "deployment", "list", "--repo", "acme/widgets", "--site", "work")
	if err != nil {
		t.Fatalf("deployment list: %v\n%s", err, out)
	}
	for _, want := range []string{"{d-1}", "COMPLETED", "Production", "{d-2}", "Staging"} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q:\n%s", want, out)
		}
	}
}

func TestDeploymentViewHumanAndJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.EscapedPath(); got != "/repositories/acme/widgets/deployments/%7Bd-1%7D" {
			t.Errorf("escaped path = %q", got)
		}
		_, _ = w.Write([]byte(`{"uuid":"{d-1}","state":{"name":"COMPLETED"},"environment":{"name":"Production"},"release":{"name":"build-42"}}`))
	}))
	defer srv.Close()

	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginBBSite(t, srv.URL)

	// A bare UUID is brace-wrapped by the client.
	out, err := execBB(t, "deployment", "view", "d-1", "--repo", "acme/widgets", "--site", "work")
	if err != nil {
		t.Fatalf("deployment view: %v\n%s", err, out)
	}
	for _, want := range []string{"{d-1}", "COMPLETED", "Production", "build-42"} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q:\n%s", want, out)
		}
	}

	jsonOut, err := execBB(t, "deployment", "view", "{d-1}", "--repo", "acme/widgets", "--site", "work", "--jq", ".state.name")
	if err != nil {
		t.Fatalf("deployment view --jq: %v\n%s", err, jsonOut)
	}
	if strings.TrimSpace(jsonOut) != `"COMPLETED"` {
		t.Fatalf("jq output = %q", jsonOut)
	}
}

func TestDeploymentViewRequiresRepo(t *testing.T) {
	stubInferDisabled(t)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	_, err := execBB(t, "deployment", "view", "{d-1}", "--site", "work")
	if err == nil || !strings.Contains(err.Error(), "a repository is required") {
		t.Fatalf("expected repo-required error, got %v", err)
	}
}

func TestEnvironmentListHuman(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repositories/acme/widgets/environments/" {
			t.Errorf("path = %q", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"values":[` +
			`{"uuid":"{e-1}","name":"Production","type":"deployment_environment"},` +
			`{"uuid":"{e-2}","name":"Staging","type":"deployment_environment"}]}`))
	}))
	defer srv.Close()

	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginBBSite(t, srv.URL)

	out, err := execBB(t, "environment", "list", "--repo", "acme/widgets", "--site", "work")
	if err != nil {
		t.Fatalf("environment list: %v\n%s", err, out)
	}
	for _, want := range []string{"Production", "Staging", "{e-1}", "{e-2}"} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q:\n%s", want, out)
		}
	}
}

func TestEnvironmentViewHumanAndJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.EscapedPath(); got != "/repositories/acme/widgets/environments/%7Be-1%7D" {
			t.Errorf("escaped path = %q", got)
		}
		_, _ = w.Write([]byte(`{"uuid":"{e-1}","name":"Staging","slug":"staging","type":"deployment_environment"}`))
	}))
	defer srv.Close()

	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginBBSite(t, srv.URL)

	out, err := execBB(t, "environment", "view", "e-1", "--repo", "acme/widgets", "--site", "work")
	if err != nil {
		t.Fatalf("environment view: %v\n%s", err, out)
	}
	for _, want := range []string{"Staging", "staging", "{e-1}"} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q:\n%s", want, out)
		}
	}

	jsonOut, err := execBB(t, "environment", "view", "e-1", "--repo", "acme/widgets", "--site", "work", "--jq", ".name")
	if err != nil {
		t.Fatalf("environment view --jq: %v\n%s", err, jsonOut)
	}
	if strings.TrimSpace(jsonOut) != `"Staging"` {
		t.Fatalf("jq output = %q", jsonOut)
	}
}
