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

func TestPageListHumanOutput(t *testing.T) {
	var gotSpaceID, gotLimit string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/spaces":
			_, _ = w.Write([]byte(`{"results":[{"id":"1","key":"DEV","name":"Development"}]}`))
		case "/pages":
			gotSpaceID = r.URL.Query().Get("space-id")
			gotLimit = r.URL.Query().Get("limit")
			_, _ = w.Write([]byte(`{"results":[{"id":"10","title":"Home","status":"current","spaceId":"1"}]}`))
		default:
			t.Errorf("unexpected path %q", r.URL.Path)
		}
	}))
	defer srv.Close()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginConfSite(t, srv.URL)

	out, err := execConf(t, "page", "list", "--space", "DEV", "--limit", "4", "--site", "work")
	if err != nil {
		t.Fatalf("page list: %v", err)
	}
	if gotSpaceID != "1" {
		t.Errorf("page list sent space-id %q, want 1 (space key not resolved)", gotSpaceID)
	}
	if gotLimit != "4" {
		t.Errorf("page list sent limit %q, want 4", gotLimit)
	}
	for _, want := range []string{"10", "Home"} {
		if !strings.Contains(out, want) {
			t.Errorf("page list output missing %q:\n%s", want, out)
		}
	}
}

func TestPageListRequiresSpace(t *testing.T) {
	_, err := execConf(t, "page", "list", "--site", "work")
	if err == nil {
		t.Fatal("page list without --space returned no error")
	}
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Code != apperr.CodeInvalidInput {
		t.Fatalf("error = %v, want an invalid_input *apperr.Error", err)
	}
}

func TestPageListJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/spaces":
			_, _ = w.Write([]byte(`{"results":[{"id":"1","key":"DEV"}]}`))
		case "/pages":
			_, _ = w.Write([]byte(`{"results":[{"id":"10","title":"Home"}]}`))
		default:
			t.Errorf("unexpected path %q", r.URL.Path)
		}
	}))
	defer srv.Close()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginConfSite(t, srv.URL)

	out, err := execConf(t, "page", "list", "--space", "DEV", "--site", "work", "--json")
	if err != nil {
		t.Fatalf("page list --json: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("page list --json output is not valid JSON: %v\n%s", err, out)
	}
	if _, ok := got["results"]; !ok {
		t.Fatalf("unexpected page list JSON: %v", got)
	}
}

func TestPageViewHumanOutput(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/pages/10" {
			t.Errorf("path = %q, want /pages/10", r.URL.Path)
		}
		if got := r.URL.Query().Get("body-format"); got != "storage" {
			t.Errorf("body-format = %q, want storage", got)
		}
		_, _ = w.Write([]byte(`{"id":"10","title":"Home","status":"current",` +
			`"spaceId":"1","version":{"number":3}}`))
	}))
	defer srv.Close()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginConfSite(t, srv.URL)

	out, err := execConf(t, "page", "view", "10", "--site", "work")
	if err != nil {
		t.Fatalf("page view: %v", err)
	}
	for _, want := range []string{"10", "Home", "current", "version:", "3"} {
		if !strings.Contains(out, want) {
			t.Errorf("page view output missing %q:\n%s", want, out)
		}
	}
}

func TestPageViewJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"id":"10","title":"Home"}`))
	}))
	defer srv.Close()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginConfSite(t, srv.URL)

	out, err := execConf(t, "page", "view", "10", "--site", "work", "--json")
	if err != nil {
		t.Fatalf("page view --json: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("page view --json output is not valid JSON: %v\n%s", err, out)
	}
	if got["id"] != "10" {
		t.Fatalf("unexpected page JSON: %v", got)
	}
}

func TestPageViewMapsNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"Page not found"}`))
	}))
	defer srv.Close()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginConfSite(t, srv.URL)

	_, err := execConf(t, "page", "view", "404", "--site", "work")
	if err == nil {
		t.Fatal("page view of a missing page returned no error")
	}
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Code != apperr.CodeNotFoundOrNotVisible {
		t.Fatalf("error = %v, want a not_found_or_not_visible *apperr.Error", err)
	}
}

func TestPageChildrenHumanOutput(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/pages/10/children" {
			t.Errorf("path = %q, want /pages/10/children", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"results":[{"id":"11","title":"Child A","status":"current"},` +
			`{"id":"12","title":"Child B","status":"current"}]}`))
	}))
	defer srv.Close()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginConfSite(t, srv.URL)

	out, err := execConf(t, "page", "children", "10", "--site", "work")
	if err != nil {
		t.Fatalf("page children: %v", err)
	}
	for _, want := range []string{"11", "Child A", "12", "Child B"} {
		if !strings.Contains(out, want) {
			t.Errorf("page children output missing %q:\n%s", want, out)
		}
	}
}

func TestPageChildrenEmpty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"results":[]}`))
	}))
	defer srv.Close()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginConfSite(t, srv.URL)

	out, err := execConf(t, "page", "children", "10", "--site", "work")
	if err != nil {
		t.Fatalf("page children: %v", err)
	}
	if !strings.Contains(out, "No pages found") {
		t.Fatalf("empty page children output:\n%s", out)
	}
}

func TestPageViewRequiresExactlyOneArg(t *testing.T) {
	if _, err := execConf(t, "page", "view", "--site", "work"); err == nil {
		t.Fatal("page view with no id returned no error")
	}
}
