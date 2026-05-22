package bitbucket

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSearchRepositoriesQuery(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repositories/acme" {
			t.Errorf("path = %q", r.URL.Path)
		}
		if got := r.URL.Query().Get("q"); got != `name ~ "cli"` {
			t.Errorf("q = %q", got)
		}
		if got := r.URL.Query().Get("sort"); got != "-updated_on" {
			t.Errorf("sort = %q", got)
		}
		if got := r.URL.Query().Get("pagelen"); got != "5" {
			t.Errorf("pagelen = %q, want 5", got)
		}
		_, _ = w.Write([]byte(`{"values":[{"full_name":"acme/cli-tools"}]}`))
	}))
	defer srv.Close()

	raw, err := newTestClient(srv).SearchRepositories(context.Background(), "acme", `name ~ "cli"`, "-updated_on", 5)
	if err != nil {
		t.Fatalf("SearchRepositories: %v", err)
	}
	page, err := Decode[RepositoryPage](raw)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if len(page.Values) != 1 || page.Values[0].FullName != "acme/cli-tools" {
		t.Fatalf("values = %+v", page.Values)
	}
}

func TestSearchRepositoriesOmitsEmptySort(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := r.URL.Query()["sort"]; ok {
			t.Errorf("sort should be omitted when empty: %q", r.URL.RawQuery)
		}
		_, _ = w.Write([]byte(`{"values":[]}`))
	}))
	defer srv.Close()

	if _, err := newTestClient(srv).SearchRepositories(context.Background(), "acme", `name ~ "x"`, "", 0); err != nil {
		t.Fatalf("SearchRepositories: %v", err)
	}
}

func TestSearchPullRequestsQuery(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repositories/acme/widgets/pullrequests" {
			t.Errorf("path = %q", r.URL.Path)
		}
		if got := r.URL.Query().Get("q"); got != `title ~ "fix"` {
			t.Errorf("q = %q", got)
		}
		_, _ = w.Write([]byte(`{"values":[{"id":7,"title":"fix it"}]}`))
	}))
	defer srv.Close()

	raw, err := newTestClient(srv).SearchPullRequests(context.Background(), "acme", "widgets", `title ~ "fix"`, "", 0)
	if err != nil {
		t.Fatalf("SearchPullRequests: %v", err)
	}
	page, err := Decode[PullRequestPage](raw)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if len(page.Values) != 1 || page.Values[0].ID != 7 {
		t.Fatalf("values = %+v", page.Values)
	}
}

func TestSearchIssuesQuery(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repositories/acme/widgets/issues" {
			t.Errorf("path = %q", r.URL.Path)
		}
		if got := r.URL.Query().Get("q"); got != `state = "open"` {
			t.Errorf("q = %q", got)
		}
		_, _ = w.Write([]byte(`{"values":[{"id":3,"title":"bug"}]}`))
	}))
	defer srv.Close()

	raw, err := newTestClient(srv).SearchIssues(context.Background(), "acme", "widgets", `state = "open"`, "", 0)
	if err != nil {
		t.Fatalf("SearchIssues: %v", err)
	}
	page, err := Decode[IssuePage](raw)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if len(page.Values) != 1 || page.Values[0].ID != 3 {
		t.Fatalf("values = %+v", page.Values)
	}
}

func TestSearchRepositoriesAllFollowsNext(t *testing.T) {
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Query().Get("page") {
		case "", "1":
			_, _ = w.Write([]byte(`{"values":[{"full_name":"acme/a"}],"next":"` +
				srv.URL + `/repositories/acme?page=2"}`))
		case "2":
			_, _ = w.Write([]byte(`{"values":[{"full_name":"acme/b"}]}`))
		default:
			t.Errorf("unexpected page %q", r.URL.Query().Get("page"))
		}
	}))
	defer srv.Close()

	raw, err := newTestClient(srv).SearchRepositoriesAll(context.Background(), "acme", `name ~ "x"`, "", 0)
	if err != nil {
		t.Fatalf("SearchRepositoriesAll: %v", err)
	}
	page, err := Decode[RepositoryPage](raw)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if len(page.Values) != 2 {
		t.Fatalf("values = %+v", page.Values)
	}
}
