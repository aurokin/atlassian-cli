package output

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

type sample struct {
	Binary  string `json:"binary"`
	Product string `json:"product"`
	Version string `json:"version"`
}

func TestRenderFullJSONIsValidAndComplete(t *testing.T) {
	var buf bytes.Buffer
	in := sample{Binary: "atl-jira", Product: "jira", Version: "1.0.0"}
	if err := Render(&buf, in, Options{JSON: "*"}); err != nil {
		t.Fatalf("Render: %v", err)
	}
	var got sample
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, buf.String())
	}
	if got != in {
		t.Fatalf("round trip = %+v, want %+v", got, in)
	}
}

func TestRenderSelectedFieldsRendersOnlyRequested(t *testing.T) {
	var buf bytes.Buffer
	in := sample{Binary: "atl-conf", Product: "confluence", Version: "2.0.0"}
	if err := Render(&buf, in, Options{JSON: "binary,version"}); err != nil {
		t.Fatalf("Render: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, buf.String())
	}
	if len(got) != 2 {
		t.Fatalf("got %d fields, want 2: %v", len(got), got)
	}
	if got["binary"] != "atl-conf" || got["version"] != "2.0.0" {
		t.Errorf("unexpected fields: %v", got)
	}
	if _, ok := got["product"]; ok {
		t.Errorf("unselected field 'product' was rendered")
	}
}

func TestRenderSelectedFieldsPreservesRequestedOrder(t *testing.T) {
	var buf bytes.Buffer
	in := sample{Binary: "atl-jira", Product: "jira", Version: "1.0.0"}
	if err := Render(&buf, in, Options{JSON: "version,binary"}); err != nil {
		t.Fatalf("Render: %v", err)
	}
	out := buf.String()
	vi, bi := strings.Index(out, "version"), strings.Index(out, "binary")
	if vi < 0 || bi < 0 {
		t.Fatalf("expected both selected fields to be present:\n%s", out)
	}
	if vi > bi {
		t.Fatalf("fields not in requested order:\n%s", out)
	}
}

func TestRenderSelectedFieldsOmitsUnknownFields(t *testing.T) {
	var buf bytes.Buffer
	in := sample{Binary: "atl-jira", Product: "jira", Version: "1.0.0"}
	if err := Render(&buf, in, Options{JSON: "binary,nonexistent"}); err != nil {
		t.Fatalf("Render: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, buf.String())
	}
	if len(got) != 1 || got["binary"] != "atl-jira" {
		t.Fatalf("unknown field not omitted cleanly: %v", got)
	}
}

func TestRenderJQReturnsNotImplemented(t *testing.T) {
	var buf bytes.Buffer
	err := Render(&buf, sample{}, Options{JQ: ".binary"})
	if !errors.Is(err, ErrJQNotImplemented) {
		t.Fatalf("err = %v, want ErrJQNotImplemented", err)
	}
}

func TestRenderHumanStringIsVerbatim(t *testing.T) {
	var buf bytes.Buffer
	if err := Render(&buf, "hello", Options{}); err != nil {
		t.Fatalf("Render: %v", err)
	}
	if buf.String() != "hello\n" {
		t.Fatalf("human output = %q, want %q", buf.String(), "hello\n")
	}
}
