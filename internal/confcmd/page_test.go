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

func TestPageCreateHumanOutput(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/spaces":
			if got := r.URL.Query().Get("keys"); got != "DEV" {
				t.Errorf("keys param = %q, want DEV", got)
			}
			_, _ = w.Write([]byte(`{"results":[{"id":"1","key":"DEV","name":"Development"}]}`))
		case "/pages":
			if r.Method != http.MethodPost {
				t.Errorf("method = %q, want POST", r.Method)
			}
			_ = json.NewDecoder(r.Body).Decode(&gotBody)
			_, _ = w.Write([]byte(`{"id":"99","title":"Release Notes","status":"current",` +
				`"spaceId":"1","version":{"number":1}}`))
		default:
			t.Errorf("unexpected path %q", r.URL.Path)
		}
	}))
	defer srv.Close()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginConfSite(t, srv.URL)

	out, err := execConf(t, "page", "create", "--space", "DEV", "--title", "Release Notes",
		"--body", "<p>hi</p>", "--body-format", "storage", "--site", "work")
	if err != nil {
		t.Fatalf("page create: %v", err)
	}
	if gotBody["spaceId"] != "1" {
		t.Errorf("create sent spaceId %v, want 1 (space key not resolved)", gotBody["spaceId"])
	}
	if gotBody["title"] != "Release Notes" {
		t.Errorf("create sent title %v, want Release Notes", gotBody["title"])
	}
	body, _ := gotBody["body"].(map[string]any)
	if body["representation"] != "storage" || body["value"] != "<p>hi</p>" {
		t.Errorf("create sent body %v, want storage/<p>hi</p>", gotBody["body"])
	}
	if !strings.Contains(out, "created page 99") {
		t.Errorf("create output missing 'created page 99':\n%s", out)
	}
}

func TestPageCreateJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/spaces":
			_, _ = w.Write([]byte(`{"results":[{"id":"1","key":"DEV"}]}`))
		case "/pages":
			_, _ = w.Write([]byte(`{"id":"99","title":"Release Notes"}`))
		default:
			t.Errorf("unexpected path %q", r.URL.Path)
		}
	}))
	defer srv.Close()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginConfSite(t, srv.URL)

	out, err := execConf(t, "page", "create", "--space", "DEV", "--title", "Release Notes",
		"--body", "<p>hi</p>", "--body-format", "storage", "--site", "work", "--json")
	if err != nil {
		t.Fatalf("page create --json: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("page create --json output is not valid JSON: %v\n%s", err, out)
	}
	if got["id"] != "99" {
		t.Fatalf("unexpected page create JSON: %v", got)
	}
}

func TestPageCreateRequiresFlags(t *testing.T) {
	_, err := execConf(t, "page", "create", "--space", "DEV", "--title", "X", "--site", "work")
	if err == nil {
		t.Fatal("page create without --body and --body-format returned no error")
	}
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Code != apperr.CodeInvalidInput {
		t.Fatalf("error = %v, want an invalid_input *apperr.Error", err)
	}
}

func TestPageCreateRejectsUnknownBodyFormat(t *testing.T) {
	_, err := execConf(t, "page", "create", "--space", "DEV", "--title", "X",
		"--body", "y", "--body-format", "html", "--site", "work")
	if err == nil {
		t.Fatal("page create with an unknown --body-format returned no error")
	}
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Code != apperr.CodeInvalidInput {
		t.Fatalf("error = %v, want an invalid_input *apperr.Error", err)
	}
}

func TestPageCreateMapsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/spaces":
			_, _ = w.Write([]byte(`{"results":[{"id":"1","key":"DEV"}]}`))
		case "/pages":
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"message":"You do not have permission."}`))
		default:
			t.Errorf("unexpected path %q", r.URL.Path)
		}
	}))
	defer srv.Close()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginConfSite(t, srv.URL)

	_, err := execConf(t, "page", "create", "--space", "DEV", "--title", "X",
		"--body", "y", "--body-format", "storage", "--site", "work")
	if err == nil {
		t.Fatal("page create against a 403 endpoint returned no error")
	}
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Code != apperr.CodeForbidden {
		t.Fatalf("error = %v, want a forbidden *apperr.Error", err)
	}
}

func TestPageEditHumanOutput(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/pages/10" {
			t.Errorf("path = %q, want /pages/10", r.URL.Path)
		}
		switch r.Method {
		case http.MethodGet:
			_, _ = w.Write([]byte(`{"id":"10","title":"Old","status":"current","spaceId":"1",` +
				`"version":{"number":3},"body":{"storage":{"representation":"storage","value":"<p>old</p>"}}}`))
		case http.MethodPut:
			_ = json.NewDecoder(r.Body).Decode(&gotBody)
			_, _ = w.Write([]byte(`{"id":"10","title":"New","status":"current",` +
				`"spaceId":"1","version":{"number":4}}`))
		default:
			t.Errorf("method = %q, want GET or PUT", r.Method)
		}
	}))
	defer srv.Close()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginConfSite(t, srv.URL)

	out, err := execConf(t, "page", "edit", "10", "--title", "New",
		"--body", "<p>new</p>", "--body-format", "storage", "--site", "work")
	if err != nil {
		t.Fatalf("page edit: %v", err)
	}
	if gotBody["title"] != "New" {
		t.Errorf("edit sent title %v, want New", gotBody["title"])
	}
	ver, _ := gotBody["version"].(map[string]any)
	if ver["number"] != float64(4) {
		t.Errorf("edit sent version %v, want number 4 (current+1)", gotBody["version"])
	}
	body, _ := gotBody["body"].(map[string]any)
	if body["value"] != "<p>new</p>" {
		t.Errorf("edit sent body %v, want value <p>new</p>", gotBody["body"])
	}
	if !strings.Contains(out, "updated page 10 to version 4") {
		t.Errorf("edit output missing 'updated page 10 to version 4':\n%s", out)
	}
}

func TestPageEditTitleOnlyReusesBody(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			_, _ = w.Write([]byte(`{"id":"10","title":"Old","status":"current","spaceId":"1",` +
				`"version":{"number":3},"body":{"storage":{"representation":"storage","value":"<p>keep me</p>"}}}`))
		case http.MethodPut:
			_ = json.NewDecoder(r.Body).Decode(&gotBody)
			_, _ = w.Write([]byte(`{"id":"10","title":"Renamed","status":"current","version":{"number":4}}`))
		default:
			t.Errorf("method = %q, want GET or PUT", r.Method)
		}
	}))
	defer srv.Close()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginConfSite(t, srv.URL)

	if _, err := execConf(t, "page", "edit", "10", "--title", "Renamed", "--site", "work"); err != nil {
		t.Fatalf("page edit: %v", err)
	}
	body, _ := gotBody["body"].(map[string]any)
	if body["representation"] != "storage" || body["value"] != "<p>keep me</p>" {
		t.Errorf("title-only edit sent body %v, want the fetched storage body re-sent", gotBody["body"])
	}
	if gotBody["title"] != "Renamed" {
		t.Errorf("edit sent title %v, want Renamed", gotBody["title"])
	}
}

func TestPageEditJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			_, _ = w.Write([]byte(`{"id":"10","title":"Old","status":"current","version":{"number":1},` +
				`"body":{"storage":{"representation":"storage","value":"<p>x</p>"}}}`))
		case http.MethodPut:
			_, _ = w.Write([]byte(`{"id":"10","title":"New","version":{"number":2}}`))
		default:
			t.Errorf("method = %q, want GET or PUT", r.Method)
		}
	}))
	defer srv.Close()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginConfSite(t, srv.URL)

	out, err := execConf(t, "page", "edit", "10", "--title", "New", "--site", "work", "--json")
	if err != nil {
		t.Fatalf("page edit --json: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("page edit --json output is not valid JSON: %v\n%s", err, out)
	}
	if got["id"] != "10" {
		t.Fatalf("unexpected page edit JSON: %v", got)
	}
}

func TestPageEditRequiresAChange(t *testing.T) {
	_, err := execConf(t, "page", "edit", "10", "--site", "work")
	if err == nil {
		t.Fatal("page edit with no --title or --body returned no error")
	}
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Code != apperr.CodeInvalidInput {
		t.Fatalf("error = %v, want an invalid_input *apperr.Error", err)
	}
}

func TestPageEditBodyRequiresFormat(t *testing.T) {
	_, err := execConf(t, "page", "edit", "10", "--body", "<p>x</p>", "--site", "work")
	if err == nil {
		t.Fatal("page edit --body without --body-format returned no error")
	}
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Code != apperr.CodeInvalidInput {
		t.Fatalf("error = %v, want an invalid_input *apperr.Error", err)
	}
}

func TestPageEditBodyFormatRequiresBody(t *testing.T) {
	_, err := execConf(t, "page", "edit", "10", "--body-format", "storage", "--site", "work")
	if err == nil {
		t.Fatal("page edit --body-format without --body returned no error")
	}
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Code != apperr.CodeInvalidInput {
		t.Fatalf("error = %v, want an invalid_input *apperr.Error", err)
	}
}

func TestPageEditTitleOnlyRejectsEmptyBody(t *testing.T) {
	putCalled := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			// A page whose GET response carries no storage body.
			_, _ = w.Write([]byte(`{"id":"10","title":"Old","status":"current","version":{"number":3}}`))
		case http.MethodPut:
			putCalled = true
			_, _ = w.Write([]byte(`{"id":"10"}`))
		default:
			t.Errorf("method = %q, want GET or PUT", r.Method)
		}
	}))
	defer srv.Close()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginConfSite(t, srv.URL)

	_, err := execConf(t, "page", "edit", "10", "--title", "New", "--site", "work")
	if err == nil {
		t.Fatal("title-only edit of a page with no storage body returned no error")
	}
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Code != apperr.CodeInvalidInput {
		t.Fatalf("error = %v, want an invalid_input *apperr.Error", err)
	}
	if putCalled {
		t.Error("title-only edit issued a PUT despite having no body to preserve")
	}
}

func TestPageEditMapsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			_, _ = w.Write([]byte(`{"id":"10","title":"Old","status":"current","version":{"number":1},` +
				`"body":{"storage":{"representation":"storage","value":"<p>x</p>"}}}`))
		case http.MethodPut:
			w.WriteHeader(http.StatusConflict)
			_, _ = w.Write([]byte(`{"message":"Version conflict"}`))
		default:
			t.Errorf("method = %q, want GET or PUT", r.Method)
		}
	}))
	defer srv.Close()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginConfSite(t, srv.URL)

	_, err := execConf(t, "page", "edit", "10", "--title", "New", "--site", "work")
	if err == nil {
		t.Fatal("page edit against a 409 endpoint returned no error")
	}
	var ae *apperr.Error
	if !errors.As(err, &ae) {
		t.Fatalf("error type = %T, want *apperr.Error", err)
	}
}

func TestPageEditRequiresExactlyOneArg(t *testing.T) {
	if _, err := execConf(t, "page", "edit", "--title", "X", "--site", "work"); err == nil {
		t.Fatal("page edit with no id returned no error")
	}
}
