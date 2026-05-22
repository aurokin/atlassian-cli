package bitbucket

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestListCommitsMainBranch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repositories/acme/widgets/commits" {
			t.Errorf("path = %q", r.URL.Path)
		}
		if got := r.URL.Query().Get("pagelen"); got != "5" {
			t.Errorf("pagelen = %q, want 5", got)
		}
		_, _ = w.Write([]byte(`{"values":[{"hash":"abcdef1234567890","message":"first"}]}`))
	}))
	defer srv.Close()

	raw, err := newTestClient(srv).ListCommits(context.Background(), "acme", "widgets", "", 5)
	if err != nil {
		t.Fatalf("ListCommits: %v", err)
	}
	page, err := Decode[CommitPage](raw)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if len(page.Values) != 1 || page.Values[0].Hash != "abcdef1234567890" {
		t.Fatalf("values = %+v", page.Values)
	}
}

func TestListCommitsRevisionScoped(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repositories/acme/widgets/commits/develop" {
			t.Errorf("path = %q", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"values":[{"hash":"deadbeef"}]}`))
	}))
	defer srv.Close()

	if _, err := newTestClient(srv).ListCommits(context.Background(), "acme", "widgets", "develop", 0); err != nil {
		t.Fatalf("ListCommits: %v", err)
	}
}

func TestGetCommit(t *testing.T) {
	srv := serveJSON(t, "/repositories/acme/widgets/commit/abc123",
		`{"hash":"abc123","message":"fix bug","author":{"raw":"Auro <auro@example.com>"}}`)
	defer srv.Close()

	raw, err := newTestClient(srv).GetCommit(context.Background(), "acme", "widgets", "abc123")
	if err != nil {
		t.Fatalf("GetCommit: %v", err)
	}
	commit, err := Decode[Commit](raw)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if commit.Hash != "abc123" || commit.Author == nil || commit.Author.Raw == "" {
		t.Fatalf("commit = %+v", commit)
	}
}

func TestListCommitsAllFollowsNext(t *testing.T) {
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Query().Get("page") {
		case "", "1":
			_, _ = w.Write([]byte(`{"values":[{"hash":"a"}],"next":"` +
				srv.URL + `/repositories/acme/widgets/commits?page=2"}`))
		case "2":
			_, _ = w.Write([]byte(`{"values":[{"hash":"b"}]}`))
		default:
			t.Errorf("unexpected page %q", r.URL.Query().Get("page"))
		}
	}))
	defer srv.Close()

	raw, err := newTestClient(srv).ListCommitsAll(context.Background(), "acme", "widgets", "", 0)
	if err != nil {
		t.Fatalf("ListCommitsAll: %v", err)
	}
	page, err := Decode[CommitPage](raw)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if len(page.Values) != 2 || page.Values[0].Hash != "a" || page.Values[1].Hash != "b" {
		t.Fatalf("values = %+v", page.Values)
	}
}
