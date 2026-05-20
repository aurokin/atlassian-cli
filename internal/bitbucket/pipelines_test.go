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
