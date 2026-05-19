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

func TestIssueLink(t *testing.T) {
	var (
		gotMethod string
		gotBody   map[string]any
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		if r.URL.Path != "/issueLink" {
			t.Errorf("path = %q, want /issueLink", r.URL.Path)
		}
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginJiraSite(t, srv.URL)

	out, err := execJira(t, "issue", "link", "PROJ-1", "PROJ-2", "--type", "Blocks", "--site", "work")
	if err != nil {
		t.Fatalf("issue link: %v", err)
	}
	if gotMethod != http.MethodPost {
		t.Errorf("method = %q, want POST", gotMethod)
	}
	typ, _ := gotBody["type"].(map[string]any)
	if typ["name"] != "Blocks" {
		t.Errorf("link sent type %v, want Blocks", gotBody["type"])
	}
	inward, _ := gotBody["inwardIssue"].(map[string]any)
	outward, _ := gotBody["outwardIssue"].(map[string]any)
	if inward["key"] != "PROJ-1" || outward["key"] != "PROJ-2" {
		t.Errorf("link sent inward/outward = %v/%v, want PROJ-1/PROJ-2",
			gotBody["inwardIssue"], gotBody["outwardIssue"])
	}
	if !strings.Contains(out, "created Blocks link: inward PROJ-1, outward PROJ-2") {
		t.Errorf("link output missing 'created Blocks link: inward PROJ-1, outward PROJ-2':\n%s", out)
	}
}

func TestIssueLinkRequiresType(t *testing.T) {
	_, err := execJira(t, "issue", "link", "PROJ-1", "PROJ-2", "--site", "work")
	if err == nil {
		t.Fatal("issue link without --type returned no error")
	}
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Code != apperr.CodeInvalidInput {
		t.Fatalf("error = %v, want an invalid_input *apperr.Error", err)
	}
}

func TestIssueLinkTypesHumanOutput(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/issueLinkType" {
			t.Errorf("path = %q, want /issueLinkType", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"issueLinkTypes":[` +
			`{"id":"1000","name":"Blocks","inward":"is blocked by","outward":"blocks"},` +
			`{"id":"1001","name":"Cloners","inward":"is cloned by","outward":"clones"}]}`))
	}))
	defer srv.Close()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginJiraSite(t, srv.URL)

	out, err := execJira(t, "issue", "link", "types", "--site", "work")
	if err != nil {
		t.Fatalf("issue link types: %v", err)
	}
	for _, want := range []string{"Blocks", "is blocked by", "blocks", "Cloners", "is cloned by", "clones"} {
		if !strings.Contains(out, want) {
			t.Errorf("link types output missing %q:\n%s", want, out)
		}
	}
}

func TestIssueLinkTypesJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"issueLinkTypes":[{"id":"1000","name":"Blocks"}]}`))
	}))
	defer srv.Close()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginJiraSite(t, srv.URL)

	out, err := execJira(t, "issue", "link", "types", "--site", "work", "--json")
	if err != nil {
		t.Fatalf("issue link types --json: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("link types --json output is not valid JSON: %v\n%s", err, out)
	}
	if _, ok := got["issueLinkTypes"]; !ok {
		t.Fatalf("unexpected link types JSON: %v", got)
	}
}
