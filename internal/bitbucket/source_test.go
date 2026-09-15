package bitbucket

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestListSourcePathAndDecode(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = w.Write([]byte(`{"values":[` +
			`{"path":"src/main.go","type":"commit_file","size":42},` +
			`{"path":"src/sub","type":"commit_directory"}]}`))
	}))
	defer srv.Close()

	raw, err := newTestClient(srv).ListSource(context.Background(), "acme", "widgets", "main", "src", 0)
	if err != nil {
		t.Fatalf("ListSource: %v", err)
	}
	if gotPath != "/repositories/acme/widgets/src/main/src" {
		t.Errorf("path = %q", gotPath)
	}
	page, err := Decode[SourcePage](raw)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if len(page.Values) != 2 || page.Values[0].Type != "commit_file" || page.Values[0].Size != 42 {
		t.Fatalf("values = %+v", page.Values)
	}
}

func TestListSourceEmptyRefHitsBareSrc(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = w.Write([]byte(`{"values":[]}`))
	}))
	defer srv.Close()

	if _, err := newTestClient(srv).ListSource(context.Background(), "acme", "widgets", "", "", 0); err != nil {
		t.Fatalf("ListSource: %v", err)
	}
	if gotPath != "/repositories/acme/widgets/src" {
		t.Errorf("path = %q, want bare /src", gotPath)
	}
}

func TestSrcPathEscapesSegmentsKeepingSlashes(t *testing.T) {
	got := srcPath("acme", "widgets", "feature/x", "dir/sub file.go")
	want := "/repositories/acme/widgets/src/feature/x/dir/sub%20file.go"
	if got != want {
		t.Errorf("srcPath = %q, want %q", got, want)
	}
}

func TestGetFileContent(t *testing.T) {
	var gotPath, gotAccept string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotAccept = r.URL.Path, r.Header.Get("Accept")
		_, _ = w.Write([]byte("package main\n"))
	}))
	defer srv.Close()

	data, err := newTestClient(srv).GetFileContent(context.Background(), "acme", "widgets", "main", "main.go")
	if err != nil {
		t.Fatalf("GetFileContent: %v", err)
	}
	if gotPath != "/repositories/acme/widgets/src/main/main.go" {
		t.Errorf("path = %q", gotPath)
	}
	if gotAccept != "*/*" {
		t.Errorf("Accept = %q, want */*", gotAccept)
	}
	if string(data) != "package main\n" {
		t.Errorf("content = %q", data)
	}
}

func TestListSourceRootRetainsTrailingSlash(t *testing.T) {
	for _, all := range []bool{false, true} {
		t.Run(fmt.Sprint("all=", all), func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/repositories/acme/widgets/src/main/" {
					http.NotFound(w, r)
					return
				}
				if r.URL.Query().Get("pagelen") != "1" {
					t.Errorf("pagelen=%q", r.URL.Query().Get("pagelen"))
				}
				_, _ = w.Write([]byte(`{"values":[{"path":"README.md","type":"commit_file"}]}`))
			}))
			defer srv.Close()
			client := newTestClient(srv)
			list := client.ListSource
			if all {
				list = client.ListSourceAll
			}
			raw, err := list(context.Background(), "acme", "widgets", "main", "", 1)
			if err != nil {
				t.Fatal(err)
			}
			page, err := Decode[SourcePage](raw)
			if err != nil || len(page.Values) != 1 || page.Values[0].Path != "README.md" {
				t.Fatalf("root listing: %s err=%v", raw, err)
			}
		})
	}
}
