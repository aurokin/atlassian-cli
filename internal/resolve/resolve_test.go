package resolve

import (
	"errors"
	"testing"

	"github.com/aurokin/atlassian-cli/internal/apperr"
)

// fakeParser is a test double for the Parser interface.
type fakeParser struct {
	match  bool
	result Resource
}

func (f fakeParser) Parse(input string) (Resource, bool) {
	if !f.match {
		return Resource{}, false
	}
	r := f.result
	r.Input = input
	return r, true
}

func (f fakeParser) CanonicalURL(baseURL string, r Resource) (string, error) {
	return baseURL + "/" + r.Key, nil
}

func TestResolveReturnsResourceOnMatch(t *testing.T) {
	p := fakeParser{match: true, result: Resource{Kind: KindJiraIssue, Product: productJira, Key: "PROJ-1"}}
	got, err := Resolve(p, "PROJ-1")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got.Kind != KindJiraIssue || got.Key != "PROJ-1" {
		t.Fatalf("Resolve = %+v", got)
	}
}

func TestResolveTrimsInputBeforeParsing(t *testing.T) {
	p := fakeParser{match: true, result: Resource{Kind: KindJiraIssue}}
	got, err := Resolve(p, "  PROJ-1\n")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got.Input != "PROJ-1" {
		t.Fatalf("Input = %q, want trimmed %q", got.Input, "PROJ-1")
	}
}

func TestResolveUnmatchedInputReturnsStructuredError(t *testing.T) {
	_, err := Resolve(fakeParser{match: false}, "nonsense")
	if err == nil {
		t.Fatal("Resolve returned no error for an unrecognized input")
	}
	var ae *apperr.Error
	if !errors.As(err, &ae) {
		t.Fatalf("error type = %T, want *apperr.Error", err)
	}
	if ae.Code != "unresolved" {
		t.Errorf("Code = %q, want %q", ae.Code, "unresolved")
	}
}

func TestResolveEmptyInputReturnsStructuredError(t *testing.T) {
	for _, in := range []string{"", "   ", "\t\n"} {
		_, err := Resolve(fakeParser{match: true}, in)
		var ae *apperr.Error
		if !errors.As(err, &ae) || ae.Code != "unresolved" {
			t.Errorf("Resolve(%q) error = %v, want unresolved", in, err)
		}
	}
}
