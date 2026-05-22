package bitbucket

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestListDeployments(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Bitbucket requires the trailing slash on the deployments listing.
		if r.URL.Path != "/repositories/acme/widgets/deployments/" {
			t.Errorf("path = %q", r.URL.Path)
		}
		if got := r.URL.Query().Get("pagelen"); got != "5" {
			t.Errorf("pagelen = %q, want 5", got)
		}
		_, _ = w.Write([]byte(`{"values":[{"uuid":"{d-1}","state":{"name":"COMPLETED"},"environment":{"name":"Production"}}]}`))
	}))
	defer srv.Close()

	raw, err := newTestClient(srv).ListDeployments(context.Background(), "acme", "widgets", 5)
	if err != nil {
		t.Fatalf("ListDeployments: %v", err)
	}
	page, err := Decode[DeploymentPage](raw)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if len(page.Values) != 1 || page.Values[0].UUID != "{d-1}" {
		t.Fatalf("values = %+v", page.Values)
	}
	if page.Values[0].Environment == nil || page.Values[0].Environment.Name != "Production" {
		t.Fatalf("environment = %+v", page.Values[0].Environment)
	}
}

func TestGetDeploymentNormalizesUUID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// A bare UUID must be brace-wrapped before the request is sent.
		if got := r.URL.EscapedPath(); got != "/repositories/acme/widgets/deployments/%7Bd-1%7D" {
			t.Errorf("escaped path = %q", got)
		}
		_, _ = w.Write([]byte(`{"uuid":"{d-1}","state":{"name":"COMPLETED"}}`))
	}))
	defer srv.Close()

	raw, err := newTestClient(srv).GetDeployment(context.Background(), "acme", "widgets", "d-1")
	if err != nil {
		t.Fatalf("GetDeployment: %v", err)
	}
	d, err := Decode[Deployment](raw)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if d.State == nil || d.State.Name != "COMPLETED" {
		t.Fatalf("deployment = %+v", d)
	}
}

func TestGetDeploymentRequiresUUID(t *testing.T) {
	// An empty UUID is rejected before any request is issued; the server must
	// never be hit.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("server should not be called for an empty UUID; got %s", r.URL.Path)
	}))
	defer srv.Close()

	if _, err := newTestClient(srv).GetDeployment(context.Background(), "acme", "widgets", "  "); err == nil {
		t.Fatal("expected an error for an empty UUID")
	}
}

func TestListEnvironments(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repositories/acme/widgets/environments/" {
			t.Errorf("path = %q", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"values":[{"uuid":"{e-1}","name":"Production","type":"deployment_environment"}]}`))
	}))
	defer srv.Close()

	raw, err := newTestClient(srv).ListEnvironments(context.Background(), "acme", "widgets", 0)
	if err != nil {
		t.Fatalf("ListEnvironments: %v", err)
	}
	page, err := Decode[EnvironmentPage](raw)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if len(page.Values) != 1 || page.Values[0].Name != "Production" {
		t.Fatalf("values = %+v", page.Values)
	}
}

func TestGetEnvironment(t *testing.T) {
	srv := serveJSON(t, "/repositories/acme/widgets/environments/{e-1}",
		`{"uuid":"{e-1}","name":"Staging","slug":"staging"}`)
	defer srv.Close()

	raw, err := newTestClient(srv).GetEnvironment(context.Background(), "acme", "widgets", "{e-1}")
	if err != nil {
		t.Fatalf("GetEnvironment: %v", err)
	}
	env, err := Decode[Environment](raw)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if env.Name != "Staging" || env.Slug != "staging" {
		t.Fatalf("environment = %+v", env)
	}
}

func TestListDeploymentsAllFollowsNext(t *testing.T) {
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Query().Get("page") {
		case "", "1":
			_, _ = w.Write([]byte(`{"values":[{"uuid":"{a}"}],"next":"` +
				srv.URL + `/repositories/acme/widgets/deployments/?page=2"}`))
		case "2":
			_, _ = w.Write([]byte(`{"values":[{"uuid":"{b}"}]}`))
		default:
			t.Errorf("unexpected page %q", r.URL.Query().Get("page"))
		}
	}))
	defer srv.Close()

	raw, err := newTestClient(srv).ListDeploymentsAll(context.Background(), "acme", "widgets", 0)
	if err != nil {
		t.Fatalf("ListDeploymentsAll: %v", err)
	}
	page, err := Decode[DeploymentPage](raw)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if len(page.Values) != 2 || page.Values[0].UUID != "{a}" || page.Values[1].UUID != "{b}" {
		t.Fatalf("values = %+v", page.Values)
	}
}
