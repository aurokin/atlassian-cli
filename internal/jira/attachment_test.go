package jira

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestClientListIssueAttachments(t *testing.T) {
	var gotFields string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/issue/PROJ-1" {
			t.Errorf("path = %q, want /issue/PROJ-1", r.URL.Path)
		}
		gotFields = r.URL.Query().Get("fields")
		_, _ = w.Write([]byte(`{"key":"PROJ-1","fields":{"attachment":[{"id":"10","filename":"a.txt"}]}}`))
	}))
	defer srv.Close()

	raw, err := newTestClient(srv).ListIssueAttachments(context.Background(), "PROJ-1")
	if err != nil {
		t.Fatalf("ListIssueAttachments: %v", err)
	}
	if gotFields != "attachment" {
		t.Errorf("fields = %q, want attachment", gotFields)
	}
	iss, err := Decode[Issue](raw)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if len(iss.Fields.Attachment) != 1 || iss.Fields.Attachment[0].ID != "10" {
		t.Fatalf("attachments = %+v", iss.Fields.Attachment)
	}
}

func TestClientGetAttachment(t *testing.T) {
	srv := serveJSON(t, "/attachment/10",
		`{"id":"10","filename":"a.txt","mimeType":"text/plain","size":3,"content":"https://x/c/10"}`)
	defer srv.Close()

	raw, err := newTestClient(srv).GetAttachment(context.Background(), "10")
	if err != nil {
		t.Fatalf("GetAttachment: %v", err)
	}
	att, err := Decode[Attachment](raw)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if att.ID != "10" || att.Content != "https://x/c/10" {
		t.Fatalf("attachment = %+v", att)
	}
}

func TestClientFetchAttachmentData(t *testing.T) {
	var gotAccept string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAccept = r.Header.Get("Accept")
		_, _ = w.Write([]byte("binary-bytes"))
	}))
	defer srv.Close()

	data, err := newTestClient(srv).FetchAttachmentData(context.Background(), srv.URL+"/content/10")
	if err != nil {
		t.Fatalf("FetchAttachmentData: %v", err)
	}
	if string(data) != "binary-bytes" {
		t.Errorf("data = %q", data)
	}
	if gotAccept != "*/*" {
		t.Errorf("Accept = %q, want */*", gotAccept)
	}
}

func TestClientFetchAttachmentDataRejectsEmptyURL(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	defer srv.Close()
	if _, err := newTestClient(srv).FetchAttachmentData(context.Background(), ""); err == nil {
		t.Fatal("expected error for empty content URL")
	}
}

func TestClientAddAttachment(t *testing.T) {
	var (
		gotMethod, gotPath, gotToken, gotContentType, gotFileName, gotFileBody string
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath = r.Method, r.URL.Path
		gotToken = r.Header.Get("X-Atlassian-Token")
		gotContentType = r.Header.Get("Content-Type")
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
		gotFileBody = string(b)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[{"id":"99","filename":"up.txt"}]`))
	}))
	defer srv.Close()

	raw, err := newTestClient(srv).AddAttachment(context.Background(), "PROJ-1", "up.txt",
		strings.NewReader("hello"))
	if err != nil {
		t.Fatalf("AddAttachment: %v", err)
	}
	if gotMethod != http.MethodPost || gotPath != "/issue/PROJ-1/attachments" {
		t.Errorf("request = %s %s", gotMethod, gotPath)
	}
	if gotToken != "no-check" {
		t.Errorf("X-Atlassian-Token = %q, want no-check", gotToken)
	}
	if !strings.HasPrefix(gotContentType, "multipart/form-data") {
		t.Errorf("Content-Type = %q, want multipart/form-data", gotContentType)
	}
	if gotFileName != "up.txt" || gotFileBody != "hello" {
		t.Errorf("uploaded file = %q / %q", gotFileName, gotFileBody)
	}
	atts, err := Decode[[]Attachment](raw)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if len(atts) != 1 || atts[0].ID != "99" {
		t.Fatalf("response = %+v", atts)
	}
}
