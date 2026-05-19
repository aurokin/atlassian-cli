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

func TestSpaceListHumanOutput(t *testing.T) {
	var gotLimit string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/spaces" {
			t.Errorf("path = %q, want /spaces", r.URL.Path)
		}
		gotLimit = r.URL.Query().Get("limit")
		_, _ = w.Write([]byte(`{"results":[{"id":"1","key":"DEV","name":"Development"},` +
			`{"id":"2","key":"OPS","name":"Operations"}]}`))
	}))
	defer srv.Close()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginConfSite(t, srv.URL)

	out, err := execConf(t, "space", "list", "--limit", "5", "--site", "work")
	if err != nil {
		t.Fatalf("space list: %v", err)
	}
	if gotLimit != "5" {
		t.Errorf("space list sent limit %q, want 5", gotLimit)
	}
	for _, want := range []string{"DEV", "Development", "OPS", "Operations"} {
		if !strings.Contains(out, want) {
			t.Errorf("space list output missing %q:\n%s", want, out)
		}
	}
}

func TestSpaceListEmpty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"results":[]}`))
	}))
	defer srv.Close()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginConfSite(t, srv.URL)

	out, err := execConf(t, "space", "list", "--site", "work")
	if err != nil {
		t.Fatalf("space list: %v", err)
	}
	if !strings.Contains(out, "No spaces found") {
		t.Fatalf("empty space list output:\n%s", out)
	}
}

func TestSpaceListJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"results":[{"id":"1","key":"DEV","name":"Development"}]}`))
	}))
	defer srv.Close()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginConfSite(t, srv.URL)

	out, err := execConf(t, "space", "list", "--site", "work", "--json")
	if err != nil {
		t.Fatalf("space list --json: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("space list --json output is not valid JSON: %v\n%s", err, out)
	}
	if _, ok := got["results"]; !ok {
		t.Fatalf("unexpected space list JSON: %v", got)
	}
}

func TestSpaceViewHumanOutput(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/spaces":
			if got := r.URL.Query().Get("keys"); got != "DEV" {
				t.Errorf("keys param = %q, want DEV", got)
			}
			_, _ = w.Write([]byte(`{"results":[{"id":"1","key":"DEV","name":"Development"}]}`))
		case "/spaces/1":
			_, _ = w.Write([]byte(`{"id":"1","key":"DEV","name":"Development","type":"global"}`))
		default:
			t.Errorf("unexpected path %q", r.URL.Path)
		}
	}))
	defer srv.Close()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginConfSite(t, srv.URL)

	out, err := execConf(t, "space", "view", "DEV", "--site", "work")
	if err != nil {
		t.Fatalf("space view: %v", err)
	}
	for _, want := range []string{"DEV", "Development", "global"} {
		if !strings.Contains(out, want) {
			t.Errorf("space view output missing %q:\n%s", want, out)
		}
	}
}

func TestSpaceViewJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/spaces":
			_, _ = w.Write([]byte(`{"results":[{"id":"1","key":"DEV","name":"Development"}]}`))
		case "/spaces/1":
			_, _ = w.Write([]byte(`{"id":"1","key":"DEV","name":"Development","type":"global"}`))
		default:
			t.Errorf("unexpected path %q", r.URL.Path)
		}
	}))
	defer srv.Close()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginConfSite(t, srv.URL)

	out, err := execConf(t, "space", "view", "DEV", "--site", "work", "--json")
	if err != nil {
		t.Fatalf("space view --json: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("space view --json output is not valid JSON: %v\n%s", err, out)
	}
	if got["key"] != "DEV" {
		t.Fatalf("unexpected space JSON: %v", got)
	}
}

func TestSpaceViewNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		// A key lookup that matches no space returns an empty result list.
		_, _ = w.Write([]byte(`{"results":[]}`))
	}))
	defer srv.Close()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginConfSite(t, srv.URL)

	_, err := execConf(t, "space", "view", "NOPE", "--site", "work")
	if err == nil {
		t.Fatal("space view of a missing space returned no error")
	}
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Code != apperr.CodeNotFoundOrNotVisible {
		t.Fatalf("error = %v, want a not_found_or_not_visible *apperr.Error", err)
	}
}

func TestSpaceListMapsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"message":"You do not have permission."}`))
	}))
	defer srv.Close()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginConfSite(t, srv.URL)

	_, err := execConf(t, "space", "list", "--site", "work")
	if err == nil {
		t.Fatal("space list against a 403 endpoint returned no error")
	}
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Code != apperr.CodeForbidden {
		t.Fatalf("error = %v, want a forbidden *apperr.Error", err)
	}
}

func TestSpaceListRequiresSite(t *testing.T) {
	_, err := execConf(t, "space", "list")
	if err == nil {
		t.Fatal("space list without --site returned no error")
	}
	var ae *apperr.Error
	if !errors.As(err, &ae) {
		t.Fatalf("error type = %T, want *apperr.Error", err)
	}
}

func TestSpaceViewRequiresExactlyOneArg(t *testing.T) {
	if _, err := execConf(t, "space", "view", "--site", "work"); err == nil {
		t.Fatal("space view with no key returned no error")
	}
}
