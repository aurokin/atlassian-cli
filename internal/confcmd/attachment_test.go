package confcmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aurokin/atlassian-cli/internal/apperr"
)

func TestAttachmentListHumanOutput(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/pages/10/attachments" {
			t.Errorf("path = %q, want /pages/10/attachments", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"results":[` +
			`{"id":"a1","title":"diagram.png","mediaType":"image/png","fileSize":2048},` +
			`{"id":"a2","title":"notes.txt","mediaType":"text/plain","fileSize":12}]}`))
	}))
	defer srv.Close()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginConfSite(t, srv.URL)

	out, err := execConf(t, "attachment", "list", "10", "--site", "work")
	if err != nil {
		t.Fatalf("attachment list: %v", err)
	}
	for _, want := range []string{"a1", "diagram.png", "image/png", "notes.txt"} {
		if !strings.Contains(out, want) {
			t.Errorf("attachment list output missing %q:\n%s", want, out)
		}
	}
}

func TestAttachmentListJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"results":[{"id":"a1"}]}`))
	}))
	defer srv.Close()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginConfSite(t, srv.URL)

	out, err := execConf(t, "attachment", "list", "10", "--site", "work", "--json")
	if err != nil {
		t.Fatalf("attachment list --json: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("attachment list --json output is not valid JSON: %v\n%s", err, out)
	}
	if _, ok := got["results"]; !ok {
		t.Fatalf("unexpected attachment list JSON: %v", got)
	}
}

func TestAttachmentDownloadToFile(t *testing.T) {
	wantData := []byte{0x89, 0x50, 0x4e, 0x47, 0x00, 0x01, 0xff}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/attachments/a1":
			_, _ = w.Write([]byte(`{"id":"a1","title":"diagram.png","fileSize":7,` +
				`"downloadLink":"/download/attachments/10/diagram.png?version=1"}`))
		case "/download/attachments/10/diagram.png":
			if got := r.URL.Query().Get("version"); got != "1" {
				t.Errorf("download version = %q, want 1", got)
			}
			_, _ = w.Write(wantData)
		default:
			t.Errorf("unexpected path %q", r.URL.Path)
		}
	}))
	defer srv.Close()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginConfSite(t, srv.URL)

	dest := filepath.Join(t.TempDir(), "diagram.png")
	out, err := execConf(t, "attachment", "download", "a1", "--out", dest, "--site", "work")
	if err != nil {
		t.Fatalf("attachment download: %v", err)
	}
	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("read downloaded file: %v", err)
	}
	if !bytes.Equal(got, wantData) {
		t.Errorf("downloaded bytes = %v, want %v", got, wantData)
	}
	if !strings.Contains(out, "downloaded attachment a1") {
		t.Errorf("download output missing 'downloaded attachment a1':\n%s", out)
	}
}

func TestAttachmentDownloadToStdout(t *testing.T) {
	wantData := []byte("plain-text-body\x00binary")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/attachments/a1":
			_, _ = w.Write([]byte(`{"id":"a1","downloadLink":"/download/attachments/10/f.bin"}`))
		case "/download/attachments/10/f.bin":
			_, _ = w.Write(wantData)
		default:
			t.Errorf("unexpected path %q", r.URL.Path)
		}
	}))
	defer srv.Close()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginConfSite(t, srv.URL)

	out, err := execConf(t, "attachment", "download", "a1", "--out", "-", "--site", "work")
	if err != nil {
		t.Fatalf("attachment download --out -: %v", err)
	}
	if out != string(wantData) {
		t.Errorf("stdout = %q, want %q", out, string(wantData))
	}
}

func TestAttachmentDownloadRequiresOut(t *testing.T) {
	_, err := execConf(t, "attachment", "download", "a1", "--site", "work")
	if err == nil {
		t.Fatal("attachment download without --out returned no error")
	}
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Code != apperr.CodeInvalidInput {
		t.Fatalf("error = %v, want an invalid_input *apperr.Error", err)
	}
}

func TestAttachmentDownloadJSONPrintsMetadata(t *testing.T) {
	var downloadHit bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/attachments/a1":
			_, _ = w.Write([]byte(`{"id":"a1","downloadLink":"/download/attachments/10/f.bin"}`))
		case "/download/attachments/10/f.bin":
			downloadHit = true
			_, _ = w.Write([]byte("binary"))
		default:
			t.Errorf("unexpected path %q", r.URL.Path)
		}
	}))
	defer srv.Close()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginConfSite(t, srv.URL)

	out, err := execConf(t, "attachment", "download", "a1", "--site", "work", "--json")
	if err != nil {
		t.Fatalf("attachment download --json: %v", err)
	}
	if downloadHit {
		t.Error("attachment download --json fetched the binary; want metadata only")
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("attachment download --json output is not valid JSON: %v\n%s", err, out)
	}
	if got["id"] != "a1" {
		t.Errorf("download --json metadata = %v, want id a1", got)
	}
}
