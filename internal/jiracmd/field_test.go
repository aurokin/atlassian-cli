package jiracmd

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFieldListHuman(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/field" {
			t.Errorf("path = %q, want /field", r.URL.Path)
		}
		_, _ = w.Write([]byte(`[` +
			`{"id":"summary","name":"Summary","custom":false,"schema":{"type":"string"}},` +
			`{"id":"customfield_10010","name":"Sprint","custom":true,"schema":{"type":"array"}}]`))
	}))
	defer srv.Close()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginJiraSite(t, srv.URL)

	out, err := execJira(t, "field", "list", "--site", "work")
	if err != nil {
		t.Fatalf("field list: %v\n%s", err, out)
	}
	for _, want := range []string{"summary", "string", "system", "Summary", "customfield_10010", "array", "custom", "Sprint"} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q:\n%s", want, out)
		}
	}
}

func TestFieldListJSONIsRawArray(t *testing.T) {
	body := `[{"id":"summary","name":"Summary","custom":false,"schema":{"type":"string"}}]`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginJiraSite(t, srv.URL)

	out, err := execJira(t, "field", "list", "--site", "work", "--json")
	if err != nil {
		t.Fatalf("field list --json: %v\n%s", err, out)
	}
	// --json emits the upstream array verbatim, including the schema object the
	// human renderer flattens away.
	for _, want := range []string{`"id": "summary"`, `"schema"`, `"type": "string"`} {
		if !strings.Contains(out, want) {
			t.Fatalf("json output missing %q:\n%s", want, out)
		}
	}
}
