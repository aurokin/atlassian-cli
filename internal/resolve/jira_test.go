package resolve

import "testing"

func TestJiraParseRecognizedForms(t *testing.T) {
	cases := []struct {
		name     string
		input    string
		wantKind ResourceKind
		wantKey  string
		wantHost string
	}{
		{"bare issue key", "PROJ-123", KindJiraIssue, "PROJ-123", ""},
		{"bare project key", "PROJ", KindJiraProject, "PROJ", ""},
		{"project key with digits", "ABC1", KindJiraProject, "ABC1", ""},
		{"browse issue URL", "https://x.atlassian.net/browse/PROJ-7", KindJiraIssue, "PROJ-7", "x.atlassian.net"},
		{"browse project URL", "https://x.atlassian.net/browse/PROJ", KindJiraProject, "PROJ", "x.atlassian.net"},
		{"nav project URL", "https://x.atlassian.net/jira/software/projects/PROJ/boards/1", KindJiraProject, "PROJ", "x.atlassian.net"},
		{"nav URL with selectedIssue", "https://x.atlassian.net/jira/software/projects/PROJ/boards/1?selectedIssue=PROJ-9", KindJiraIssue, "PROJ-9", "x.atlassian.net"},
		{"nav URL with issues segment", "https://x.atlassian.net/jira/software/c/projects/PROJ/issues/PROJ-42", KindJiraIssue, "PROJ-42", "x.atlassian.net"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := Jira.Parse(tc.input)
			if !ok {
				t.Fatalf("Parse(%q) did not resolve", tc.input)
			}
			if got.Kind != tc.wantKind {
				t.Errorf("Kind = %q, want %q", got.Kind, tc.wantKind)
			}
			if got.Key != tc.wantKey {
				t.Errorf("Key = %q, want %q", got.Key, tc.wantKey)
			}
			if got.SiteHost != tc.wantHost {
				t.Errorf("SiteHost = %q, want %q", got.SiteHost, tc.wantHost)
			}
			if got.Product != productJira {
				t.Errorf("Product = %q, want %q", got.Product, productJira)
			}
		})
	}
}

func TestJiraParseRejectsUnrecognizedInput(t *testing.T) {
	for _, in := range []string{
		"",
		"proj-1",      // lowercase
		"PROJ-",       // no issue number
		"123",         // numeric (a Confluence id, not Jira)
		"X",           // too short for a project key
		"https://x.atlassian.net/wiki/spaces/DEV", // a Confluence URL
		"https://x.atlassian.net/some/other/path",
		"ftp://x.atlassian.net/browse/PROJ-1", // non-http scheme
	} {
		if got, ok := Jira.Parse(in); ok {
			t.Errorf("Parse(%q) unexpectedly resolved to %+v", in, got)
		}
	}
}

func TestJiraCanonicalURL(t *testing.T) {
	issue, _ := Jira.Parse("PROJ-5")
	got, err := Jira.CanonicalURL("https://x.atlassian.net/", issue)
	if err != nil {
		t.Fatalf("CanonicalURL: %v", err)
	}
	if got != "https://x.atlassian.net/browse/PROJ-5" {
		t.Errorf("issue CanonicalURL = %q", got)
	}

	project, _ := Jira.Parse("PROJ")
	got, err = Jira.CanonicalURL("https://x.atlassian.net", project)
	if err != nil {
		t.Fatalf("CanonicalURL: %v", err)
	}
	if got != "https://x.atlassian.net/browse/PROJ" {
		t.Errorf("project CanonicalURL = %q", got)
	}
}

func TestJiraCanonicalURLWithoutKeyErrors(t *testing.T) {
	if _, err := Jira.CanonicalURL("https://x.atlassian.net", Resource{Kind: KindJiraIssue}); err == nil {
		t.Fatal("CanonicalURL without a key returned no error")
	}
}
