package jira

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClientListWorklogs(t *testing.T) {
	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/issue/PROJ-1/worklog" {
			t.Errorf("path = %q, want /issue/PROJ-1/worklog", r.URL.Path)
		}
		gotQuery = r.URL.RawQuery
		_, _ = w.Write([]byte(`{"worklogs":[{"id":"10000","timeSpent":"1h"}]}`))
	}))
	defer srv.Close()

	raw, err := newTestClient(srv).ListWorklogs(context.Background(), "PROJ-1", 0, 5)
	if err != nil {
		t.Fatalf("ListWorklogs: %v", err)
	}
	if gotQuery != "maxResults=5" {
		t.Errorf("query = %q, want maxResults=5", gotQuery)
	}
	wp, err := Decode[WorklogPage](raw)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if len(wp.Worklogs) != 1 || wp.Worklogs[0].TimeSpent != "1h" {
		t.Fatalf("worklogs = %+v", wp.Worklogs)
	}
}

func TestClientListWorklogsPassesStartedAfter(t *testing.T) {
	var gotStartedAfter string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotStartedAfter = r.URL.Query().Get("startedAfter")
		_, _ = w.Write([]byte(`{"worklogs":[]}`))
	}))
	defer srv.Close()
	if _, err := newTestClient(srv).ListWorklogs(context.Background(), "PROJ-1", 1700000000000, 0); err != nil {
		t.Fatalf("ListWorklogs: %v", err)
	}
	if gotStartedAfter != "1700000000000" {
		t.Errorf("startedAfter = %q, want 1700000000000", gotStartedAfter)
	}
}

func TestClientAddWorklog(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/issue/PROJ-1/worklog" {
			t.Errorf("path = %q, want /issue/PROJ-1/worklog", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want POST", r.Method)
		}
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_, _ = w.Write([]byte(`{"id":"10005","timeSpent":"3h 30m"}`))
	}))
	defer srv.Close()

	adf := DocOf("deep work")
	raw, err := newTestClient(srv).AddWorklog(context.Background(), "PROJ-1", "3h 30m", adf)
	if err != nil {
		t.Fatalf("AddWorklog: %v", err)
	}
	if gotBody["timeSpent"] != "3h 30m" {
		t.Errorf("AddWorklog timeSpent = %v, want 3h 30m", gotBody["timeSpent"])
	}
	cm, ok := gotBody["comment"].(map[string]any)
	if !ok {
		t.Fatalf("AddWorklog comment = %v, want ADF object", gotBody["comment"])
	}
	if cm["type"] != "doc" {
		t.Errorf("AddWorklog comment type = %v, want doc", cm["type"])
	}
	wl, err := Decode[Worklog](raw)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if wl.ID != "10005" {
		t.Fatalf("worklog = %+v", wl)
	}
}

func TestClientAddWorklogWithoutComment(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_, _ = w.Write([]byte(`{"id":"10005","timeSpent":"15m"}`))
	}))
	defer srv.Close()

	if _, err := newTestClient(srv).AddWorklog(context.Background(), "PROJ-1", "15m", nil); err != nil {
		t.Fatalf("AddWorklog: %v", err)
	}
	if _, ok := gotBody["comment"]; ok {
		t.Errorf("AddWorklog nil-comment body included comment = %v", gotBody["comment"])
	}
}

func TestClientListWorklogsAllFollowsPages(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/issue/PROJ-1/worklog" {
			t.Errorf("path = %q, want /issue/PROJ-1/worklog", r.URL.Path)
		}
		switch r.URL.Query().Get("startAt") {
		case "", "0":
			_, _ = w.Write([]byte(`{"startAt":0,"maxResults":1,"total":2,"worklogs":[{"id":"10000"}]}`))
		case "1":
			_, _ = w.Write([]byte(`{"startAt":1,"maxResults":1,"total":2,"worklogs":[{"id":"10001"}]}`))
		default:
			t.Errorf("unexpected startAt %q", r.URL.Query().Get("startAt"))
		}
	}))
	defer srv.Close()

	raw, err := newTestClient(srv).ListWorklogsAll(context.Background(), "PROJ-1", 0, 1)
	if err != nil {
		t.Fatalf("ListWorklogsAll: %v", err)
	}
	wp, err := Decode[WorklogPage](raw)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if len(wp.Worklogs) != 2 {
		t.Fatalf("aggregated %d worklogs, want 2", len(wp.Worklogs))
	}
}
