package resolve

import "testing"

func TestBitbucketParseRecognizedForms(t *testing.T) {
	cases := []struct {
		name     string
		input    string
		wantKind ResourceKind
		wantKey  string
		wantID   string
		wantHost string
	}{
		{"bare repo", "acme/widgets", KindBitbucketRepo, "acme/widgets", "", ""},
		{"repo URL", "https://bitbucket.org/acme/widgets", KindBitbucketRepo, "acme/widgets", "", "bitbucket.org"},
		{"repo URL trailing slash", "https://bitbucket.org/acme/widgets/", KindBitbucketRepo, "acme/widgets", "", "bitbucket.org"},
		{"pull request URL", "https://bitbucket.org/acme/widgets/pull-requests/42", KindBitbucketPullRequest, "acme/widgets", "42", "bitbucket.org"},
		{"issue URL", "https://bitbucket.org/acme/widgets/issues/7", KindBitbucketIssue, "acme/widgets", "7", "bitbucket.org"},
		{"commit URL", "https://bitbucket.org/acme/widgets/commits/abc123", KindBitbucketCommit, "acme/widgets", "abc123", "bitbucket.org"},
		{"src sub-page falls back to repo", "https://bitbucket.org/acme/widgets/src/main/README.md", KindBitbucketRepo, "acme/widgets", "", "bitbucket.org"},
		{"pull-requests list falls back to repo", "https://bitbucket.org/acme/widgets/pull-requests", KindBitbucketRepo, "acme/widgets", "", "bitbucket.org"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := Bitbucket.Parse(tc.input)
			if !ok {
				t.Fatalf("Parse(%q) did not resolve", tc.input)
			}
			if got.Kind != tc.wantKind {
				t.Errorf("Kind = %q, want %q", got.Kind, tc.wantKind)
			}
			if got.Key != tc.wantKey {
				t.Errorf("Key = %q, want %q", got.Key, tc.wantKey)
			}
			if got.ID != tc.wantID {
				t.Errorf("ID = %q, want %q", got.ID, tc.wantID)
			}
			if got.SiteHost != tc.wantHost {
				t.Errorf("SiteHost = %q, want %q", got.SiteHost, tc.wantHost)
			}
			if got.Product != productBitbucket {
				t.Errorf("Product = %q, want %q", got.Product, productBitbucket)
			}
		})
	}
}

func TestBitbucketParseRejectsUnrecognizedInput(t *testing.T) {
	for _, in := range []string{
		"",
		"acme",                          // single segment, no repo
		"acme/widgets/extra",            // not a bare repo (three segments)
		"/acme/widgets",                 // not http(s)
		"ftp://bitbucket.org/acme/repo", // wrong scheme
		"https://bitbucket.org",         // no repo path
		"https://bitbucket.org/acme",    // workspace only
	} {
		if _, ok := Bitbucket.Parse(in); ok {
			t.Errorf("Parse(%q) unexpectedly resolved", in)
		}
	}
}

func TestBitbucketCanonicalURL(t *testing.T) {
	cases := []struct {
		name string
		base string
		res  Resource
		want string
	}{
		{
			"repo from bare key uses API base mapped to web host",
			"https://api.bitbucket.org/2.0",
			Resource{Kind: KindBitbucketRepo, Key: "acme/widgets"},
			"https://bitbucket.org/acme/widgets",
		},
		{
			"pull request from URL host",
			"https://bitbucket.org",
			Resource{Kind: KindBitbucketPullRequest, Key: "acme/widgets", ID: "42"},
			"https://bitbucket.org/acme/widgets/pull-requests/42",
		},
		{
			"issue",
			"https://bitbucket.org",
			Resource{Kind: KindBitbucketIssue, Key: "acme/widgets", ID: "7"},
			"https://bitbucket.org/acme/widgets/issues/7",
		},
		{
			"commit",
			"https://bitbucket.org",
			Resource{Kind: KindBitbucketCommit, Key: "acme/widgets", ID: "abc123"},
			"https://bitbucket.org/acme/widgets/commits/abc123",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Bitbucket.CanonicalURL(tc.base, tc.res)
			if err != nil {
				t.Fatalf("CanonicalURL: %v", err)
			}
			if got != tc.want {
				t.Errorf("CanonicalURL = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestBitbucketCanonicalURLRequiresFields(t *testing.T) {
	if _, err := Bitbucket.CanonicalURL("https://bitbucket.org", Resource{Kind: KindBitbucketRepo}); err == nil {
		t.Error("expected error for a missing workspace/repo key")
	}
	if _, err := Bitbucket.CanonicalURL("https://bitbucket.org",
		Resource{Kind: KindBitbucketPullRequest, Key: "acme/widgets"}); err == nil {
		t.Error("expected error for a pull request without an id")
	}
}

func TestParserForBitbucket(t *testing.T) {
	p, err := ParserFor(productBitbucket)
	if err != nil {
		t.Fatalf("ParserFor: %v", err)
	}
	if _, ok := p.(bitbucketParser); !ok {
		t.Fatalf("ParserFor returned %T, want bitbucketParser", p)
	}
}
