package e2e

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

func downloadArgs(product string) []string {
	if product == "jira" {
		return []string{"issue", "attachment", "download", "10"}
	}
	return []string{"attachment", "download", "10"}
}

func downloadFixture(t *testing.T, product string, binary http.HandlerFunc) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	metadataRequests := &atomic.Int32{}
	metadataPath := "/attachment/10"
	if product == "conf" {
		metadataPath = "/attachments/10"
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+dummyToken {
			t.Error("download fixture request was not authenticated")
		}
		switch r.URL.Path {
		case metadataPath:
			metadataRequests.Add(1)
			if product == "jira" {
				_, _ = fmt.Fprintf(w, `{"id":"10","filename":"fixture.bin","content":"http://%s/binary/10"}`, r.Host)
			} else {
				_, _ = w.Write([]byte(`{"id":"10","title":"fixture.bin","downloadLink":"/binary/10"}`))
			}
		case "/binary/10":
			if r.Header.Get("Accept") != "*/*" {
				t.Errorf("binary Accept=%q, want */*", r.Header.Get("Accept"))
			}
			binary(w, r)
		default:
			t.Errorf("unexpected download path %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	return srv, metadataRequests
}

func TestDownloadFailuresPreserveDestination(t *testing.T) {
	for _, product := range []string{"jira", "conf"} {
		t.Run(product, func(t *testing.T) {
			for _, fault := range []struct {
				name string
				exit int
				code string
			}{
				{"forbidden", 5, "forbidden"}, {"short-body", 1, "request_failed"},
			} {
				t.Run(fault.name, func(t *testing.T) {
					var downloads atomic.Int32
					srv, metadata := downloadFixture(t, product, func(w http.ResponseWriter, _ *http.Request) {
						downloads.Add(1)
						if fault.name == "forbidden" {
							w.WriteHeader(http.StatusForbidden)
							_, _ = w.Write([]byte(`{"message":"binary not permitted"}`))
							return
						}
						w.Header().Set("Content-Length", "100")
						_, _ = w.Write([]byte{0, 255, 1})
					})
					defer srv.Close()
					p := isolated(t)
					p.profile(t, product, srv.URL)
					original := []byte("existing destination\x00must survive\xff")
					existing := filepath.Join(t.TempDir(), "existing.bin")
					if err := os.WriteFile(existing, original, 0600); err != nil {
						t.Fatal(err)
					}
					missing := filepath.Join(t.TempDir(), "not-created.bin")
					for _, destination := range []string{existing, missing, "-"} {
						r := p.run(t, product, append(downloadArgs(product), "--out", destination)...)
						if r.exit != fault.exit || r.stdout != "" || !strings.Contains(r.stderr, fault.code) {
							t.Fatalf("failed download contract: %+v", r)
						}
						actual, err := os.ReadFile(existing)
						if err != nil || !bytes.Equal(actual, original) {
							t.Fatalf("failed download corrupted existing file: %v", err)
						}
						if _, err := os.Stat(missing); !os.IsNotExist(err) {
							t.Fatalf("failed download created destination: %v", err)
						}
					}
					if metadata.Load() != 3 || downloads.Load() != 3 {
						t.Fatalf("failure not exercised: metadata=%d downloads=%d", metadata.Load(), downloads.Load())
					}
				})
			}
		})
	}
}

func TestDownloadBinaryStdoutAndMetadataOnly(t *testing.T) {
	for _, product := range []string{"jira", "conf"} {
		t.Run(product, func(t *testing.T) {
			data := []byte{0, 255, 'b', 'i', 'n', '\n', 1}
			var downloads atomic.Int32
			srv, metadata := downloadFixture(t, product, func(w http.ResponseWriter, _ *http.Request) { downloads.Add(1); _, _ = w.Write(data) })
			defer srv.Close()
			p := isolated(t)
			p.profile(t, product, srv.URL)
			r := p.run(t, product, append(downloadArgs(product), "--out", "-")...)
			success(t, r)
			if !bytes.Equal([]byte(r.stdout), data) {
				t.Fatalf("binary stdout was altered: %q", r.stdout)
			}
			if downloads.Load() != 1 {
				t.Fatal("binary download not exercised")
			}
			r = p.run(t, product, append(downloadArgs(product), "--jq", ".id")...)
			success(t, r)
			if strings.TrimSpace(r.stdout) != `"10"` || downloads.Load() != 1 || metadata.Load() != 2 {
				t.Fatalf("metadata mode fetched binary or lost ID: %+v", r)
			}
		})
	}
}
