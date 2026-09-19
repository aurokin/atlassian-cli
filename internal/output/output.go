// Package output renders command results as human-readable text or JSON. It
// backs the global --json and --jq flags so every atl-* command renders
// results the same way.
package output

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/itchyny/gojq"

	"github.com/aurokin/atlassian-cli/internal/apperr"
)

// Options controls how a value is rendered. It mirrors the global --json and
// --jq flags.
type Options struct {
	// JSON selects JSON rendering: "" means human output, "*" means the full
	// value, and any other value is a comma-separated list of top-level fields.
	JSON string
	// JQ is a jq filter expression. When set it owns the output: the value is
	// filtered through jq instead of rendered as human text or selected JSON.
	JQ string
}

// Render writes v to w according to opts. With --jq the value is filtered
// through a jq expression; otherwise, with no JSON option it writes a minimal
// human representation, and with --json it writes JSON, optionally narrowed to
// selected top-level fields.
func Render(w io.Writer, v any, opts Options) error {
	if opts.JQ != "" {
		// A --json field list and --jq are two different projections of the
		// same data; combining them is ambiguous.
		if opts.JSON != "" && opts.JSON != "*" {
			return apperr.InvalidInput(
				"--jq cannot be combined with a --json field list; use one or the other")
		}
		return renderJQ(w, v, opts.JQ)
	}
	switch opts.JSON {
	case "":
		return renderHuman(w, v)
	case "*":
		return renderJSON(w, v)
	default:
		return renderSelected(w, v, splitFields(opts.JSON))
	}
}

// renderJQ evaluates a jq expression against v and writes each result as
// compact JSON on its own line. A parse, compile, or runtime failure surfaces
// as a structured invalid_input error, since each reflects a bad expression
// for the given input.
func renderJQ(w io.Writer, v any, expr string) error {
	query, err := gojq.Parse(expr)
	if err != nil {
		return apperr.InvalidInput("invalid --jq expression: " + err.Error())
	}
	code, err := gojq.Compile(query)
	if err != nil {
		return apperr.InvalidInput("invalid --jq expression: " + err.Error())
	}
	input, err := toJSONValue(v)
	if err != nil {
		return err
	}
	iter := code.Run(input)
	for {
		result, ok := iter.Next()
		if !ok {
			break
		}
		if resErr, ok := result.(error); ok {
			return apperr.InvalidInput("--jq filter failed: " + resErr.Error())
		}
		// gojq.Marshal handles the value types gojq emits (including big
		// integers) and formats numbers the way jq does.
		line, err := gojq.Marshal(result)
		if err != nil {
			return fmt.Errorf("output: marshal jq result: %w", err)
		}
		if _, err := fmt.Fprintln(w, string(line)); err != nil {
			return err
		}
	}
	return nil
}

// toJSONValue round-trips v through JSON into a generic value, so gojq always
// receives normalized map[string]any / []any / scalar input regardless of
// whether the caller passed a json.RawMessage or a typed struct.
func toJSONValue(v any) (any, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("output: encode value for --jq: %w", err)
	}
	var out any
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("output: decode value for --jq: %w", err)
	}
	return out, nil
}

// renderHuman writes a minimal human representation. Phase 1 keeps this basic:
// strings are written verbatim, everything else falls back to indented JSON.
func renderHuman(w io.Writer, v any) error {
	if s, ok := v.(string); ok {
		_, err := fmt.Fprintln(w, s)
		return err
	}
	return renderJSON(w, v)
}

// renderJSON writes v as indented JSON followed by a newline.
func renderJSON(w io.Writer, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("output: marshal JSON: %w", err)
	}
	_, err = fmt.Fprintln(w, string(b))
	return err
}

// renderSelected writes only the requested top-level fields, preserving the
// order they were requested in. Unknown fields are omitted silently. An array
// response is projected element by element, so a list command's --json=id
// yields an array of {"id"} objects.
func renderSelected(w io.Writer, v any, fields []string) error {
	raw, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("output: marshal JSON: %w", err)
	}
	keys := make([][]byte, len(fields))
	for i, f := range fields {
		keys[i], _ = json.Marshal(f)
	}
	var compact bytes.Buffer
	if err := selectFields(&compact, raw, fields, keys); err != nil {
		return err
	}
	var pretty bytes.Buffer
	if err := json.Indent(&pretty, compact.Bytes(), "", "  "); err != nil {
		return fmt.Errorf("output: indent JSON: %w", err)
	}
	_, err = fmt.Fprintln(w, pretty.String())
	return err
}

// selectFields appends the projection of an object, or of each element of an
// array of objects, to out as compact JSON. keys holds the fields already
// JSON-encoded, so they are marshalled once rather than per element.
func selectFields(out *bytes.Buffer, raw json.RawMessage, fields []string, keys [][]byte) error {
	if len(raw) == 0 || raw[0] != '[' {
		return selectObjectFields(out, raw, fields, keys)
	}
	var items []json.RawMessage
	if err := json.Unmarshal(raw, &items); err != nil {
		return errNotSelectable()
	}
	out.WriteByte('[')
	for i, item := range items {
		if i > 0 {
			out.WriteByte(',')
		}
		if err := selectObjectFields(out, item, fields, keys); err != nil {
			return err
		}
	}
	out.WriteByte(']')
	return nil
}

// selectObjectFields appends the projection of a single JSON object to out,
// keeping the requested order and skipping unknown names.
func selectObjectFields(out *bytes.Buffer, raw json.RawMessage, fields []string, keys [][]byte) error {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil || obj == nil {
		return errNotSelectable() // a scalar, a nested array, or null
	}
	out.WriteByte('{')
	first := true
	for i, f := range fields {
		val, ok := obj[f]
		if !ok {
			continue // unknown selected fields are omitted
		}
		if !first {
			out.WriteByte(',')
		}
		first = false
		out.Write(keys[i])
		out.WriteByte(':')
		out.Write(val)
	}
	out.WriteByte('}')
	return nil
}

// errNotSelectable reports a response with no fields to select: a scalar,
// null, or an array holding non-objects. Like a failing --jq filter, this is
// the caller's projection not fitting the response, so it shares the
// invalid_input category even though the request has already been made.
func errNotSelectable() error {
	return apperr.InvalidInput(
		"--json field selection needs an object or an array of objects, but this response is neither; use --jq to project it, such as --jq '.[].id'")
}

// splitFields parses a comma-separated field list, trimming spaces and
// dropping empty entries.
func splitFields(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// TabWriter returns a tabwriter configured uniformly for every atl-* command's
// aligned, column-separated list output.
func TabWriter(w io.Writer) *tabwriter.Writer {
	return tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
}
