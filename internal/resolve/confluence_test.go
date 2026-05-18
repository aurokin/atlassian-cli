package resolve

import "testing"

func TestConfluenceParseRecognizedForms(t *testing.T) {
	cases := []struct {
		name      string
		input     string
		wantKind  ResourceKind
		wantID    string
		wantKey   string
		wantTitle string
		wantHost  string
	}{
		{"bare page id", "123456", KindConfluencePage, "123456", "", "", ""},
		{"page URL with slug", "https://x.atlassian.net/wiki/spaces/DEV/pages/789/Getting+Started", KindConfluencePage, "789", "DEV", "Getting Started", "x.atlassian.net"},
		{"page URL without slug", "https://x.atlassian.net/wiki/spaces/DEV/pages/789", KindConfluencePage, "789", "DEV", "", "x.atlassian.net"},
		{"space URL", "https://x.atlassian.net/wiki/spaces/DEV", KindConfluenceSpace, "", "DEV", "", "x.atlassian.net"},
		{"space URL with overview", "https://x.atlassian.net/wiki/spaces/DEV/overview", KindConfluenceSpace, "", "DEV", "", "x.atlassian.net"},
		{"personal space URL", "https://x.atlassian.net/wiki/spaces/~admin/overview", KindConfluenceSpace, "", "~admin", "", "x.atlassian.net"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := Confluence.Parse(tc.input)
			if !ok {
				t.Fatalf("Parse(%q) did not resolve", tc.input)
			}
			if got.Kind != tc.wantKind {
				t.Errorf("Kind = %q, want %q", got.Kind, tc.wantKind)
			}
			if got.ID != tc.wantID {
				t.Errorf("ID = %q, want %q", got.ID, tc.wantID)
			}
			if got.Key != tc.wantKey {
				t.Errorf("Key = %q, want %q", got.Key, tc.wantKey)
			}
			if got.Title != tc.wantTitle {
				t.Errorf("Title = %q, want %q", got.Title, tc.wantTitle)
			}
			if got.SiteHost != tc.wantHost {
				t.Errorf("SiteHost = %q, want %q", got.SiteHost, tc.wantHost)
			}
			if got.Product != productConfluence {
				t.Errorf("Product = %q, want %q", got.Product, productConfluence)
			}
		})
	}
}

func TestConfluenceParseRejectsUnrecognizedInput(t *testing.T) {
	for _, in := range []string{
		"",
		"PROJ-123",                              // a Jira issue key
		"PROJ",                                  // a Jira project key
		"abc",                                   // a non-numeric bare token
		"12.3",                                  // not a whole number
		"https://x.atlassian.net/browse/PROJ-1", // a Jira URL
		"https://x.atlassian.net/wiki/spaces",   // no space key
		"https://x.atlassian.net/some/other/path",
		"ftp://x.atlassian.net/wiki/spaces/DEV", // non-http scheme
	} {
		if got, ok := Confluence.Parse(in); ok {
			t.Errorf("Parse(%q) unexpectedly resolved to %+v", in, got)
		}
	}
}

func TestConfluenceCanonicalURL(t *testing.T) {
	// A page with a known space key, base lacking the /wiki segment.
	page, _ := Confluence.Parse("https://x.atlassian.net/wiki/spaces/DEV/pages/789/Title")
	got, err := Confluence.CanonicalURL("https://x.atlassian.net", page)
	if err != nil {
		t.Fatalf("CanonicalURL: %v", err)
	}
	if got != "https://x.atlassian.net/wiki/spaces/DEV/pages/789" {
		t.Errorf("page CanonicalURL = %q", got)
	}

	// A base already carrying /wiki must not double it.
	got, err = Confluence.CanonicalURL("https://x.atlassian.net/wiki/", page)
	if err != nil {
		t.Fatalf("CanonicalURL: %v", err)
	}
	if got != "https://x.atlassian.net/wiki/spaces/DEV/pages/789" {
		t.Errorf("page CanonicalURL (base with /wiki) = %q", got)
	}

	// A bare page id has no space key: fall back to viewpage.action.
	bare, _ := Confluence.Parse("789")
	got, err = Confluence.CanonicalURL("https://x.atlassian.net", bare)
	if err != nil {
		t.Fatalf("CanonicalURL: %v", err)
	}
	if got != "https://x.atlassian.net/wiki/pages/viewpage.action?pageId=789" {
		t.Errorf("bare page CanonicalURL = %q", got)
	}

	// A space.
	space, _ := Confluence.Parse("https://x.atlassian.net/wiki/spaces/DEV")
	got, err = Confluence.CanonicalURL("https://x.atlassian.net", space)
	if err != nil {
		t.Fatalf("CanonicalURL: %v", err)
	}
	if got != "https://x.atlassian.net/wiki/spaces/DEV" {
		t.Errorf("space CanonicalURL = %q", got)
	}
}

func TestConfluenceCanonicalURLMissingFieldsError(t *testing.T) {
	if _, err := Confluence.CanonicalURL("https://x.atlassian.net", Resource{Kind: KindConfluencePage}); err == nil {
		t.Error("page CanonicalURL without an id returned no error")
	}
	if _, err := Confluence.CanonicalURL("https://x.atlassian.net", Resource{Kind: KindConfluenceSpace}); err == nil {
		t.Error("space CanonicalURL without a key returned no error")
	}
}
