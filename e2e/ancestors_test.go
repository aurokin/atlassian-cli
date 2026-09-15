package e2e

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func TestConfluenceAncestorTraversal(t *testing.T) {
	var paths []string
	var mu sync.Mutex
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		paths = append(paths, r.URL.Path)
		mu.Unlock()
		if r.Method != "GET" || r.URL.Query().Get("limit") != "1" {
			t.Errorf("wrong ancestor request: %s %s", r.Method, r.URL)
		}
		switch {
		case strings.HasSuffix(r.URL.Path, "/pages/30/ancestors"):
			_, _ = w.Write([]byte(`{"results":[{"id":"20","type":"folder","title":"middle"}]}`))
		case strings.HasSuffix(r.URL.Path, "/folders/20/ancestors"):
			_, _ = w.Write([]byte(`{"results":[{"id":"10","type":"page","title":"root"}]}`))
		case strings.HasSuffix(r.URL.Path, "/pages/10/ancestors"):
			_, _ = w.Write([]byte(`{"results":[]}`))
		default:
			t.Errorf("unexpected request %s", r.URL)
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()
	p := isolated(t)
	p.profile(t, "conf", srv.URL)
	r := p.run(t, "conf", "page", "ancestors", "30", "--all", "--limit", "1", "--jq", "[.results[] | .id]")
	success(t, r)
	mu.Lock()
	defer mu.Unlock()
	if strings.TrimSpace(r.stdout) != `["10","20"]` || len(paths) != 3 {
		t.Fatalf("ancestor order/completeness: %s paths=%v", r.stdout, paths)
	}
}
