package conf

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aurokin/atlassian-cli/internal/auth"
	"github.com/aurokin/atlassian-cli/internal/httpclient"
)

// Phase 7 client methods: footer comments, labels, and attachments. These
// exercise the conf client directly, parallel to the per-method tests in
// client_test.go.

func TestClientListFooterComments(t *testing.T) {
	srv, query := serveJSON(t, "/pages/10/footer-comments",
		`{"results":[{"id":"c1","status":"current","title":"Re: Home"}]}`)
	defer srv.Close()

	raw, err := newTestClient(srv).ListFooterComments(context.Background(), "10", 25)
	if err != nil {
		t.Fatalf("ListFooterComments: %v", err)
	}
	if got := query.Get("limit"); got != "25" {
		t.Errorf("limit param = %q, want 25", got)
	}
	cl, err := Decode[CommentList](raw)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if len(cl.Results) != 1 || cl.Results[0].ID != "c1" {
		t.Fatalf("comments = %+v", cl.Results)
	}
}

func TestClientGetFooterComment(t *testing.T) {
	srv, query := serveJSON(t, "/footer-comments/c1",
		`{"id":"c1","status":"current","pageId":"10","version":{"number":2},`+
			`"body":{"storage":{"representation":"storage","value":"<p>hi</p>"}}}`)
	defer srv.Close()

	raw, err := newTestClient(srv).GetFooterComment(context.Background(), "c1")
	if err != nil {
		t.Fatalf("GetFooterComment: %v", err)
	}
	if got := query.Get("body-format"); got != "storage" {
		t.Errorf("body-format param = %q, want storage", got)
	}
	c, err := Decode[Comment](raw)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if c.ID != "c1" || c.Body.Storage.Value != "<p>hi</p>" {
		t.Fatalf("comment = %+v", c)
	}
}

func TestClientCreateFooterComment(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want POST", r.Method)
		}
		if r.URL.Path != "/footer-comments" {
			t.Errorf("path = %q, want /footer-comments", r.URL.Path)
		}
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_, _ = w.Write([]byte(`{"id":"c9","status":"current","pageId":"10"}`))
	}))
	defer srv.Close()

	raw, err := newTestClient(srv).CreateFooterComment(context.Background(), "10", "storage", "<p>nice</p>")
	if err != nil {
		t.Fatalf("CreateFooterComment: %v", err)
	}
	if gotBody["pageId"] != "10" {
		t.Errorf("CreateFooterComment sent pageId %v, want 10", gotBody["pageId"])
	}
	body, _ := gotBody["body"].(map[string]any)
	if body["representation"] != "storage" || body["value"] != "<p>nice</p>" {
		t.Errorf("CreateFooterComment body = %v, want storage/<p>nice</p>", gotBody["body"])
	}
	c, err := Decode[Comment](raw)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if c.ID != "c9" {
		t.Fatalf("comment = %+v", c)
	}
}

func TestClientUpdateFooterComment(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("method = %q, want PUT", r.Method)
		}
		if r.URL.Path != "/footer-comments/c1" {
			t.Errorf("path = %q, want /footer-comments/c1", r.URL.Path)
		}
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_, _ = w.Write([]byte(`{"id":"c1","status":"current","version":{"number":4}}`))
	}))
	defer srv.Close()

	raw, err := newTestClient(srv).UpdateFooterComment(context.Background(), "c1", "storage", "<p>v4</p>", 4)
	if err != nil {
		t.Fatalf("UpdateFooterComment: %v", err)
	}
	ver, _ := gotBody["version"].(map[string]any)
	if ver["number"] != float64(4) {
		t.Errorf("UpdateFooterComment version = %v, want number 4", gotBody["version"])
	}
	body, _ := gotBody["body"].(map[string]any)
	if body["value"] != "<p>v4</p>" {
		t.Errorf("UpdateFooterComment body = %v, want value <p>v4</p>", gotBody["body"])
	}
	c, err := Decode[Comment](raw)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if c.Version.Number != 4 {
		t.Fatalf("comment = %+v", c)
	}
}

func TestClientDeleteFooterComment(t *testing.T) {
	var gotMethod string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		if r.URL.Path != "/footer-comments/c1" {
			t.Errorf("path = %q, want /footer-comments/c1", r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	if err := newTestClient(srv).DeleteFooterComment(context.Background(), "c1"); err != nil {
		t.Fatalf("DeleteFooterComment: %v", err)
	}
	if gotMethod != http.MethodDelete {
		t.Errorf("DeleteFooterComment method = %q, want DELETE", gotMethod)
	}
}

func TestClientListLabels(t *testing.T) {
	srv, query := serveJSON(t, "/pages/10/labels",
		`{"results":[{"id":"1","name":"needs-review","prefix":"global"}]}`)
	defer srv.Close()

	raw, err := newTestClient(srv).ListLabels(context.Background(), "10", 5)
	if err != nil {
		t.Fatalf("ListLabels: %v", err)
	}
	if got := query.Get("limit"); got != "5" {
		t.Errorf("limit param = %q, want 5", got)
	}
	ll, err := Decode[LabelList](raw)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if len(ll.Results) != 1 || ll.Results[0].Name != "needs-review" {
		t.Fatalf("labels = %+v", ll.Results)
	}
}

func TestClientAddLabel(t *testing.T) {
	var gotBody []map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want POST", r.Method)
		}
		if r.URL.Path != "/rest/api/content/10/label" {
			t.Errorf("path = %q, want /rest/api/content/10/label", r.URL.Path)
		}
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_, _ = w.Write([]byte(`{"results":[{"id":"1","name":"needs-review","prefix":"global"}]}`))
	}))
	defer srv.Close()

	if _, err := newTestClient(srv).AddLabel(context.Background(), "10", "needs-review"); err != nil {
		t.Fatalf("AddLabel: %v", err)
	}
	if len(gotBody) != 1 || gotBody[0]["name"] != "needs-review" {
		t.Errorf("AddLabel sent body %v, want [{name: needs-review}]", gotBody)
	}
}

