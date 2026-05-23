package bitbucket

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aurokin/atlassian-cli/internal/apperr"
)

func TestNormalizePipelineUUID(t *testing.T) {
	cases := map[string]string{
		"abc":     "{abc}",
		"{abc}":   "{abc}",
		"  abc  ": "{abc}",
		"":        "",
	}
	for in, want := range cases {
		if got := NormalizePipelineUUID(in); got != want {
			t.Errorf("NormalizePipelineUUID(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestPipelineRefDetection(t *testing.T) {
	if n, ok := PipelineRef("42"); !ok || n != 42 {
		t.Errorf("PipelineRef(42) = %d,%v", n, ok)
	}
	if _, ok := PipelineRef("{uuid}"); ok {
		t.Errorf("PipelineRef(uuid) should not be a build number")
	}
	if _, ok := PipelineRef("0"); ok {
		t.Errorf("PipelineRef(0) should not be a build number")
	}
}

func TestListPipelinesQuery(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repositories/acme/widgets/pipelines/" {
			t.Errorf("path = %q", r.URL.Path)
		}
		if got := r.URL.Query().Get("sort"); got != "-created_on" {
			t.Errorf("sort = %q", got)
		}
		if got := r.URL.Query().Get("status"); got != "COMPLETED" {
			t.Errorf("status = %q", got)
		}
		_, _ = w.Write([]byte(`{"values":[{"build_number":1}]}`))
	}))
	defer srv.Close()

	raw, err := newTestClient(srv).ListPipelines(context.Background(), "acme", "widgets", "COMPLETED", 0)
	if err != nil {
		t.Fatalf("ListPipelines: %v", err)
	}
	if _, err := Decode[PipelinePage](raw); err != nil {
		t.Fatalf("Decode: %v", err)
	}
}

func TestGetPipelineByBuildNumberNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"values":[{"build_number":1}]}`))
	}))
	defer srv.Close()

	_, err := newTestClient(srv).GetPipelineByBuildNumber(context.Background(), "acme", "widgets", 7)
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Code != apperr.CodeNotFoundOrNotVisible {
		t.Fatalf("error = %v, want not_found_or_not_visible", err)
	}
}

func TestTriggerPipelineDefaultsRefType(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repositories/acme/widgets/pipelines" {
			t.Errorf("path = %q", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"build_number":1}`))
	}))
	defer srv.Close()

	if _, err := newTestClient(srv).TriggerPipeline(context.Background(), "acme", "widgets", "", "main"); err != nil {
		t.Fatalf("TriggerPipeline: %v", err)
	}
	if _, err := newTestClient(srv).TriggerPipeline(context.Background(), "acme", "widgets", "branch", ""); err == nil {
		t.Fatalf("expected error for empty ref name")
	}
}

func TestStopPipeline(t *testing.T) {
	var gotMethod, gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath = r.Method, r.URL.Path
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	if err := newTestClient(srv).StopPipeline(context.Background(), "acme", "widgets", "abc-123"); err != nil {
		t.Fatalf("StopPipeline: %v", err)
	}
	if gotMethod != http.MethodPost || gotPath != "/repositories/acme/widgets/pipelines/{abc-123}/stopPipeline" {
		t.Errorf("request = %s %s", gotMethod, gotPath)
	}
}

func TestListPipelineSteps(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = w.Write([]byte(`{"values":[{"uuid":"{s1}","name":"Build","state":{"name":"COMPLETED","result":{"name":"SUCCESSFUL"}}}]}`))
	}))
	defer srv.Close()

	raw, err := newTestClient(srv).ListPipelineSteps(context.Background(), "acme", "widgets", "{p1}", 0)
	if err != nil {
		t.Fatalf("ListPipelineSteps: %v", err)
	}
	if gotPath != "/repositories/acme/widgets/pipelines/{p1}/steps/" {
		t.Errorf("path = %q", gotPath)
	}
	page, err := Decode[PipelineStepPage](raw)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if len(page.Values) != 1 || page.Values[0].UUID != "{s1}" || page.Values[0].Name != "Build" {
		t.Fatalf("values = %+v", page.Values)
	}
}

func TestGetPipelineStepLog(t *testing.T) {
	var gotPath, gotAccept string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotAccept = r.URL.Path, r.Header.Get("Accept")
		_, _ = w.Write([]byte("+ go test ./...\nok\n"))
	}))
	defer srv.Close()

	data, err := newTestClient(srv).GetPipelineStepLog(context.Background(), "acme", "widgets", "p1", "s1")
	if err != nil {
		t.Fatalf("GetPipelineStepLog: %v", err)
	}
	if gotPath != "/repositories/acme/widgets/pipelines/{p1}/steps/{s1}/log" {
		t.Errorf("path = %q", gotPath)
	}
	if gotAccept != "*/*" {
		t.Errorf("Accept = %q, want */*", gotAccept)
	}
	if string(data) != "+ go test ./...\nok\n" {
		t.Errorf("log = %q", data)
	}
}
