package jiracmd

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestIssueAssign(t *testing.T) {
	var (
		gotMethod string
		gotBody   map[string]any
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		if r.URL.Path != "/issue/PROJ-1/assignee" {
			t.Errorf("path = %q, want /issue/PROJ-1/assignee", r.URL.Path)
		}
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginJiraSite(t, srv.URL)

	out, err := execJira(t, "issue", "assign", "PROJ-1", "abc123", "--site", "work")
	if err != nil {
		t.Fatalf("issue assign: %v", err)
	}
	if gotMethod != http.MethodPut {
		t.Errorf("method = %q, want PUT", gotMethod)
	}
	if gotBody["accountId"] != "abc123" {
		t.Errorf("assign sent accountId %v, want abc123", gotBody["accountId"])
	}
	if !strings.Contains(out, "assigned PROJ-1 to abc123") {
		t.Errorf("assign output missing 'assigned PROJ-1 to abc123':\n%s", out)
	}
}

func TestIssueAssignUnassign(t *testing.T) {
	var rawBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rawBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginJiraSite(t, srv.URL)

	out, err := execJira(t, "issue", "assign", "PROJ-1", "-", "--site", "work")
	if err != nil {
		t.Fatalf("issue assign -: %v", err)
	}
	// The raw body must carry a JSON null, not the literal string "-" or an
	// omitted key. Decode it and inspect the value's JSON form.
	var got map[string]json.RawMessage
	if err := json.Unmarshal(rawBody, &got); err != nil {
		t.Fatalf("assign body is not JSON: %v\n%s", err, rawBody)
	}
	v, ok := got["accountId"]
	if !ok {
		t.Fatalf("assign body missing accountId key: %s", rawBody)
	}
	if string(v) != "null" {
		t.Errorf("assign body accountId = %s, want null", v)
	}
	if !strings.Contains(out, "unassigned PROJ-1") {
		t.Errorf("unassign output missing 'unassigned PROJ-1':\n%s", out)
	}
}
