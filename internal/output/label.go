package output

import (
	"fmt"
	"io"
	"strings"
)

// LabelWriter renders aligned "label: value" detail rows — the compact
// human-output format every view command uses. Rows are buffered so the label
// column can be sized to the widest label at Flush time, giving consistent
// alignment across products without per-command hardcoded padding widths.
//
// Labels are supplied without their trailing colon; LabelWriter appends it and
// pads the column. The typical use is:
//
//	lw := output.NewLabelWriter(w)
//	lw.Add("key", iss.Key)
//	lw.AddIf("status", status)   // skipped when status == ""
//	lw.Addf("version", "%d", n)
//	lw.Flush()
type LabelWriter struct {
	w    io.Writer
	rows []labelRow
}

type labelRow struct {
	label string
	value string
}

// NewLabelWriter returns a LabelWriter that writes flushed rows to w.
func NewLabelWriter(w io.Writer) *LabelWriter {
	return &LabelWriter{w: w}
}

// Add buffers a row with the given label and value.
func (lw *LabelWriter) Add(label, value string) {
	lw.rows = append(lw.rows, labelRow{label: label, value: value})
}

// Addf buffers a row whose value is formatted from format and args.
func (lw *LabelWriter) Addf(label, format string, args ...any) {
	lw.Add(label, fmt.Sprintf(format, args...))
}

// AddIf buffers a row only when value is non-empty, matching the common
// "omit absent fields" pattern in view commands.
func (lw *LabelWriter) AddIf(label, value string) {
	if value != "" {
		lw.Add(label, value)
	}
}

// Flush writes every buffered row, aligning values to the widest label, then
// clears the buffer. Each label is suffixed with ":" and the label column is
// padded so the values line up. Flushing with no rows writes nothing.
func (lw *LabelWriter) Flush() error {
	width := 0
	for _, r := range lw.rows {
		if n := len(r.label) + 1; n > width { // +1 for the ":"
			width = n
		}
	}
	var b strings.Builder
	for _, r := range lw.rows {
		fmt.Fprintf(&b, "%-*s %s\n", width, r.label+":", r.value)
	}
	lw.rows = nil
	_, err := io.WriteString(lw.w, b.String())
	return err
}
