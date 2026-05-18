// Package output renders command results as human-readable text or JSON. It
// backs the global --json and --jq flags so every atl-* command renders
// results the same way.
package output

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

// Options controls how a value is rendered. It mirrors the global --json and
// --jq flags.
type Options struct {
	// JSON selects JSON rendering: "" means human output, "*" means the full
	// value, and any other value is a comma-separated list of top-level fields.
	JSON string
	// JQ is a jq-style filter expression. Phase 1 does not implement it.
	JQ string
}

// ErrJQNotImplemented is returned when --jq is requested. jq-style filtering
// is a documented Phase 1 stub; it will be implemented once the dependency
// trade-off is settled.
var ErrJQNotImplemented = errors.New("output: --jq filtering is not yet implemented")

// errNotObject is returned when field selection is requested for a value that
// does not serialize to a JSON object.
var errNotObject = errors.New("output: field selection requires a JSON object")

// Render writes v to w according to opts. With no JSON option it writes a
// minimal human representation; with --json it writes JSON, optionally
// narrowed to selected top-level fields.
func Render(w io.Writer, v any, opts Options) error {
	if opts.JQ != "" {
		return ErrJQNotImplemented
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
// order they were requested in. Unknown fields are omitted silently.
func renderSelected(w io.Writer, v any, fields []string) error {
	raw, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("output: marshal JSON: %w", err)
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return errNotObject
	}

	var compact bytes.Buffer
	compact.WriteByte('{')
	first := true
	for _, f := range fields {
		val, ok := obj[f]
		if !ok {
			continue // unknown selected fields are omitted
		}
		if !first {
			compact.WriteByte(',')
		}
		first = false
		key, _ := json.Marshal(f)
		compact.Write(key)
		compact.WriteByte(':')
		compact.Write(val)
	}
	compact.WriteByte('}')

	var pretty bytes.Buffer
	if err := json.Indent(&pretty, compact.Bytes(), "", "  "); err != nil {
		return fmt.Errorf("output: indent JSON: %w", err)
	}
	_, err = fmt.Fprintln(w, pretty.String())
	return err
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
