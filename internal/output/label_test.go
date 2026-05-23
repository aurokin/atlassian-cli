package output

import (
	"bytes"
	"strings"
	"testing"
)

func TestLabelWriterAlignsToWidestLabel(t *testing.T) {
	var buf bytes.Buffer
	lw := NewLabelWriter(&buf)
	lw.Add("id", "1")
	lw.Add("description", "a widget")
	if err := lw.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	// "description:" is 12 chars; both value columns must start at the same
	// offset (13: 12 + one space).
	got := buf.String()
	lines := strings.Split(strings.TrimRight(got, "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("got %d lines, want 2:\n%s", len(lines), got)
	}
	idCol := strings.Index(lines[0], "1")
	descCol := strings.Index(lines[1], "a widget")
	if idCol != descCol {
		t.Fatalf("values not aligned: id value at %d, description value at %d\n%s", idCol, descCol, got)
	}
	if !strings.HasPrefix(lines[0], "id:") || !strings.HasPrefix(lines[1], "description:") {
		t.Fatalf("labels missing colon suffix:\n%s", got)
	}
}

func TestLabelWriterAddIfSkipsEmpty(t *testing.T) {
	var buf bytes.Buffer
	lw := NewLabelWriter(&buf)
	lw.Add("key", "PROJ-1")
	lw.AddIf("status", "")
	lw.AddIf("type", "Bug")
	if err := lw.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	got := buf.String()
	if strings.Contains(got, "status") {
		t.Fatalf("empty AddIf row was written:\n%s", got)
	}
	if !strings.Contains(got, "key:") || !strings.Contains(got, "type:") {
		t.Fatalf("expected key and type rows:\n%s", got)
	}
}

func TestLabelWriterAddf(t *testing.T) {
	var buf bytes.Buffer
	lw := NewLabelWriter(&buf)
	lw.Addf("build", "#%d", 42)
	if err := lw.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	if got := buf.String(); got != "build: #42\n" {
		t.Fatalf("Addf output = %q, want %q", got, "build: #42\n")
	}
}

func TestLabelWriterFlushEmptyWritesNothing(t *testing.T) {
	var buf bytes.Buffer
	lw := NewLabelWriter(&buf)
	if err := lw.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	if buf.Len() != 0 {
		t.Fatalf("empty Flush wrote %q", buf.String())
	}
}

func TestLabelWriterFlushResetsBuffer(t *testing.T) {
	var buf bytes.Buffer
	lw := NewLabelWriter(&buf)
	lw.Add("a", "1")
	_ = lw.Flush()
	buf.Reset()
	// A second Flush after the first must not re-emit the first batch.
	if err := lw.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	if buf.Len() != 0 {
		t.Fatalf("second Flush re-emitted rows: %q", buf.String())
	}
}
