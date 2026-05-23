package jiracmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
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
		if r.URL.Path != "/issue/PROJ-1" {
			t.Errorf("path = %q, want /issue/PROJ-1", r.URL.Path)
		}
		if got := r.URL.Query().Get("fields"); got != "attachment" {
			t.Errorf("fields = %q, want attachment", got)
		}
		_, _ = w.Write([]byte(`{"key":"PROJ-1","fields":{"attachment":[` +
			`{"id":"10","filename":"diagram.png","mimeType":"image/png","size":2048},` +
			`{"id":"11","filename":"notes.txt","mimeType":"text/plain","size":12}]}}`))
	}))
	defer srv.Close()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginJiraSite(t, srv.URL)

	out, err := execJira(t, "issue", "attachment", "list", "PROJ-1", "--site", "work")
	if err != nil {
		t.Fatalf("attachment list: %v\n%s", err, out)
	}
	for _, want := range []string{"10", "diagram.png", "image/png", "2048", "11", "notes.txt"} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q:\n%s", want, out)
		}
	}
}

func TestAttachmentListEmpty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"key":"PROJ-1","fields":{}}`))
	}))
	defer srv.Close()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginJiraSite(t, srv.URL)

	out, err := execJira(t, "issue", "attachment", "list", "PROJ-1", "--site", "work")
	if err != nil {
		t.Fatalf("attachment list: %v\n%s", err, out)
	}
	if !strings.Contains(out, "No attachments found.") {
		t.Errorf("output missing empty notice:\n%s", out)
	}
}

func TestAttachmentListJSONIsAttachmentArray(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"key":"PROJ-1","fields":{"attachment":[` +
			`{"id":"10","filename":"a.txt","mimeType":"text/plain","size":3,"extraField":"kept"}]}}`))
	}))
	defer srv.Close()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginJiraSite(t, srv.URL)

	out, err := execJira(t, "issue", "attachment", "list", "PROJ-1", "--site", "work", "--json")
	if err != nil {
		t.Fatalf("attachment list --json: %v\n%s", err, out)
	}
	// --json emits the attachment array (not the enclosing issue), preserving
	// upstream fields the model omits.
	var got []map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("output is not a JSON array: %v\n%s", err, out)
	}
	if len(got) != 1 || got[0]["id"] != "10" || got[0]["extraField"] != "kept" {
		t.Fatalf("unexpected json: %s", out)
	}
}

func TestAttachmentDownloadToFile(t *testing.T) {
	wantData := []byte{0x89, 0x50, 0x4e, 0x47, 0x00, 0x01, 0xff}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/attachment/10":
			_, _ = w.Write([]byte(`{"id":"10","filename":"diagram.png","size":7,` +
				`"content":"` + serverURL(r) + `/content/10"}`))
		case "/content/10":
			_, _ = w.Write(wantData)
		default:
			t.Errorf("unexpected path %q", r.URL.Path)
		}
	}))
	defer srv.Close()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginJiraSite(t, srv.URL)

	dest := filepath.Join(t.TempDir(), "diagram.png")
	out, err := execJira(t, "issue", "attachment", "download", "10", "--out", dest, "--site", "work")
	if err != nil {
		t.Fatalf("attachment download: %v\n%s", err, out)
	}
	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("read downloaded file: %v", err)
	}
	if !bytes.Equal(got, wantData) {
		t.Errorf("downloaded bytes = %v, want %v", got, wantData)
	}
	if !strings.Contains(out, "downloaded attachment 10") {
		t.Errorf("output missing confirmation:\n%s", out)
	}
}

