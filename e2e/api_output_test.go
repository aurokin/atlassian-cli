package e2e

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestRawAPIBodyMethods(t *testing.T) {
	for _, product := range []string{"jira", "conf", "bb"} {
		t.Run(product, func(t *testing.T) {
			for _, method := range []string{"post", "put", "patch", "delete"} {
				t.Run(method, func(t *testing.T) {
					body := `{"title":"literal body with spaces","enabled":false}`
					var requests atomic.Int32
					srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						requests.Add(1)
						data, err := io.ReadAll(r.Body)
						if err != nil || string(data) != body {
							t.Errorf("request body changed: %q (%v)", data, err)
						}
						if r.Method != strings.ToUpper(method) || r.URL.Path != "/fixture" || r.URL.Query().Get("version") != "3" {
							t.Errorf("wrong request %s %s", r.Method, r.URL)
						}
						if r.Header.Get("Authorization") != "Bearer "+dummyToken || r.Header.Get("Content-Type") != "application/json" || r.Header.Get("Accept") != "application/json" {
							t.Error("request lost authentication/JSON headers")
						}
						w.WriteHeader(http.StatusCreated)
						_, _ = w.Write([]byte(`{"id":"created","future_field":{"kept":true}}`))
					}))
					defer srv.Close()
					p := isolated(t)
					p.profile(t, product, srv.URL)
					r := p.run(t, product, "api", "/fixture?version=3", "-X", method, "--data", body, "--json")
					success(t, r)
					var response struct {
						ID     string `json:"id"`
						Future struct {
							Kept bool `json:"kept"`
						} `json:"future_field"`
					}
					if err := json.Unmarshal([]byte(r.stdout), &response); err != nil || response.ID != "created" || !response.Future.Kept || requests.Load() != 1 {
						t.Fatalf("raw API result lost upstream fields: %s", r.stdout)
					}
				})
			}
		})
	}
}

func TestAPIObjectArrayAndEmptyOutput(t *testing.T) {
	cases := []struct {
		name, body, want string
		flags            []string
	}{
		{"object-fields", `{"id":1,"name":"one","omitted":true}`, `{"name":"one","id":1}`, []string{"--json=name,id,unknown"}},
		{"array-fields", `[{"id":1,"extra":true},{"id":2,"extra":false}]`, `[{"id":1},{"id":2}]`, []string{"--json=id"}},
		{"array-jq", `[{"id":1},{"id":2}]`, "1\n2\n", []string{"--jq", ".[].id"}},
		{"object-jq", `{"values":[{"id":1},{"id":2}]}`, `[1,2]`, []string{"--jq", ".values | map(.id)"}},
		{"empty-array", `[]`, `[]`, []string{"--json=id"}},
		{"empty-array-jq", `[]`, `0`, []string{"--jq", "length"}},
		{"empty-object", `{}`, `{}`, []string{"--json=id"}},
		{"bodyless", "", "", []string{"--json"}},
	}
	for _, product := range []string{"jira", "conf", "bb"} {
		t.Run(product, func(t *testing.T) {
			for _, tc := range cases {
				t.Run(tc.name, func(t *testing.T) {
					var requests atomic.Int32
					srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						requests.Add(1)
						if tc.body == "" {
							w.WriteHeader(http.StatusNoContent)
							return
						}
						_, _ = w.Write([]byte(tc.body))
					}))
					defer srv.Close()
					p := isolated(t)
					p.profile(t, product, srv.URL)
					r := p.run(t, product, append([]string{"api", "/fixture"}, tc.flags...)...)
					success(t, r)
					if tc.name == "array-jq" || tc.body == "" {
						if r.stdout != tc.want {
							t.Fatalf("wrong stream/empty output %q", r.stdout)
						}
					} else {
						var compact bytes.Buffer
						if err := json.Compact(&compact, []byte(r.stdout)); err != nil || compact.String() != tc.want {
							t.Fatalf("wrong projection: %s", r.stdout)
						}
					}
					if requests.Load() != 1 {
						t.Fatal("output fixture was not requested exactly once")
					}
				})
			}
		})
	}
}
