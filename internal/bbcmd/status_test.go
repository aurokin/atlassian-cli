package bbcmd

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestStatusHuman(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/user" {
			t.Errorf("path = %q", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"account_id":"123","display_name":"Auro","username":"auro"}`))
	}))
	defer srv.Close()

	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginBBSite(t, srv.URL)

	out, err := execBB(t, "status", "--site", "work")
	if err != nil {
		t.Fatalf("status: %v\n%s", err, out)
	}
	for _, want := range []string{"authenticated", "work", "Auro", "123", "auro"} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q:\n%s", want, out)
		}
	}
}

func TestStatusJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"account_id":"123","display_name":"Auro","username":"auro"}`))
	}))
	defer srv.Close()

	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginBBSite(t, srv.URL)

	out, err := execBB(t, "status", "--site", "work", "--jq", ".username")
	if err != nil {
		t.Fatalf("status --jq: %v\n%s", err, out)
	}
	if strings.TrimSpace(out) != `"auro"` {
		t.Fatalf("jq output = %q", out)
	}
}
