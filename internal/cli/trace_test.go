package cli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestTraceFlagWiresTracingToStderrSeam confirms that --trace routes request
// diagnostics to the trace writer (stderr in production) while stdout stays
// pure JSON, and that the credential never appears in the trace.
func TestTraceFlagWiresTracingToStderrSeam(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"accountId":"abc"}`))
	}))
	defer srv.Close()

	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginDataCenter(t, srv.URL) // arms ATL_API_TOKEN=test-token

	var traceBuf bytes.Buffer
	orig := traceOut
	traceOut = &traceBuf
	t.Cleanup(func() { traceOut = orig })

	out, err := execRoot(t, jiraInfo(), "api", "/rest/api/2/myself", "--site", "work", "--trace", "--json=*")
	if err != nil {
		t.Fatalf("api --trace: %v\n%s", err, out)
	}

	// stdout stays pure JSON — the trace must not bleed into it.
	var got map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("stdout is not pure JSON (trace may have leaked):\n%s", out)
	}
	if strings.Contains(out, "[trace]") {
		t.Fatalf("trace leaked into stdout:\n%s", out)
	}

	tr := traceBuf.String()
	if !strings.Contains(tr, "[trace] > GET") || !strings.Contains(tr, "[trace] < 200") {
		t.Fatalf("trace writer missing request/response lines:\n%s", tr)
	}
	if strings.Contains(tr, "test-token") {
		t.Fatalf("trace leaked the token:\n%s", tr)
	}
}

// TestNoTraceFlagProducesNoTrace confirms tracing is off unless --trace is set.
func TestNoTraceFlagProducesNoTrace(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginDataCenter(t, srv.URL)

	var traceBuf bytes.Buffer
	orig := traceOut
	traceOut = &traceBuf
	t.Cleanup(func() { traceOut = orig })

	if _, err := execRoot(t, jiraInfo(), "api", "/rest/api/2/myself", "--site", "work"); err != nil {
		t.Fatalf("api: %v", err)
	}
	if traceBuf.Len() != 0 {
		t.Fatalf("expected no trace output without --trace, got:\n%s", traceBuf.String())
	}
}
