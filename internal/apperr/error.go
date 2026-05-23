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
	// CodeFeatureDisabled marks a request that targets a product capability
	// that exists but is switched off for the resource (for example a
	// Bitbucket repository whose issue tracker is disabled). It is distinct
	// from not_found_or_not_visible so an agent can tell "turn the feature on"
	// from "the resource is missing or hidden".
	CodeFeatureDisabled = "feature_disabled"
	// CodeGone marks a 410: the endpoint existed but has been removed, usually
	// because the CLI is calling a withdrawn API version.
	CodeGone = "gone"
	// CodeTimeout marks a transport-level deadline: the request did not complete
	// in time (context deadline or client timeout). Distinct from request_failed
	// so a caller can single out the retryable case.
	CodeTimeout = "timeout"
	// CodeRequestFailed marks a transport-level failure with no HTTP response
	// (connection refused, DNS failure, body read error).
	CodeRequestFailed = "request_failed"
	// CodeResponseDecodeFailed marks a response (or aggregated page set) that
	// could not be decoded into the expected model.
	CodeResponseDecodeFailed = "response_decode_failed"
	// CodeUntrustedURL marks an absolute request URL whose origin is neither the
	// configured site nor the Atlassian API gateway for the target.
	CodeUntrustedURL = "untrusted_url"
	// CodeHTTPError marks a non-2xx response that no more specific category
	// claimed (the catch-all for unexpected statuses).
	CodeHTTPError = "http_error"
	// CodeResultTruncated marks an --all request that hit the page-follow cap
	// while the API still had more pages.
	CodeResultTruncated = "result_truncated"
)

// Process exit codes. Distinct codes let scripts and agents branch on the
// failure category without parsing output. Categories without a dedicated code
// fall through to exitGeneric.
const (
	exitGeneric      = 1
	exitUnauthorized = 4
	exitForbidden    = 5
	exitNotFound     = 6
	exitRateLimited  = 7
	exitInvalidInput = 8
	exitTimeout      = 9
)

// ExitCode maps the error's category to a stable process exit code. Categories
// without a dedicated code return exitGeneric (1), matching the historical
// "any error exits 1" behavior for everything not explicitly classified.
func (e *Error) ExitCode() int {
	switch e.Code {
	case CodeUnauthorized:
		return exitUnauthorized
	case CodeForbidden:
		return exitForbidden
	case CodeNotFoundOrNotVisible:
		return exitNotFound
	case CodeRateLimited:
		return exitRateLimited
	case CodeInvalidInput:
		return exitInvalidInput
	case CodeTimeout:
		return exitTimeout
	default:
		return exitGeneric
	}
}

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

// FeatureDisabled builds an error for a request that targets a product
// capability that is switched off for the resource. The caller sets Status
// from the originating HTTP response, since the underlying status varies by
// product (Bitbucket reports a disabled issue tracker as 404).
func FeatureDisabled(message string) *Error {
	return &Error{Code: CodeFeatureDisabled, Message: message}
}