func TestAttachmentDownloadToStdout(t *testing.T) {
	wantData := []byte("plain-text\x00binary")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/attachment/10":
			_, _ = w.Write([]byte(`{"id":"10","content":"` + serverURL(r) + `/content/10"}`))
		case "/content/10":
			_, _ = w.Write(wantData)
		default:
			t.Errorf("unexpected path %q", r.URL.Path)
		}
	}))
	defer srv.Close()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginJiraSite(t, srv.URL)

	out, err := execJira(t, "issue", "attachment", "download", "10", "--out", "-", "--site", "work")
	if err != nil {
		t.Fatalf("attachment download --out -: %v\n%s", err, out)
	}
	if out != string(wantData) {
		t.Errorf("stdout = %q, want %q", out, string(wantData))
	}
}

func TestAttachmentDownloadRequiresOut(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	_, err := execJira(t, "issue", "attachment", "download", "10", "--site", "work")
	if err == nil {
		t.Fatal("attachment download without --out returned no error")
	}
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Code != apperr.CodeInvalidInput {
		t.Fatalf("error = %v, want an invalid_input *apperr.Error", err)
	}
}

func TestAttachmentDownloadJSONPrintsMetadata(t *testing.T) {
	downloadHit := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/attachment/10":
			_, _ = w.Write([]byte(`{"id":"10","content":"` + serverURL(r) + `/content/10"}`))
		case "/content/10":
			downloadHit = true
			_, _ = w.Write([]byte("binary"))
		default:
			t.Errorf("unexpected path %q", r.URL.Path)
		}
	}))
	defer srv.Close()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginJiraSite(t, srv.URL)

	out, err := execJira(t, "issue", "attachment", "download", "10", "--site", "work", "--json")
	if err != nil {
		t.Fatalf("attachment download --json: %v\n%s", err, out)
	}
	if downloadHit {
		t.Error("attachment download --json fetched the binary; want metadata only")
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, out)
	}
	if got["id"] != "10" {
		t.Fatalf("unexpected metadata json: %s", out)
	}
}

func TestAttachmentAddUploadsFile(t *testing.T) {
	var gotFileName, gotBody, gotToken string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/issue/PROJ-1/attachments" {
			t.Errorf("path = %q", r.URL.Path)
		}
		gotToken = r.Header.Get("X-Atlassian-Token")
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Fatalf("ParseMultipartForm: %v", err)
		}
		f, hdr, err := r.FormFile("file")
		if err != nil {
			t.Fatalf("FormFile: %v", err)
		}
		defer f.Close()
		gotFileName = hdr.Filename
		b, _ := io.ReadAll(f)
		gotBody = string(b)
		_, _ = w.Write([]byte(`[{"id":"99","filename":"upload.txt"}]`))
	}))
	defer srv.Close()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	loginJiraSite(t, srv.URL)

	dir := t.TempDir()
	path := filepath.Join(dir, "upload.txt")
	if err := os.WriteFile(path, []byte("file-content"), 0o644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	out, err := execJira(t, "issue", "attachment", "add", "PROJ-1", "--file", path, "--site", "work")
	if err != nil {
		t.Fatalf("attachment add: %v\n%s", err, out)
	}
	if gotToken != "no-check" {
		t.Errorf("X-Atlassian-Token = %q, want no-check", gotToken)
	}
	if gotFileName != "upload.txt" || gotBody != "file-content" {
		t.Errorf("uploaded %q / %q", gotFileName, gotBody)
	}
	if !strings.Contains(out, "uploaded upload.txt (id 99)") {
		t.Errorf("output missing confirmation:\n%s", out)
	}
}

func TestAttachmentAddRequiresFile(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	_, err := execJira(t, "issue", "attachment", "add", "PROJ-1", "--site", "work")
	if err == nil {
		t.Fatal("attachment add without --file returned no error")
	}
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Code != apperr.CodeInvalidInput {
		t.Fatalf("error = %v, want an invalid_input *apperr.Error", err)
	}
}

// serverURL reconstructs the test server's base URL from a request, so a
// handler can embed an absolute content URL pointing back at itself.
func serverURL(r *http.Request) string {
	return "http://" + r.Host
}
