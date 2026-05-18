package cli

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/aurokin/atlassian-cli/internal/apperr"
	"github.com/aurokin/atlassian-cli/internal/appinfo"
)

func TestResolveCommandHumanOutput(t *testing.T) {
	cases := []struct {
		name  string
		info  appinfo.Info
		input string
		want  []string
	}{
		{"jira issue", jiraInfo(), "PROJ-123", []string{"jira_issue", "PROJ-123"}},
		{"jira project", jiraInfo(), "PROJ", []string{"jira_project", "PROJ"}},
		{"jira browse URL", jiraInfo(), "https://x.atlassian.net/browse/PROJ-7", []string{"jira_issue", "PROJ-7", "x.atlassian.net"}},
		{"confluence page URL", confInfo(), "https://x.atlassian.net/wiki/spaces/DEV/pages/789/Getting+Started", []string{"confluence_page", "789", "DEV", "Getting Started"}},
		{"confluence space URL", confInfo(), "https://x.atlassian.net/wiki/spaces/DEV", []string{"confluence_space", "DEV"}},
		{"confluence bare id", confInfo(), "123456", []string{"confluence_page", "123456"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, err := execRoot(t, tc.info, "resolve", tc.input)
			if err != nil {
				t.Fatalf("resolve %q: %v", tc.input, err)
			}
			for _, w := range tc.want {
				if !strings.Contains(out, w) {
					t.Errorf("output missing %q:\n%s", w, out)
				}
			}
		})
	}
}

func TestResolveCommandJSONEnvelope(t *testing.T) {
	out, err := execRoot(t, jiraInfo(), "resolve", "PROJ-123", "--json")
	if err != nil {
		t.Fatalf("resolve --json: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("resolve --json output is not valid JSON: %v\n%s", err, out)
	}
	if got["kind"] != "jira_issue" || got["key"] != "PROJ-123" || got["product"] != "jira" {
		t.Fatalf("unexpected Resource envelope: %v", got)
	}
}

func TestResolveCommandJSONHonorsFieldSelection(t *testing.T) {
	out, err := execRoot(t, jiraInfo(), "resolve", "PROJ-123", "--json=kind")
	if err != nil {
		t.Fatalf("resolve --json=kind: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("resolve --json=kind output is not valid JSON: %v\n%s", err, out)
	}
	if len(got) != 1 || got["kind"] != "jira_issue" {
		t.Fatalf("field selection ignored: %v", got)
	}
}

func TestResolveCommandUnresolvedInputErrors(t *testing.T) {
	_, err := execRoot(t, jiraInfo(), "resolve", "not a key")
	if err == nil {
		t.Fatal("resolve returned no error for an unrecognized input")
	}
	var ae *apperr.Error
	if !errors.As(err, &ae) {
		t.Fatalf("error type = %T, want *apperr.Error", err)
	}
	if ae.Code != "unresolved" {
		t.Errorf("Code = %q, want %q", ae.Code, "unresolved")
	}
}

func TestResolveCommandRequiresExactlyOneArg(t *testing.T) {
	if _, err := execRoot(t, jiraInfo(), "resolve"); err == nil {
		t.Fatal("resolve with no argument returned no error")
	}
}
