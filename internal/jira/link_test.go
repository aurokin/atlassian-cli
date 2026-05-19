package jira

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClientCreateIssueLink(t *testing.T) {
	var (
		gotMethod string
		gotBody   map[string]any
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		if r.URL.Path != "/issueLink" {
			t.Errorf("path = %q, want /issueLink", r.URL.Path)
		}
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	if err := newTestClient(srv).CreateIssueLink(context.Background(), "PROJ-1", "PROJ-2", "Blocks"); err != nil {
		t.Fatalf("CreateIssueLink: %v", err)
	}
	if gotMethod != http.MethodPost {
		t.Errorf("method = %q, want POST", gotMethod)
	}
	typ, _ := gotBody["type"].(map[string]any)
	if typ["name"] != "Blocks" {
		t.Errorf("CreateIssueLink type = %v, want Blocks", gotBody["type"])
	}
	inward, _ := gotBody["inwardIssue"].(map[string]any)
	outward, _ := gotBody["outwardIssue"].(map[string]any)
	if inward["key"] != "PROJ-1" || outward["key"] != "PROJ-2" {
		t.Errorf("CreateIssueLink inward/outward = %v/%v, want PROJ-1/PROJ-2",
			gotBody["inwardIssue"], gotBody["outwardIssue"])
	}
}

func TestClientListIssueLinkTypes(t *testing.T) {
	srv := serveJSON(t, "/issueLinkType",
		`{"issueLinkTypes":[{"id":"1000","name":"Blocks","inward":"is blocked by","outward":"blocks"}]}`)
	defer srv.Close()

	raw, err := newTestClient(srv).ListIssueLinkTypes(context.Background())
	if err != nil {
		t.Fatalf("ListIssueLinkTypes: %v", err)
	}
	lt, err := Decode[LinkTypeList](raw)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if len(lt.Types) != 1 || lt.Types[0].Name != "Blocks" || lt.Types[0].Inward != "is blocked by" {
		t.Fatalf("link types = %+v", lt.Types)
	}
}
