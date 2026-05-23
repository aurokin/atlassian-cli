package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/aurokin/atlassian-cli/internal/apperr"
)

func TestWantsStructured(t *testing.T) {
	cases := []struct {
		name string
		g    GlobalFlags
		want bool
	}{
		{"neither", GlobalFlags{}, false},
		{"json-all", GlobalFlags{JSON: "*"}, true},
		{"json-fields", GlobalFlags{JSON: "a,b"}, true},
		{"jq-only", GlobalFlags{JQ: ".x"}, true},
		{"both", GlobalFlags{JSON: "*", JQ: ".x"}, true},
		{"site-irrelevant", GlobalFlags{Site: "work"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.g.WantsStructured(); got != tc.want {
				t.Fatalf("WantsStructured() = %v, want %v", got, tc.want)
			}
		})
	}
}

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
