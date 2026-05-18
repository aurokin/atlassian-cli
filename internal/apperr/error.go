// Package apperr defines the structured, machine-readable error envelope
// shared across atl-* commands. Its JSON shape follows docs/access-error-model.md.
package apperr

import "fmt"

// Stable machine-readable error codes. The code values match the recovery
// cases in docs/access-error-model.md.
const (
	CodeUnauthorized         = "unauthorized"
	CodeForbidden            = "forbidden"
	CodeNotFoundOrNotVisible = "not_found_or_not_visible"
	CodeRateLimited          = "rate_limited"
	CodeInvalidInput         = "invalid_input"
)

// Error is a structured CLI error. It is safe to render directly as JSON and
// implements the error interface for human display.
//
// The Code field serializes as "error" to match the JSON shape documented in
// docs/access-error-model.md.
type Error struct {
	Code               string `json:"error"`
	Message            string `json:"message"`
	Status             int    `json:"status,omitempty"`
	Product            string `json:"product,omitempty"`
	Site               string `json:"site,omitempty"`
	TokenStyle         string `json:"token_style,omitempty"`
	APIBaseURL         string `json:"api_base_url,omitempty"`
	RequiredScope      string `json:"required_scope,omitempty"`
	RequiredPermission string `json:"required_permission,omitempty"`
	Next               string `json:"next,omitempty"`
}

// Error implements the error interface, returning a human-readable string
// that includes the code and message.
func (e *Error) Error() string {
	if e.Message == "" {
		return e.Code
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// New builds an Error with the given code and message.
func New(code, message string) *Error {
	return &Error{Code: code, Message: message}
}

// Unauthorized builds a 401 error: a bad, expired, or mismatched token.
func Unauthorized(message string) *Error {
	return &Error{Code: CodeUnauthorized, Message: message, Status: 401}
}

// Forbidden builds a 403 error: authenticated but missing a permission,
// scope, or license.
func Forbidden(message string) *Error {
	return &Error{Code: CodeForbidden, Message: message, Status: 403}
}

// NotFoundOrNotVisible builds a 404 error. It deliberately preserves the
// ambiguity between a resource that is absent and one that exists but is
// hidden from the authenticated account.
func NotFoundOrNotVisible(message string) *Error {
	return &Error{Code: CodeNotFoundOrNotVisible, Message: message, Status: 404}
}

// RateLimited builds a 429 error.
func RateLimited(message string) *Error {
	return &Error{Code: CodeRateLimited, Message: message, Status: 429}
}

// InvalidInput builds an error for malformed or missing command input. It
// carries no HTTP status because it is detected before any request is made.
func InvalidInput(message string) *Error {
	return &Error{Code: CodeInvalidInput, Message: message}
}
