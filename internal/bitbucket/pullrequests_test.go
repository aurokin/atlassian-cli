package bitbucket

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestListPullRequestsQuery(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repositories/acme/widgets/pullrequests" {
			t.Errorf("path = %q", r.URL.Path)
		}
		if got := r.URL.Query().Get("state"); got != "MERGED" {
			t.Errorf("state = %q, want MERGED", got)
		}
		if got := r.URL.Query().Get("pagelen"); got != "10" {
			t.Errorf("pagelen = %q, want 10", got)
		}
		_, _ = w.Write([]byte(`{"values":[{"id":1,"title":"t","state":"MERGED"}]}`))
	}))
	defer srv.Close()

	raw, err := newTestClient(srv).ListPullRequests(context.Background(), "acme", "widgets", "MERGED", 10)
	if err != nil {
		t.Fatalf("ListPullRequests: %v", err)
	}
	page, err := Decode[PullRequestPage](raw)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if len(page.Values) != 1 || page.Values[0].ID != 1 {
		t.Fatalf("values = %+v", page.Values)
	}
}

func TestListPullRequestsAllOmitsEmptyState(t *testing.T) {
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := r.URL.Query()["state"]; ok {
			t.Errorf("state query should be absent, got %q", r.URL.Query().Get("state"))
		}
		switch r.URL.Query().Get("page") {
		case "", "1":
			_, _ = w.Write([]byte(`{"values":[{"id":1}],"next":"` + srv.URL +
				`/repositories/acme/widgets/pullrequests?page=2"}`))
		case "2":
			_, _ = w.Write([]byte(`{"values":[{"id":2}]}`))
		}
	}))
	defer srv.Close()

	raw, err := newTestClient(srv).ListPullRequestsAll(context.Background(), "acme", "widgets", "", 0)
	if err != nil {
		t.Fatalf("ListPullRequestsAll: %v", err)
	}
	page, err := Decode[PullRequestPage](raw)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if len(page.Values) != 2 {
		t.Fatalf("values = %+v", page.Values)
	}
}

func TestGetPullRequest(t *testing.T) {
	srv := serveJSON(t, "/repositories/acme/widgets/pullrequests/7",
		`{"id":7,"title":"Add widget","state":"OPEN"}`)
	defer srv.Close()

	raw, err := newTestClient(srv).GetPullRequest(context.Background(), "acme", "widgets", 7)
	if err != nil {
		t.Fatalf("GetPullRequest: %v", err)
	}
	pr, err := Decode[PullRequest](raw)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if pr.ID != 7 || pr.Title != "Add widget" {
		t.Fatalf("pr = %+v", pr)
	}
}

func TestCreatePullRequestBody(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want POST", r.Method)
		}
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &gotBody)
		_, _ = w.Write([]byte(`{"id":3,"title":"t","state":"OPEN"}`))
	}))
	defer srv.Close()

	_, err := newTestClient(srv).CreatePullRequest(context.Background(), "acme", "widgets",
		CreatePullRequestOptions{
			Title:             "t",
			SourceBranch:      "feature",
			DestinationBranch: "main",
			CloseSourceBranch: true,
		})
	if err != nil {
		t.Fatalf("CreatePullRequest: %v", err)
	}
	if gotBody["title"] != "t" || gotBody["close_source_branch"] != true {
		t.Fatalf("body = %+v", gotBody)
	}
	if _, ok := gotBody["draft"]; ok {
		t.Fatalf("draft should be omitted when false: %+v", gotBody)
	}
	dst, _ := gotBody["destination"].(map[string]any)
	branch, _ := dst["branch"].(map[string]any)
	if branch["name"] != "main" {
		t.Fatalf("destination = %+v", gotBody["destination"])
	}
}
