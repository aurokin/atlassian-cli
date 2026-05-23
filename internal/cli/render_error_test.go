package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/aurokin/atlassian-cli/internal/apperr"
)

// TestRenderErrorStructuredOutput verifies that the machine-readable error
// envelope is emitted whenever structured output is selected — by --json OR by
// --jq — and that plain human output is used otherwise.
func TestRenderErrorStructuredOutput(t *testing.T) {
	ae := apperr.Unauthorized("bad token")

	cases := []struct {
		name     string
		g        *GlobalFlags
		wantJSON bool
	}{
		{"human", &GlobalFlags{}, false},
		{"json-all", &GlobalFlags{JSON: "*"}, true},
		{"json-fields", &GlobalFlags{JSON: "error,message"}, true},
		{"jq-only", &GlobalFlags{JQ: ".error"}, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			renderError(&buf, tc.g, ae)
			out := buf.String()

			var decoded apperr.Error
			isJSON := json.Unmarshal([]byte(out), &decoded) == nil

			if tc.wantJSON {
				if !isJSON {
					t.Fatalf("expected JSON envelope, got plain text: %q", out)
				}
				if decoded.Code != apperr.CodeUnauthorized {
					t.Fatalf("envelope code = %q, want %q", decoded.Code, apperr.CodeUnauthorized)
				}
			} else {
				if isJSON {
					t.Fatalf("expected plain text, got JSON: %q", out)
				}
				if !strings.HasPrefix(out, "Error:") {
					t.Fatalf("plain output = %q, want an \"Error:\" line", out)
				}
			}
		})
	}
}
