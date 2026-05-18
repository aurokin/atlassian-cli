package jiracmd

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aurokin/atlassian-cli/internal/apperr"
)

func TestProjectListHumanOutput(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"values":[{"key":"PROJ","name":"Project X"},{"key":"OPS","name":"Operations"}],"isLast":true}`))
	}))
	defer srv.Close()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginJiraSite(t, srv.URL)

	out, err := execJira(t, "project", "list", "--site", "work")
	if err != nil {
		t.Fatalf("project list: %v", err)
	}
	for _, want := range []string{"PROJ", "Project X", "OPS", "Operations"} {
		if !strings.Contains(out, want) {
			t.Errorf("project list output missing %q:\n%s", want, out)
		}
	}
}

func TestProjectListEmpty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"values":[],"isLast":true}`))
	}))
	defer srv.Close()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginJiraSite(t, srv.URL)

	out, err := execJira(t, "project", "list", "--site", "work")
	if err != nil {
		t.Fatalf("project list: %v", err)
	}
	if !strings.Contains(out, "No projects found") {
		t.Fatalf("empty project list output:\n%s", out)
	}
}

func TestProjectViewJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"id":"100","key":"PROJ","name":"Project X","projectTypeKey":"software"}`))
	}))
	defer srv.Close()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginJiraSite(t, srv.URL)

	out, err := execJira(t, "project", "view", "PROJ", "--site", "work", "--json")
	if err != nil {
		t.Fatalf("project view --json: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("project view --json output is not valid JSON: %v\n%s", err, out)
	}
	if got["key"] != "PROJ" || got["name"] != "Project X" {
		t.Fatalf("unexpected project JSON: %v", got)
	}
}

func TestProjectViewHumanOutput(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"key":"PROJ","name":"Project X","projectTypeKey":"software","lead":{"displayName":"Ada Lovelace"}}`))
	}))
	defer srv.Close()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginJiraSite(t, srv.URL)

	out, err := execJira(t, "project", "view", "PROJ", "--site", "work")
	if err != nil {
		t.Fatalf("project view: %v", err)
	}
	for _, want := range []string{"PROJ", "Project X", "software", "Ada Lovelace"} {
		if !strings.Contains(out, want) {
			t.Errorf("project view output missing %q:\n%s", want, out)
		}
	}
}

func TestProjectViewMapsNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"errorMessages":["No project could be found with key 'NOPE'."]}`))
	}))
	defer srv.Close()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginJiraSite(t, srv.URL)

	_, err := execJira(t, "project", "view", "NOPE", "--site", "work")
	if err == nil {
		t.Fatal("project view of a missing project returned no error")
	}
	var ae *apperr.Error
	if !errors.As(err, &ae) {
		t.Fatalf("error type = %T, want *apperr.Error", err)
	}
	if ae.Code != apperr.CodeNotFoundOrNotVisible {
		t.Errorf("Code = %q, want %q", ae.Code, apperr.CodeNotFoundOrNotVisible)
	}
}

func TestProjectListRequiresSite(t *testing.T) {
	_, err := execJira(t, "project", "list")
	if err == nil {
		t.Fatal("project list without --site returned no error")
	}
	var ae *apperr.Error
	if !errors.As(err, &ae) {
		t.Fatalf("error type = %T, want *apperr.Error", err)
	}
}

func TestProjectViewRequiresExactlyOneArg(t *testing.T) {
	if _, err := execJira(t, "project", "view", "--site", "work"); err == nil {
		t.Fatal("project view with no key returned no error")
	}
}
