package e2e

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

func bbCapabilityJSON[T any](t *testing.T, p process, args ...string) T {
	t.Helper()
	r := p.run(t, "bb", append(args, "--json")...)
	success(t, r)
	var value T
	if err := json.Unmarshal([]byte(r.stdout), &value); err != nil {
		t.Fatalf("invalid JSON: %v: %s", err, r.stdout)
	}
	return value
}

func TestBitbucketPipelineProcessLifecycle(t *testing.T) {
	const base = "/repositories/acme/fixture/pipelines"
	type pipeline struct {
		UUID  string `json:"uuid"`
		Build int    `json:"build_number"`
		State struct {
			Name   string `json:"name"`
			Result struct {
				Name string `json:"name"`
			} `json:"result"`
		} `json:"state"`
	}
	var mu sync.Mutex
	state := ""
	stopped := false
	listStarts := 0
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		if r.Header.Get("Authorization") != "Bearer "+dummyToken {
			t.Error("missing fixture authentication")
		}
		current := func() string {
			result := ""
			if stopped {
				result = `,"result":{"name":"STOPPED"}`
			}
			return fmt.Sprintf(`{"uuid":"{run-42}","build_number":42,"state":{"name":%q%s}}`, state, result)
		}
		switch r.Method + " " + r.URL.Path {
		case "POST " + base:
			var body struct {
				Target struct {
					Type    string `json:"type"`
					RefType string `json:"ref_type"`
					RefName string `json:"ref_name"`
				} `json:"target"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Error(err)
			}
			if body.Target.Type != "pipeline_ref_target" || body.Target.RefType != "tag" || body.Target.RefName != "release/e2e" {
				t.Errorf("wrong pipeline target: %+v", body)
			}
			if state != "" {
				t.Error("pipeline started twice")
			}
			state = "PENDING"
			w.WriteHeader(http.StatusCreated)
			fmt.Fprint(w, current())
		case "GET " + base + "/":
			if r.URL.Query().Get("page") == "2" {
				fmt.Fprintf(w, `{"values":[%s]}`, current())
				return
			}
			listStarts++
			if listStarts == 1 && (r.URL.Query().Get("status") != "PENDING" || r.URL.Query().Get("pagelen") != "1") {
				t.Errorf("explicit pipeline list lost filter/page size: %s", r.URL.RawQuery)
			}
			if r.URL.Query().Get("sort") != "-created_on" {
				t.Errorf("missing newest-first sort: %s", r.URL.RawQuery)
			}
			if status := r.URL.Query().Get("status"); status != "" && status != "PENDING" {
				t.Errorf("wrong status filter: %s", status)
			}
			fmt.Fprintf(w, `{"values":[{"uuid":"{other}","build_number":43}],"next":%q}`, srv.URL+base+"/?page=2")
		case "GET " + base + "/{run-42}":
			fmt.Fprint(w, current())
		case "GET " + base + "/{run-42}/steps/":
			if r.URL.Query().Get("page") == "2" {
				fmt.Fprint(w, `{"values":[{"uuid":"{step-2}","name":"Verify","state":{"name":"COMPLETED","result":{"name":"SUCCESSFUL"}}}]}`)
				return
			}
			if r.URL.Query().Get("pagelen") != "1" {
				t.Error("steps did not force page size 1")
			}
			fmt.Fprintf(w, `{"values":[{"uuid":"{step-1}","name":"Build"}],"next":%q}`, srv.URL+base+"/{run-42}/steps/?page=2")
		case "GET " + base + "/{run-42}/steps/{step-2}/log":
			if r.Header.Get("Accept") != "*/*" {
				t.Error("log did not request raw content")
			}
			fmt.Fprint(w, "CLI_E2E_MARKER\n\x00tail\n")
		case "POST " + base + "/{run-42}/stopPipeline":
			state = "COMPLETED"
			stopped = true
			w.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	p := isolated(t)
	p.profile(t, "bb", srv.URL)
	run := bbCapabilityJSON[pipeline](t, p, "pipeline", "run", "--repo", "acme/fixture", "--ref", "release/e2e", "--ref-type", "tag")
	if run.UUID != "{run-42}" || run.Build != 42 || run.State.Name != "PENDING" {
		t.Fatalf("wrong triggered pipeline: %+v", run)
	}
	page := bbCapabilityJSON[struct {
		Values []pipeline `json:"values"`
	}](t, p, "pipeline", "list", "--repo", "acme/fixture", "--all", "--limit", "1", "--status", "pending")
	if len(page.Values) != 2 || page.Values[0].Build != 43 || page.Values[1].UUID != run.UUID {
		t.Fatalf("pipeline pagination lost IDs: %+v", page)
	}
	viewed := bbCapabilityJSON[pipeline](t, p, "pipeline", "view", "42", "--repo", "acme/fixture")
	if viewed.UUID != run.UUID || viewed.State.Name != "PENDING" {
		t.Fatalf("numeric lookup did not cross page boundary: %+v", viewed)
	}
	steps := bbCapabilityJSON[struct {
		Values []struct {
			UUID string `json:"uuid"`
			Name string `json:"name"`
		} `json:"values"`
	}](t, p, "pipeline", "steps", "run-42", "--repo", "acme/fixture", "--all", "--limit", "1")
	if len(steps.Values) != 2 || steps.Values[0].UUID != "{step-1}" || steps.Values[1].UUID != "{step-2}" || steps.Values[1].Name != "Verify" {
		t.Fatalf("wrong paginated steps: %+v", steps)
	}
	log := p.run(t, "bb", "pipeline", "log", "run-42", "step-2", "--repo", "acme/fixture")
	success(t, log)
	if log.stdout != "CLI_E2E_MARKER\n\x00tail\n" {
		t.Fatalf("log bytes changed: %q", log.stdout)
	}
	stop := bbCapabilityJSON[struct {
		UUID    string `json:"uuid"`
		Stopped bool   `json:"stopped"`
	}](t, p, "pipeline", "stop", "42", "--repo", "acme/fixture")
	if stop.UUID != run.UUID || !stop.Stopped {
		t.Fatalf("wrong stop result: %+v", stop)
	}
	viewed = bbCapabilityJSON[pipeline](t, p, "pipeline", "view", "run-42", "--repo", "acme/fixture")
	if viewed.State.Name != "COMPLETED" || viewed.State.Result.Name != "STOPPED" {
		t.Fatalf("stop not reflected in readback: %+v", viewed)
	}
	failure(t, p.run(t, "bb", "pipeline", "view", "999", "--repo", "acme/fixture", "--json"), 6, "not_found_or_not_visible")
}

func TestBitbucketDeploymentEnvironmentProcess(t *testing.T) {
	for _, family := range []string{"environment", "deployment"} {
		t.Run(family, func(t *testing.T) {
			base := "/repositories/acme/fixture/" + family + "s/"
			var requests atomic.Int32
			var srv *httptest.Server
			srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				if r.Method != "GET" {
					t.Errorf("read used %s", r.Method)
				}
				item := func(id string) string {
					if family == "environment" {
						return fmt.Sprintf(`{"uuid":%q,"name":"E2E","slug":"e2e","type":"deployment_environment"}`, id)
					}
					return fmt.Sprintf(`{"uuid":%q,"state":{"name":"COMPLETED"},"environment":{"uuid":"{env-1}","name":"E2E"},"release":{"name":"build-42"}}`, id)
				}
				switch r.URL.Path {
				case base:
					if r.URL.Query().Get("page") == "2" {
						fmt.Fprintf(w, `{"values":[%s]}`, item("{second}"))
						return
					}
					if r.URL.Query().Get("pagelen") != "1" {
						t.Error("list must force page size 1")
					}
					fmt.Fprintf(w, `{"values":[%s],"next":%q}`, item("{first}"), srv.URL+base+"?page=2")
				case base + "{first}":
					if !strings.HasSuffix(r.URL.EscapedPath(), "%7Bfirst%7D") {
						t.Errorf("UUID not escaped: %s", r.URL.EscapedPath())
					}
					fmt.Fprint(w, item("{first}"))
				default:
					http.NotFound(w, r)
				}
			}))
			defer srv.Close()
			p := isolated(t)
			p.profile(t, "bb", srv.URL)
			type item struct {
				UUID        string `json:"uuid"`
				Name        string `json:"name"`
				Environment struct {
					UUID string `json:"uuid"`
				} `json:"environment"`
				State struct {
					Name string `json:"name"`
				} `json:"state"`
			}
			page := bbCapabilityJSON[struct {
				Values []item `json:"values"`
			}](t, p, family, "list", "--repo", "acme/fixture", "--all", "--limit", "1")
			if len(page.Values) != 2 || page.Values[0].UUID != "{first}" || page.Values[1].UUID != "{second}" {
				t.Fatalf("bad %s pagination: %+v", family, page)
			}
			viewed := bbCapabilityJSON[item](t, p, family, "view", "first", "--repo", "acme/fixture")
			if viewed.UUID != "{first}" {
				t.Fatalf("wrong UUID: %+v", viewed)
			}
			if family == "environment" && viewed.Name != "E2E" {
				t.Fatalf("environment metadata lost: %+v", viewed)
			}
			if family == "deployment" && (viewed.Environment.UUID != "{env-1}" || viewed.State.Name != "COMPLETED") {
				t.Fatalf("deployment association/state lost: %+v", viewed)
			}
			failure(t, p.run(t, "bb", family, "view", "missing", "--repo", "acme/fixture", "--json"), 6, "not_found_or_not_visible")
			if requests.Load() != 4 {
				t.Fatalf("unexpected request count %d", requests.Load())
			}
		})
	}
}

func TestBitbucketProjectProcessLifecycle(t *testing.T) {
	var mu sync.Mutex
	var project map[string]any
	var writes atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		switch r.Method + " " + r.URL.Path {
		case "POST /workspaces/acme/projects":
			writes.Add(1)
			if err := json.NewDecoder(r.Body).Decode(&project); err != nil {
				t.Error(err)
			}
			want := map[string]any{"key": "E2E", "name": "E2E Project", "description": "Private fixture", "is_private": true}
			if !reflect.DeepEqual(project, want) {
				t.Errorf("wrong project creation body: %+v", project)
			}
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(project)
		case "GET /workspaces/acme/projects/E2E":
			if project == nil {
				http.NotFound(w, r)
				return
			}
			_ = json.NewEncoder(w).Encode(project)
		case "DELETE /workspaces/acme/projects/E2E":
			writes.Add(1)
			project = nil
			w.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	p := isolated(t)
	p.profile(t, "bb", srv.URL)
	failure(t, p.run(t, "bb", "project", "delete", "E2E", "--workspace", "acme", "--json"), 8, "invalid_input")
	if writes.Load() != 0 {
		t.Fatal("unconfirmed project deletion sent a write")
	}
	created := bbCapabilityJSON[map[string]any](t, p, "project", "create", "E2E", "--workspace", "acme", "--name", "E2E Project", "--description", "Private fixture", "--private")
	readback := bbCapabilityJSON[map[string]any](t, p, "project", "view", "E2E", "--workspace", "acme")
	if !reflect.DeepEqual(created, readback) || readback["key"] != "E2E" || readback["is_private"] != true {
		t.Fatalf("project readback lost creation: %+v", readback)
	}
	deleted := bbCapabilityJSON[struct {
		Resource string `json:"resource"`
		ID       string `json:"id"`
		Deleted  bool   `json:"deleted"`
	}](t, p, "project", "delete", "E2E", "--workspace", "acme", "--yes")
	if !deleted.Deleted || deleted.ID != "E2E" || deleted.Resource != "project" {
		t.Fatalf("wrong delete result: %+v", deleted)
	}
	failure(t, p.run(t, "bb", "project", "view", "E2E", "--workspace", "acme", "--json"), 6, "not_found_or_not_visible")
	if writes.Load() != 2 {
		t.Fatalf("wrong write count: %d", writes.Load())
	}
}

func TestBitbucketIndependentApprovalProcess(t *testing.T) {
	var mu sync.Mutex
	approved := false
	const base = "/repositories/acme/fixture/pullrequests/17"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		identity := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		switch r.Method + " " + r.URL.Path {
		case "GET " + base:
			fmt.Fprintf(w, `{"id":17,"author":{"uuid":"{author}"},"participants":[{"user":{"uuid":"{reviewer}"},"approved":%t}]}`, approved)
		case "POST " + base + "/approve":
			if identity != "fixture-reviewer-token" {
				w.WriteHeader(http.StatusForbidden)
				fmt.Fprint(w, `{"message":"author cannot approve own PR"}`)
				return
			}
			approved = true
			fmt.Fprint(w, `{"user":{"uuid":"{reviewer}"},"approved":true,"role":"PARTICIPANT"}`)
		case "DELETE " + base + "/approve":
			if identity != "fixture-reviewer-token" {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			approved = false
			w.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	author := isolated(t)
	author.profile(t, "bb", srv.URL)
	reviewer := isolated(t)
	reviewer.profile(t, "bb", srv.URL)
	for i, entry := range reviewer.env {
		if strings.HasPrefix(entry, "ATL_E2E_TOKEN=") {
			reviewer.env[i] = "ATL_E2E_TOKEN=fixture-reviewer-token"
		}
	}
	failure(t, author.run(t, "bb", "pr", "approve", "17", "--repo", "acme/fixture", "--json"), 5, "forbidden")
	denied := bbCapabilityJSON[struct {
		Participants []struct {
			Approved bool `json:"approved"`
		} `json:"participants"`
	}](t, author, "pr", "view", "17", "--repo", "acme/fixture")
	if len(denied.Participants) != 1 || denied.Participants[0].Approved {
		t.Fatal("denied self-approval changed approval state")
	}

	type participant struct {
		User struct {
			UUID string `json:"uuid"`
		} `json:"user"`
		Approved bool `json:"approved"`
	}
	approval := bbCapabilityJSON[participant](t, reviewer, "pr", "approve", "17", "--repo", "acme/fixture")
	if !approval.Approved || approval.User.UUID != "{reviewer}" {
		t.Fatalf("wrong approver: %+v", approval)
	}
	type pr struct {
		ID     int `json:"id"`
		Author struct {
			UUID string `json:"uuid"`
		} `json:"author"`
		Participants []participant `json:"participants"`
	}
	readback := bbCapabilityJSON[pr](t, author, "pr", "view", "17", "--repo", "acme/fixture")
	if readback.Author.UUID != "{author}" || len(readback.Participants) != 1 || !readback.Participants[0].Approved || readback.Participants[0].User.UUID == readback.Author.UUID {
		t.Fatalf("independent approval not visible: %+v", readback)
	}
	removed := bbCapabilityJSON[struct {
		ID     int    `json:"id"`
		Action string `json:"action"`
		Done   bool   `json:"done"`
	}](t, reviewer, "pr", "unapprove", "17", "--repo", "acme/fixture")
	if removed.ID != 17 || removed.Action != "unapprove" || !removed.Done {
		t.Fatalf("wrong unapproval result: %+v", removed)
	}
	readback = bbCapabilityJSON[pr](t, author, "pr", "view", "17", "--repo", "acme/fixture")
	if len(readback.Participants) != 1 || readback.Participants[0].Approved {
		t.Fatalf("unapproval not visible: %+v", readback)
	}
}

func TestBitbucketPRSearchProcess(t *testing.T) {
	const query = `state="OPEN" AND title ~ "CLI E2E"`
	const base = "/repositories/acme/fixture/pullrequests"
	var requests atomic.Int32
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if r.Method != "GET" || r.URL.Path != base {
			t.Errorf("wrong search request %s %s", r.Method, r.URL.Path)
		}
		if r.URL.Query().Get("q") == "invalid expression" {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprint(w, `{"message":"Invalid query"}`)
			return
		}
		if r.URL.Query().Get("page") == "2" {
			fmt.Fprint(w, `{"values":[{"id":29,"state":"OPEN","title":"CLI E2E second"}]}`)
			return
		}
		if r.URL.Query().Get("q") != query || r.URL.Query().Get("sort") != "id" || r.URL.Query().Get("pagelen") != "1" {
			t.Errorf("lost filter/sort/page size: %s", r.URL.RawQuery)
		}
		fmt.Fprintf(w, `{"values":[{"id":17,"state":"OPEN","title":"CLI E2E first"}],"next":%q}`, srv.URL+base+"?page=2")
	}))
	defer srv.Close()
	p := isolated(t)
	p.profile(t, "bb", srv.URL)
	page := bbCapabilityJSON[struct {
		Values []struct {
			ID    int    `json:"id"`
			State string `json:"state"`
			Title string `json:"title"`
		} `json:"values"`
	}](t, p, "search", "prs", query, "--repo", "acme/fixture", "--sort", "id", "--all", "--limit", "1")
	if len(page.Values) != 2 || page.Values[0].ID != 17 || page.Values[1].ID != 29 {
		t.Fatalf("query pagination lost/duplicated known IDs: %+v", page)
	}
	for _, pr := range page.Values {
		if pr.State != "OPEN" || !strings.HasPrefix(pr.Title, "CLI E2E") {
			t.Fatalf("query membership violated: %+v", pr)
		}
	}
	failure(t, p.run(t, "bb", "search", "prs", "invalid expression", "--repo", "acme/fixture", "--json"), 1, "http_error")
	if requests.Load() != 3 {
		t.Fatalf("unexpected search count: %d", requests.Load())
	}
}
