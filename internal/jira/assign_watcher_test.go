package jira

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClientAssignIssue(t *testing.T) {
	var gotBody map[string]json.RawMessage
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("method = %q, want PUT", r.Method)
		}
		if r.URL.Path != "/issue/PROJ-1/assignee" {
			t.Errorf("path = %q, want /issue/PROJ-1/assignee", r.URL.Path)
		}
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &gotBody)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	id := "abc123"
	if err := newTestClient(srv).AssignIssue(context.Background(), "PROJ-1", &id); err != nil {
		t.Fatalf("AssignIssue: %v", err)
	}
	if string(gotBody["accountId"]) != `"abc123"` {
		t.Errorf("AssignIssue body accountId = %s, want \"abc123\"", gotBody["accountId"])
	}
}

func TestClientAssignIssueUnassign(t *testing.T) {
	var gotBody map[string]json.RawMessage
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &gotBody)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	if err := newTestClient(srv).AssignIssue(context.Background(), "PROJ-1", nil); err != nil {
		t.Fatalf("AssignIssue: %v", err)
	}
	v, ok := gotBody["accountId"]
	if !ok {
		t.Fatal("AssignIssue body missing accountId key")
	}
	if string(v) != "null" {
		t.Errorf("AssignIssue body accountId = %s, want null", v)
	}
}

func TestClientAddWatcher(t *testing.T) {
	var (
		gotMethod string
		gotBody   []byte
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		if r.URL.Path != "/issue/PROJ-1/watchers" {
			t.Errorf("path = %q, want /issue/PROJ-1/watchers", r.URL.Path)
		}
		gotBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	if err := newTestClient(srv).AddWatcher(context.Background(), "PROJ-1", "u1"); err != nil {
		t.Fatalf("AddWatcher: %v", err)
	}
	if gotMethod != http.MethodPost {
		t.Errorf("method = %q, want POST", gotMethod)
	}
	// The body must be the raw account id as a JSON string, not an object.
	if string(gotBody) != `"u1"` {
		t.Errorf("AddWatcher body = %s, want \"u1\"", gotBody)
	}
}

func TestClientAddWatcherCallerWhenEmpty(t *testing.T) {
	var gotBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	if err := newTestClient(srv).AddWatcher(context.Background(), "PROJ-1", ""); err != nil {
		t.Fatalf("AddWatcher: %v", err)
	}
	if len(gotBody) != 0 {
		t.Errorf("AddWatcher with empty id sent body %q, want empty so Jira adds the caller", gotBody)
	}
}

func TestClientRemoveWatcher(t *testing.T) {
	var (
		gotMethod string
		gotQuery  string
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		if r.URL.Path != "/issue/PROJ-1/watchers" {
			t.Errorf("path = %q, want /issue/PROJ-1/watchers", r.URL.Path)
		}
		gotQuery = r.URL.RawQuery
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	if err := newTestClient(srv).RemoveWatcher(context.Background(), "PROJ-1", "u1"); err != nil {
		t.Fatalf("RemoveWatcher: %v", err)
	}
	if gotMethod != http.MethodDelete {
		t.Errorf("method = %q, want DELETE", gotMethod)
	}
	if gotQuery != "accountId=u1" {
		t.Errorf("query = %q, want accountId=u1", gotQuery)
	}
}

func TestClientListWatchers(t *testing.T) {
	srv := serveJSON(t, "/issue/PROJ-1/watchers",
		`{"isWatching":true,"watchCount":1,"watchers":[{"accountId":"u1","displayName":"Alice"}]}`)
	defer srv.Close()

	raw, err := newTestClient(srv).ListWatchers(context.Background(), "PROJ-1")
	if err != nil {
		t.Fatalf("ListWatchers: %v", err)
	}
	ws, err := Decode[Watchers](raw)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if !ws.IsWatching || ws.WatchCount != 1 || len(ws.Watchers) != 1 || ws.Watchers[0].AccountID != "u1" {
		t.Fatalf("watchers = %+v", ws)
	}
}
