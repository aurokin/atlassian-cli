package jiracmd

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aurokin/atlassian-cli/internal/apperr"
)

func TestStatusHumanOutput(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/myself" {
			t.Errorf("path = %q, want /myself", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"accountId":"5b10a2","displayName":"Ada Lovelace","emailAddress":"ada@example.com"}`))
	}))
	defer srv.Close()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginJiraSite(t, srv.URL)

	out, err := execJira(t, "status", "--site", "work")
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	for _, want := range []string{"authenticated", "work", "Ada Lovelace", "5b10a2", "ada@example.com"} {
		if !strings.Contains(out, want) {
			t.Errorf("status output missing %q:\n%s", want, out)
		}
	}
}

func TestStatusJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"accountId":"5b10a2","displayName":"Ada Lovelace"}`))
	}))
	defer srv.Close()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginJiraSite(t, srv.URL)

	out, err := execJira(t, "status", "--site", "work", "--json")
	if err != nil {
		t.Fatalf("status --json: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("status --json output is not valid JSON: %v\n%s", err, out)
	}
	if got["accountId"] != "5b10a2" {
		t.Fatalf("unexpected status JSON: %v", got)
	}
}

func TestStatusMapsAuthFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"message":"Client must be authenticated to access this resource."}`))
	}))
	defer srv.Close()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginJiraSite(t, srv.URL)

	_, err := execJira(t, "status", "--site", "work")
	if err == nil {
		t.Fatal("status against a 401 endpoint returned no error")
	}
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Code != apperr.CodeUnauthorized {
		t.Fatalf("error = %v, want an unauthorized *apperr.Error", err)
	}
}

func TestStatusRequiresSite(t *testing.T) {
	_, err := execJira(t, "status")
	if err == nil {
		t.Fatal("status without --site returned no error")
	}
	var ae *apperr.Error
	if !errors.As(err, &ae) {
		t.Fatalf("error type = %T, want *apperr.Error", err)
	}
}
