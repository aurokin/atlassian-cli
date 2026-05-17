package apperr

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestErrorJSONShapeMatchesAccessErrorModel(t *testing.T) {
	// Mirrors the example envelope in docs/access-error-model.md.
	e := &Error{
		Code:               CodeForbidden,
		Message:            "The authenticated account cannot edit this Confluence page.",
		Site:               "work",
		TokenStyle:         "cloud-scoped",
		APIBaseURL:         "https://api.atlassian.com/ex/confluence/cloud-id",
		RequiredScope:      "write:page:confluence",
		RequiredPermission: "page edit permission",
		Next:               "Ask a space admin for edit access.",
	}
	b, err := json.Marshal(e)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	// The machine-readable code serializes under the "error" key.
	if got["error"] != CodeForbidden {
		t.Errorf("error = %v, want %v", got["error"], CodeForbidden)
	}
	for _, key := range []string{"message", "site", "token_style", "api_base_url", "required_scope", "required_permission", "next"} {
		if _, ok := got[key]; !ok {
			t.Errorf("missing JSON key %q", key)
		}
	}
}

func TestErrorOmitsEmptyOptionalFields(t *testing.T) {
	b, err := json.Marshal(New(CodeInvalidInput, "bad input"))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	for _, key := range []string{"status", "site", "token_style", "required_scope"} {
		if _, ok := got[key]; ok {
			t.Errorf("empty optional field %q should be omitted", key)
		}
	}
}

func TestErrorStringIncludesCodeAndMessage(t *testing.T) {
	s := Unauthorized("token expired").Error()
	if !strings.Contains(s, CodeUnauthorized) || !strings.Contains(s, "token expired") {
		t.Fatalf("Error() = %q, want code and message", s)
	}
}

func TestStatusHelpers(t *testing.T) {
	cases := []struct {
		name     string
		err      *Error
		wantCode string
		wantStat int
	}{
		{"unauthorized", Unauthorized("m"), CodeUnauthorized, 401},
		{"forbidden", Forbidden("m"), CodeForbidden, 403},
		{"not found", NotFoundOrNotVisible("m"), CodeNotFoundOrNotVisible, 404},
		{"rate limited", RateLimited("m"), CodeRateLimited, 429},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if c.err.Code != c.wantCode {
				t.Errorf("Code = %q, want %q", c.err.Code, c.wantCode)
			}
			if c.err.Status != c.wantStat {
				t.Errorf("Status = %d, want %d", c.err.Status, c.wantStat)
			}
		})
	}
}

func TestErrorSatisfiesErrorInterface(t *testing.T) {
	var err error = Forbidden("denied")
	if err.Error() == "" {
		t.Fatal("Error() returned empty string")
	}
}