func TestClientRemoveLabel(t *testing.T) {
	var gotMethod string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		if r.URL.Path != "/rest/api/content/10/label/needs-review" {
			t.Errorf("path = %q, want /rest/api/content/10/label/needs-review", r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	if err := newTestClient(srv).RemoveLabel(context.Background(), "10", "needs-review"); err != nil {
		t.Fatalf("RemoveLabel: %v", err)
	}
	if gotMethod != http.MethodDelete {
		t.Errorf("RemoveLabel method = %q, want DELETE", gotMethod)
	}
}

func TestClientListAttachments(t *testing.T) {
	srv, query := serveJSON(t, "/pages/10/attachments",
		`{"results":[{"id":"a1","title":"diagram.png","mediaType":"image/png","fileSize":2048}]}`)
	defer srv.Close()

	raw, err := newTestClient(srv).ListAttachments(context.Background(), "10", 3)
	if err != nil {
		t.Fatalf("ListAttachments: %v", err)
	}
	if got := query.Get("limit"); got != "3" {
		t.Errorf("limit param = %q, want 3", got)
	}
	al, err := Decode[AttachmentList](raw)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if len(al.Results) != 1 || al.Results[0].FileSize != 2048 {
		t.Fatalf("attachments = %+v", al.Results)
	}
}

func TestClientGetAttachment(t *testing.T) {
	srv, _ := serveJSON(t, "/attachments/a1",
		`{"id":"a1","title":"diagram.png","downloadLink":"/download/attachments/10/diagram.png"}`)
	defer srv.Close()

	raw, err := newTestClient(srv).GetAttachment(context.Background(), "a1")
	if err != nil {
		t.Fatalf("GetAttachment: %v", err)
	}
	att, err := Decode[Attachment](raw)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if att.ID != "a1" || att.DownloadLink != "/download/attachments/10/diagram.png" {
		t.Fatalf("attachment = %+v", att)
	}
}

func TestClientFetchAttachmentData(t *testing.T) {
	wantData := []byte{0x89, 0x50, 0x4e, 0x47, 0x00, 0xff}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/download/attachments/10/f.bin" {
			t.Errorf("path = %q, want /download/attachments/10/f.bin", r.URL.Path)
		}
		_, _ = w.Write(wantData)
	}))
	defer srv.Close()

	got, err := newTestClient(srv).FetchAttachmentData(context.Background(), "/download/attachments/10/f.bin")
	if err != nil {
		t.Fatalf("FetchAttachmentData: %v", err)
	}
	if !bytes.Equal(got, wantData) {
		t.Errorf("FetchAttachmentData = %v, want %v", got, wantData)
	}
}

// TestDownloadURLResolvesAgainstCloudContextPath confirms downloadURL strips
// the trailing "/api/v2" from a Cloud API base so a context-rooted
// downloadLink lands under ".../wiki", not the API base. The data-center
// helper cannot cover this branch — its API base is the configured URL
// verbatim, with no "/api/v2" suffix to trim.
func TestDownloadURLResolvesAgainstCloudContextPath(t *testing.T) {
	target := httpclient.Target{
		Product:    httpclient.ProductConfluence,
		TokenStyle: auth.StyleCloudClassic,
		SiteName:   "test",
		BaseURL:    "https://example.atlassian.net",
	}
	cred := auth.Credential{
		Style:    auth.StyleCloudClassic,
		Username: "tester@example.com",
		Token:    "test-token",
	}
	cc := New(httpclient.New(target, cred, nil))

	got, err := cc.downloadURL("/download/attachments/10/f.bin")
	if err != nil {
		t.Fatalf("downloadURL: %v", err)
	}
	want := "https://example.atlassian.net/wiki/download/attachments/10/f.bin"
	if got != want {
		t.Errorf("downloadURL = %q, want %q", got, want)
	}
}

func TestDownloadURLKeepsAbsoluteLink(t *testing.T) {
	cc := newTestClient(httptest.NewServer(http.NotFoundHandler()))
	abs := "https://media.example.com/file?token=abc"
	got, err := cc.downloadURL(abs)
	if err != nil {
		t.Fatalf("downloadURL: %v", err)
	}
	if got != abs {
		t.Errorf("downloadURL = %q, want it unchanged %q", got, abs)
	}
}

func TestDownloadURLRejectsEmptyLink(t *testing.T) {
	cc := newTestClient(httptest.NewServer(http.NotFoundHandler()))
	if _, err := cc.downloadURL(""); err == nil {
		t.Fatal("downloadURL accepted an empty link")
	}
}
