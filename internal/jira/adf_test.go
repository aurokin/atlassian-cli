package jira

import (
	"encoding/json"
	"testing"
)

func TestTextOfADFDocument(t *testing.T) {
	doc := json.RawMessage(`{"type":"doc","content":[` +
		`{"type":"paragraph","content":[{"type":"text","text":"Hello "},{"type":"text","text":"world"}]},` +
		`{"type":"paragraph","content":[{"type":"text","text":"Second line"}]}]}`)
	if got := TextOf(doc); got != "Hello world\nSecond line" {
		t.Errorf("TextOf = %q, want %q", got, "Hello world\nSecond line")
	}
}

func TestTextOfHandlesHardBreak(t *testing.T) {
	doc := json.RawMessage(`{"type":"doc","content":[{"type":"paragraph","content":[` +
		`{"type":"text","text":"line one"},{"type":"hardBreak"},{"type":"text","text":"line two"}]}]}`)
	if got := TextOf(doc); got != "line one\nline two" {
		t.Errorf("TextOf = %q, want %q", got, "line one\nline two")
	}
}

func TestTextOfPlainString(t *testing.T) {
	if got := TextOf(json.RawMessage(`"already plain"`)); got != "already plain" {
		t.Errorf("TextOf = %q, want %q", got, "already plain")
	}
}

func TestTextOfEmptyOrUnparseable(t *testing.T) {
	for _, in := range []string{"", "12345", "not json"} {
		if got := TextOf(json.RawMessage(in)); got != "" {
			t.Errorf("TextOf(%q) = %q, want empty", in, got)
		}
	}
}

func TestTextOfNestedList(t *testing.T) {
	doc := json.RawMessage(`{"type":"doc","content":[{"type":"bulletList","content":[` +
		`{"type":"listItem","content":[{"type":"paragraph","content":[{"type":"text","text":"one"}]}]},` +
		`{"type":"listItem","content":[{"type":"paragraph","content":[{"type":"text","text":"two"}]}]}]}]}`)
	if got := TextOf(doc); got != "one\ntwo" {
		t.Errorf("TextOf = %q, want %q", got, "one\ntwo")
	}
}

func TestTextOfSkipsUnknownNodeTypes(t *testing.T) {
	doc := json.RawMessage(`{"type":"doc","content":[{"type":"paragraph","content":[` +
		`{"type":"text","text":"hi "},{"type":"emoji","attrs":{}},{"type":"text","text":"there"}]}]}`)
	if got := TextOf(doc); got != "hi there" {
		t.Errorf("TextOf = %q, want %q", got, "hi there")
	}
}
