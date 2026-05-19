package output

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/aurokin/atlassian-cli/internal/apperr"
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

func TestRenderJQExtractsField(t *testing.T) {
	var buf bytes.Buffer
	in := sample{Binary: "atl-jira", Product: "jira", Version: "1.0.0"}
	if err := Render(&buf, in, Options{JQ: ".product"}); err != nil {
		t.Fatalf("Render: %v", err)
	}
	if got := buf.String(); got != "\"jira\"\n" {
		t.Fatalf("--jq .product = %q, want %q", got, "\"jira\"\n")
	}
}

func TestRenderJQIteratesArrayResults(t *testing.T) {
	var buf bytes.Buffer
	// A json.RawMessage input exercises the path most commands take.
	in := json.RawMessage(`{"results":[{"id":"1"},{"id":"2"}]}`)
	if err := Render(&buf, in, Options{JQ: ".results[].id"}); err != nil {
		t.Fatalf("Render: %v", err)
	}
	if got := buf.String(); got != "\"1\"\n\"2\"\n" {
		t.Fatalf("--jq iteration = %q, want each id on its own line", got)
	}
}

func TestRenderJQComposesPipesAndSelect(t *testing.T) {
	var buf bytes.Buffer
	in := json.RawMessage(`{"issues":[{"key":"A-1","done":true},{"key":"A-2","done":false}]}`)
	if err := Render(&buf, in, Options{JQ: ".issues[] | select(.done) | .key"}); err != nil {
		t.Fatalf("Render: %v", err)
	}
	if got := buf.String(); got != "\"A-1\"\n" {
		t.Fatalf("--jq select = %q, want only the done issue's key", got)
	}
}

func TestRenderJQInvalidExpressionIsStructured(t *testing.T) {
	var buf bytes.Buffer
	err := Render(&buf, sample{}, Options{JQ: "{"})
	if err == nil {
		t.Fatal("Render accepted an invalid --jq expression")
	}
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Code != apperr.CodeInvalidInput {
		t.Fatalf("err = %v, want an invalid_input *apperr.Error", err)
	}
}

func TestRenderJQCompileErrorIsStructured(t *testing.T) {
	var buf bytes.Buffer
	// "$undefined" parses cleanly but fails to compile: the variable is
	// unbound. This exercises the compile-error branch, distinct from parse.
	err := Render(&buf, sample{}, Options{JQ: "$undefined"})
	if err == nil {
		t.Fatal("Render accepted a --jq expression that fails to compile")
	}
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Code != apperr.CodeInvalidInput {
		t.Fatalf("err = %v, want an invalid_input *apperr.Error", err)
	}
}

func TestRenderJQEmptyStreamPrintsNothing(t *testing.T) {
	var buf bytes.Buffer
	in := json.RawMessage(`{"results":[]}`)
	if err := Render(&buf, in, Options{JQ: ".results[]"}); err != nil {
		t.Fatalf("Render: %v", err)
	}
	if got := buf.String(); got != "" {
		t.Fatalf("empty --jq stream printed %q, want nothing", got)
	}
}

func TestRenderJQRuntimeErrorIsStructured(t *testing.T) {
	var buf bytes.Buffer
	// Indexing a string with a field access is a jq runtime type error.
	err := Render(&buf, json.RawMessage(`"plain string"`), Options{JQ: ".field"})
	if err == nil {
		t.Fatal("Render accepted a --jq expression that fails at runtime")
	}
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Code != apperr.CodeInvalidInput {
		t.Fatalf("err = %v, want an invalid_input *apperr.Error", err)
	}
}

func TestRenderJQRejectsJSONFieldListCombo(t *testing.T) {
	var buf bytes.Buffer
	err := Render(&buf, sample{}, Options{JSON: "binary,version", JQ: ".binary"})
	if err == nil {
		t.Fatal("Render accepted --jq combined with a --json field list")
	}
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Code != apperr.CodeInvalidInput {
		t.Fatalf("err = %v, want an invalid_input *apperr.Error", err)
	}
}

func TestRenderJQAllowsBareJSONFlag(t *testing.T) {
	var buf bytes.Buffer
	// Bare --json (the "*" sentinel) with --jq is allowed: --jq owns output.
	if err := Render(&buf, sample{Product: "jira"}, Options{JSON: "*", JQ: ".product"}); err != nil {
		t.Fatalf("Render: %v", err)
	}
	if got := buf.String(); got != "\"jira\"\n" {
		t.Fatalf("--json + --jq = %q, want %q", got, "\"jira\"\n")
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
