package restutil

import (
	"errors"
	"net/url"
	"strings"
	"testing"

	"github.com/aurokin/atlassian-cli/internal/apperr"
)

func TestWithQueryEmpty(t *testing.T) {
	if got := WithQuery("/spaces", url.Values{}); got != "/spaces" {
		t.Errorf("WithQuery with empty values = %q, want /spaces", got)
	}
}

func TestWithQueryNonEmpty(t *testing.T) {
	q := url.Values{}
	q.Set("limit", "25")
	if got := WithQuery("/spaces", q); got != "/spaces?limit=25" {
		t.Errorf("WithQuery = %q, want /spaces?limit=25", got)
	}
}

func TestDecodeSuccess(t *testing.T) {
	type doc struct {
		Key string `json:"key"`
	}
	got, err := Decode[doc]([]byte(`{"key":"DEV"}`), "Jira")
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if got.Key != "DEV" {
		t.Errorf("decoded = %+v, want key DEV", got)
	}
}

func TestDecodeMalformedNamesProduct(t *testing.T) {
	type doc struct {
		Key string `json:"key"`
	}
	_, err := Decode[doc]([]byte("not json"), "Confluence")
	if err == nil {
		t.Fatal("Decode accepted malformed JSON")
	}
	var ae *apperr.Error
	if !errors.As(err, &ae) {
		t.Fatalf("error type = %T, want *apperr.Error", err)
	}
	if ae.Code != "response_decode_failed" {
		t.Errorf("code = %q, want response_decode_failed", ae.Code)
	}
	if !strings.Contains(ae.Message, "Confluence") {
		t.Errorf("message %q does not name the product", ae.Message)
	}
}

func TestDecodeError(t *testing.T) {
	err := DecodeError("Jira", errors.New("boom"))
	var ae *apperr.Error
	if !errors.As(err, &ae) {
		t.Fatalf("error type = %T, want *apperr.Error", err)
	}
	if ae.Code != "response_decode_failed" {
		t.Errorf("code = %q, want response_decode_failed", ae.Code)
	}
	if !strings.Contains(ae.Message, "Jira") || !strings.Contains(ae.Message, "boom") {
		t.Errorf("message %q missing product or cause", ae.Message)
	}
}
