package bbcmd

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aurokin/atlassian-cli/internal/apperr"
)

func TestIssueListHumanAndStateQuery(t *testing.T) {
	var gotState string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repositories/acme/widgets/issues" {
			t.Errorf("path = %q", r.URL.Path)
		}
		gotState = r.URL.Query().Get("state")
		_, _ = w.Write([]byte(`{"values":[` +
			`{"id":3,"title":"Crash on save","state":"open","kind":"bug"},` +
			`{"id":1,"title":"Add export","state":"new","kind":"enhancement"}]}`))
	}))
	defer srv.Close()

	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginBBSite(t, srv.URL)

	out, err := execBB(t, "issue", "list", "--repo", "acme/widgets", "--site", "work", "--state", "open")
	if err != nil {
		t.Fatalf("issue list: %v\n%s", err, out)
	}
	// Issue states are lower-case and passed through verbatim (not upper-cased).
	if gotState != "open" {
		t.Fatalf("state query = %q, want open", gotState)
	}
	for _, want := range []string{"#3", "Crash on save", "bug", "#1", "Add export", "enhancement"} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q:\n%s", want, out)
		}
	}
}

func TestIssueListStateAllOmitsFilter(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := r.URL.Query()["state"]; ok {
			t.Errorf("state query should be absent for ALL, got %q", r.URL.Query().Get("state"))
		}
		_, _ = w.Write([]byte(`{"values":[]}`))
	}))
	defer srv.Close()

	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginBBSite(t, srv.URL)

	out, err := execBB(t, "issue", "list", "--repo", "acme/widgets", "--site", "work", "--state", "ALL")
	if err != nil {
		t.Fatalf("issue list: %v\n%s", err, out)
	}
	if !strings.Contains(out, "No issues found.") {
		t.Fatalf("expected empty message:\n%s", out)
	}
}

func TestIssueViewHumanAndJSON(t *testing.T) {
	body := `{"id":3,"title":"Crash on save","state":"open","kind":"bug","priority":"major",` +
		`"reporter":{"display_name":"Auro"},"created_on":"2026-05-20T10:00:00Z",` +
		`"content":{"raw":"steps to repro"}}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repositories/acme/widgets/issues/3" {
			t.Errorf("path = %q", r.URL.Path)
		}
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()

	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginBBSite(t, srv.URL)

	out, err := execBB(t, "issue", "view", "3", "--repo", "acme/widgets", "--site", "work")
	if err != nil {
		t.Fatalf("issue view: %v\n%s", err, out)
	}
	for _, want := range []string{"#3", "Crash on save", "open", "bug", "major", "Auro", "2026-05-20", "steps to repro"} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q:\n%s", want, out)
		}
	}

	jsonOut, err := execBB(t, "issue", "view", "3", "--repo", "acme/widgets", "--site", "work", "--jq", ".kind")
	if err != nil {
		t.Fatalf("issue view --jq: %v\n%s", err, jsonOut)
	}
	if strings.TrimSpace(jsonOut) != `"bug"` {
		t.Fatalf("jq output = %q", jsonOut)
	}
}

func TestIssueViewInvalidID(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	_, err := execBB(t, "issue", "view", "0", "--repo", "acme/widgets", "--site", "work")
	if err == nil || !strings.Contains(err.Error(), "invalid issue id") {
		t.Fatalf("expected invalid id error, got %v", err)
	}
}

func TestIssueCreateSendsBody(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want POST", r.Method)
		}
		if r.URL.Path != "/repositories/acme/widgets/issues" {
			t.Errorf("path = %q", r.URL.Path)
		}
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &gotBody)
		_, _ = w.Write([]byte(`{"id":9,"title":"New bug"}`))
	}))
	defer srv.Close()

	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginBBSite(t, srv.URL)

	out, err := execBB(t, "issue", "create", "--repo", "acme/widgets", "--site", "work",
		"--title", "New bug", "--body", "details", "--kind", "bug", "--priority", "major")
	if err != nil {
		t.Fatalf("issue create: %v\n%s", err, out)
	}
	if !strings.Contains(out, "created issue #9: New bug") {
		t.Fatalf("unexpected output:\n%s", out)
	}
	if gotBody["title"] != "New bug" || gotBody["kind"] != "bug" || gotBody["priority"] != "major" {
		t.Fatalf("body = %+v", gotBody)
	}
	content, _ := gotBody["content"].(map[string]any)
	if content["raw"] != "details" {
		t.Fatalf("content = %+v", gotBody["content"])
	}
}

func TestIssueCreateRequiresTitle(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	_, err := execBB(t, "issue", "create", "--repo", "acme/widgets", "--site", "work")
	if err == nil || !strings.Contains(err.Error(), "a title is required") {
		t.Fatalf("expected title-required error, got %v", err)
	}
}

// TestIssueTrackerDisabledMapsToFeatureDisabled exercises the B3a
// feature_disabled remap end-to-end: a repository whose issue tracker is off
// returns 404 with a recognizable message, which surfaces as feature_disabled.
func TestIssueTrackerDisabledMapsToFeatureDisabled(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"type":"error","error":{"message":"Repository has no issue tracker."}}`))
	}))
	defer srv.Close()

	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginBBSite(t, srv.URL)

	_, err := execBB(t, "issue", "list", "--repo", "acme/widgets", "--site", "work")
	if err == nil {
		t.Fatalf("expected feature_disabled error")
	}
	var ae *apperr.Error
	if !errors.As(err, &ae) {
		t.Fatalf("error is %T, want *apperr.Error", err)
	}
	if ae.Code != apperr.CodeFeatureDisabled {
		t.Fatalf("error code = %q, want feature_disabled", ae.Code)
	}
	if !strings.Contains(ae.Message, "issue tracker") {
		t.Fatalf("error message = %q, want the tracker message", ae.Message)
	}
}

func TestIssueUpdateTransitionsState(t *testing.T) {
	var gotMethod, gotPath string
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath = r.Method, r.URL.Path
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &gotBody)
		_, _ = w.Write([]byte(`{"id":3,"title":"t","state":"resolved"}`))
	}))
	defer srv.Close()

	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginBBSite(t, srv.URL)

	out, err := execBB(t, "issue", "update", "3", "--repo", "acme/widgets", "--site", "work",
		"--state", "resolved")
	if err != nil {
		t.Fatalf("issue update: %v\n%s", err, out)
	}
	if gotMethod != http.MethodPut || gotPath != "/repositories/acme/widgets/issues/3" {
		t.Errorf("request = %s %s", gotMethod, gotPath)
	}
	if gotBody["state"] != "resolved" {
		t.Fatalf("body = %+v", gotBody)
	}
	if !strings.Contains(out, "updated issue #3 (state: resolved)") {
		t.Fatalf("unexpected output:\n%s", out)
	}
}

func TestIssueUpdateRequiresAField(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	// The empty-update guard runs before client construction, so a clean config
	// never reaches the network.
	_, err := execBB(t, "issue", "update", "3", "--repo", "acme/widgets", "--site", "work")
	if err == nil || !strings.Contains(err.Error(), "nothing to update") {
		t.Fatalf("expected nothing-to-update error, got %v", err)
	}
}
