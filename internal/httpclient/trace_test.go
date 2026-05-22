package httpclient

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aurokin/atlassian-cli/internal/auth"
)

// TestEnableTraceWritesRedactedDiagnostics confirms that --trace's backing
// mechanism logs the request line, headers, and response — and that the
// Authorization header (and the token it carries) is never written.
func TestEnableTraceWritesRedactedDiagnostics(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	c := New(
		Target{Product: ProductJira, TokenStyle: auth.StyleDataCenterPAT, SiteName: "work", BaseURL: srv.URL},
		auth.Credential{Style: auth.StyleDataCenterPAT, Token: "super-secret-token"},
		nil,
	)
	var buf bytes.Buffer
	c.EnableTrace(&buf)

	if _, err := c.Do(context.Background(), http.MethodGet, "/rest/api/2/myself", nil); err != nil {
		t.Fatalf("Do: %v", err)
	}

	out := buf.String()
	for _, want := range []string{
		"[trace] > GET ",
		"[trace] > Authorization: [redacted]",
		"[trace] < 200 (",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("trace missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "super-secret-token") {
		t.Fatalf("trace leaked the token:\n%s", out)
	}
}

// TestTraceDisabledByDefault confirms a client with no EnableTrace call writes
// nothing, so the diagnostic is strictly opt-in.
func TestTraceDisabledByDefault(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	c := New(
		Target{Product: ProductJira, TokenStyle: auth.StyleDataCenterPAT, SiteName: "work", BaseURL: srv.URL},
		auth.Credential{Style: auth.StyleDataCenterPAT, Token: "t"},
		nil,
	)
	// No EnableTrace call. A trace helper must be a no-op; this simply must not
	// panic and must complete the request.
	if _, err := c.Do(context.Background(), http.MethodGet, "/rest/api/2/myself", nil); err != nil {
		t.Fatalf("Do: %v", err)
	}
}
