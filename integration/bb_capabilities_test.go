//go:build integration

package integration

import (
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
)

// Project administration is selected explicitly; ordinary repository workflows
// do not imply workspace project administration permission. Only the exact
// uniquely named project created below is eligible for deletion.
func TestBitbucketProjectAdministration(t *testing.T) {
	requireIntegration(t)
	if os.Getenv("ATL_IT_BB_PROJECT_ADMIN") != "1" {
		t.Skip("set ATL_IT_BB_PROJECT_ADMIN=1 to select live project administration")
	}
	s := bbSession(t)
	ws := bbWorkspace(t)
	key := "E2E" + strings.ToUpper(strconv.FormatInt(time.Now().UnixNano(), 36))
	name := "CLI E2E " + key
	created := s.mustRun("project", "create", key, "--workspace", ws, "--name", name, "--private", "--json")
	s.cleanupOwned("project", ws+"/"+key, func() bool {
		result := s.run("project", "delete", key, "--workspace", ws, "--yes", "--json")
		if result.err != nil {
			t.Errorf("owned project cleanup failed: %s%s", result.stdout, result.stderr)
			return false
		}
		var deleted struct {
			Resource string `json:"resource"`
			ID       string `json:"id"`
			Deleted  bool   `json:"deleted"`
		}
		if err := jsonUnmarshal(result.stdout, &deleted); err != nil || !deleted.Deleted || deleted.ID != key || deleted.Resource != "project" {
			t.Errorf("invalid project cleanup result: %s", result.stdout)
			return false
		}
		return bbRequireMissing(t, s, "project", "view", key, "--workspace", ws)
	})
	type project struct {
		Key     string `json:"key"`
		Name    string `json:"name"`
		Private bool   `json:"is_private"`
	}
	var initial project
	if err := jsonUnmarshal(created.stdout, &initial); err != nil {
		t.Fatal(err)
	}
	if initial.Key != key || initial.Name != name || !initial.Private {
		t.Fatalf("project creation identity/visibility differs: %+v", initial)
	}
	var viewed project
	s.mustJSON(&viewed, "project", "view", key, "--workspace", ws)
	if viewed != initial {
		t.Fatalf("project readback differs: %+v", viewed)
	}
}

// TestBitbucketPipelinesAndDeployments requires verified available free build
// quota before selecting ATL_IT_BB_PIPELINES=1. It plans two 1x steps capped at
// one minute each, within a three-minute build allowance. The caller must not
// enable paid overages or start this test without confirming that allowance.
//
// The CLI has no custom selector, so both branch-specific default YAML files
// are committed while Pipelines is explicitly disabled, with [skip ci] commit
// messages to suppress delayed push events when enablement becomes visible.
// No commits are made after enablement. The marker permits manual triggers:
// https://support.atlassian.com/bitbucket-cloud/kb/how-to-skip-triggering-an-automatic-pipeline-build-using-skip-ci-label/
// Setup API references:
// https://developer.atlassian.com/cloud/bitbucket/rest/api-group-pipelines/
// https://support.atlassian.com/bitbucket-cloud/kb/api-to-create-bitbucket-cloud-pipeline-deployment-environment/
// https://support.atlassian.com/bitbucket-cloud/docs/step-options/
func TestBitbucketPipelinesAndDeployments(t *testing.T) {
	requireIntegration(t)
	if os.Getenv("ATL_IT_BB_PIPELINES") != "1" {
		t.Skip("set ATL_IT_BB_PIPELINES=1 only after verifying at least three available free build minutes")
	}
	s := bbSession(t)
	target := ownedBBRepo(t, s)
	apiBase := "/repositories/" + target
	type pipeline struct {
		UUID  string `json:"uuid"`
		Build int    `json:"build_number"`
		State struct {
			Name   string `json:"name"`
			Result struct {
				Name string `json:"name"`
			} `json:"result"`
		} `json:"state"`
		Target struct {
			RefName string `json:"ref_name"`
			Commit  struct {
				Hash string `json:"hash"`
			} `json:"commit"`
		} `json:"target"`
	}
	// Registered after repository cleanup so active owned builds stop first.
	// The scan is limited to this exact newly-created repository and also catches
	// a server-side creation whose response could not be decoded.
	t.Cleanup(func() {
		result := s.run("pipeline", "list", "--repo", target, "--all", "--json", "--timeout", "15s")
		if result.err != nil {
			t.Errorf("cannot reconcile owned pipeline cleanup: %s%s", result.stdout, result.stderr)
			return
		}
		var page struct {
			Values []pipeline `json:"values"`
		}
		if err := jsonUnmarshal(result.stdout, &page); err != nil {
			t.Error(err)
			return
		}
		for _, run := range page.Values {
			if run.State.Name == "COMPLETED" {
				t.Logf("owned pipeline cleanup UUID=%s already terminal result=%s", run.UUID, run.State.Result.Name)
				continue
			}
			if run.UUID == "" {
				t.Error("owned pipeline missing UUID for stop cleanup")
				continue
			}
			stopped := s.run("pipeline", "stop", run.UUID, "--repo", target, "--timeout", "15s")
			if stopped.err != nil {
				t.Errorf("cannot stop owned pipeline %s: %s%s", run.UUID, stopped.stdout, stopped.stderr)
				continue
			}
			terminal := false
			for deadline := time.Now().Add(time.Minute); time.Now().Before(deadline); {
				result := s.run("pipeline", "view", run.UUID, "--repo", target, "--json", "--timeout", "10s")
				var current pipeline
				if result.err != nil || jsonUnmarshal(result.stdout, &current) != nil {
					t.Errorf("cannot verify owned pipeline %s stop: %s%s", run.UUID, result.stdout, result.stderr)
					break
				}
				if current.State.Name == "COMPLETED" {
					t.Logf("owned pipeline cleanup UUID=%s terminal result=%s", run.UUID, current.State.Result.Name)
					terminal = true
					break
				}
				time.Sleep(2 * time.Second)
			}
			if !terminal {
				t.Errorf("owned pipeline %s did not reach terminal state before repository cleanup", run.UUID)
			}
		}
	})
	var configuration struct {
		Enabled bool `json:"enabled"`
	}
	s.mustRun("api", apiBase+"/pipelines_config", "--method", "PUT", "--data", `{"enabled":false}`, "--json")
	s.mustJSON(&configuration, "api", apiBase+"/pipelines_config")
	if configuration.Enabled {
		t.Fatal("Pipelines must be disabled before uploading any fixture commits")
	}
	const marker = "ATL_CLI_E2E_PIPELINE_MARKER"
	const environmentName = "CLI E2E Test"
	fastYAML := `image: alpine:3.22
pipelines:
  default:
    - step:
        name: E2E marker deployment
        max-time: 1
        size: 1x
        deployment: CLI E2E Test
        script:
          - echo ATL_CLI_E2E_PIPELINE_MARKER
`
	slowYAML := `image: alpine:3.22
pipelines:
  default:
    - step:
        name: E2E stoppable
        max-time: 1
        size: 1x
        script:
          - echo ATL_CLI_E2E_STOP_MARKER
          - sleep 50
`
	mainCommit := seedBBCommit(t, s, target, "main", "", "bitbucket-pipelines.yml", fastYAML)
	stopCommit := seedBBCommit(t, s, target, "stop", mainCommit, "bitbucket-pipelines.yml", slowYAML)
	// No commit/source/ref writes occur below this line.
	s.mustRun("api", apiBase+"/pipelines_config", "--method", "PUT", "--data", `{"enabled":true}`, "--json")
	s.mustJSON(&configuration, "api", apiBase+"/pipelines_config")
	if !configuration.Enabled {
		t.Fatal("owned repository Pipelines enablement did not persist")
	}
	type environment struct {
		UUID string `json:"uuid"`
		Name string `json:"name"`
	}
	var env environment
	s.mustJSON(&env, "api", apiBase+"/environments/", "--method", "POST", "--data", `{"environment_type":{"name":"TEST"},"name":"CLI E2E Test"}`)
	if env.UUID == "" || env.Name != environmentName {
		t.Fatalf("environment setup did not return owned identity: %+v", env)
	}
	var envView environment
	s.mustJSON(&envView, "environment", "view", env.UUID, "--repo", target)
	if envView != env {
		t.Fatalf("environment readback differs: %+v", envView)
	}
	var environments struct {
		Values []environment `json:"values"`
	}
	// A successful environment view can precede its list-index visibility.
	// Retry only the read membership assertion; never repeat the creation.
	found := 0
	environmentDeadline := time.Now().Add(time.Minute)
	for attempt := 1; time.Now().Before(environmentDeadline); attempt++ {
		s.mustJSON(&environments, "environment", "list", "--repo", target, "--all", "--limit", "1", "--timeout", "10s")
		found = 0
		for _, entry := range environments.Values {
			if entry.UUID == env.UUID && entry.Name == environmentName {
				found++
			}
		}
		if found != 0 {
			t.Logf("owned environment visible in paginated list after %d read attempts", attempt)
			break
		}
		time.Sleep(2 * time.Second)
	}
	if found != 1 {
		t.Fatalf("owned environment not uniquely listed: %+v", environments)
	}
	var runs struct {
		Values []pipeline `json:"values"`
	}
	s.mustJSON(&runs, "pipeline", "list", "--repo", target, "--all")
	if len(runs.Values) != 0 {
		t.Fatal("unexpected automatic pipeline trigger before explicit CLI run; stopping owned builds during cleanup")
	}
	t.Log("starting at most two explicit 1x builds with max-time=1; planned allowance three free build minutes")
	waitFor := func(uuid string, done func(pipeline) bool) pipeline {
		t.Helper()
		deadline := time.Now().Add(3 * time.Minute)
		var current pipeline
		for time.Now().Before(deadline) {
			s.mustJSON(&current, "pipeline", "view", uuid, "--repo", target, "--timeout", "10s")
			if done(current) {
				return current
			}
			if current.State.Name == "COMPLETED" {
				t.Fatalf("pipeline %s completed before expected state: %s", uuid, current.State.Result.Name)
			}
			time.Sleep(2 * time.Second)
		}
		t.Fatalf("pipeline %s exceeded bounded state wait", uuid)
		return current
	}
	var fast pipeline
	s.mustJSON(&fast, "pipeline", "run", "--repo", target, "--ref", "main", "--timeout", "15s")
	if fast.UUID == "" || fast.Build == 0 {
		t.Fatal("explicit pipeline trigger returned no UUID/build number")
	}
	completed := waitFor(fast.UUID, func(run pipeline) bool { return run.State.Name == "COMPLETED" })
	if completed.State.Result.Name != "SUCCESSFUL" || completed.Target.RefName != "main" || completed.Target.Commit.Hash != mainCommit {
		t.Fatalf("marker pipeline did not succeed on exact seeded commit: %+v", completed)
	}
	type step struct {
		UUID  string `json:"uuid"`
		State struct {
			Name   string `json:"name"`
			Result struct {
				Name string `json:"name"`
			} `json:"result"`
		} `json:"state"`
	}
	var steps struct {
		Values []step `json:"values"`
	}
	s.mustJSON(&steps, "pipeline", "steps", fast.UUID, "--repo", target, "--all", "--limit", "1")
	if len(steps.Values) != 1 || steps.Values[0].UUID == "" || steps.Values[0].State.Result.Name != "SUCCESSFUL" {
		t.Fatalf("marker step not uniquely successful: %+v", steps)
	}
	log := s.mustRun("pipeline", "log", fast.UUID, steps.Values[0].UUID, "--repo", target)
	if !strings.Contains(log.stdout, marker) {
		t.Fatal("marker absent from successful pipeline log")
	}
	type deployment struct {
		UUID        string      `json:"uuid"`
		Environment environment `json:"environment"`
		State       struct {
			Name string `json:"name"`
		} `json:"state"`
	}
	var deployments struct {
		Values []deployment `json:"values"`
	}
	// This owned repository has exactly one deployment-producing pipeline.
	deadline := time.Now().Add(time.Minute)
	for time.Now().Before(deadline) {
		s.mustJSON(&deployments, "deployment", "list", "--repo", target, "--all", "--limit", "1", "--timeout", "10s")
		if len(deployments.Values) > 0 {
			break
		}
		time.Sleep(2 * time.Second)
	}
	if len(deployments.Values) != 1 || deployments.Values[0].UUID == "" || deployments.Values[0].Environment.UUID != env.UUID {
		t.Fatalf("successful pipeline deployment missing/wrong environment: %+v", deployments)
	}
	var deploymentView deployment
	s.mustJSON(&deploymentView, "deployment", "view", deployments.Values[0].UUID, "--repo", target)
	if deploymentView.UUID != deployments.Values[0].UUID || deploymentView.Environment.UUID != env.UUID || deploymentView.State.Name != "COMPLETED" {
		t.Fatalf("deployment readback differs: %+v", deploymentView)
	}
	var slow pipeline
	s.mustJSON(&slow, "pipeline", "run", "--repo", target, "--ref", "stop", "--timeout", "15s")
	if slow.UUID == "" || slow.UUID == fast.UUID {
		t.Fatal("stop fixture did not return a distinct pipeline UUID")
	}
	running := waitFor(slow.UUID, func(run pipeline) bool { return run.State.Name == "IN_PROGRESS" })
	if running.Target.RefName != "stop" || running.Target.Commit.Hash != stopCommit {
		t.Fatalf("stop fixture targets wrong commit: %+v", running)
	}
	var stopped struct {
		UUID    string `json:"uuid"`
		Stopped bool   `json:"stopped"`
	}
	s.mustJSON(&stopped, "pipeline", "stop", slow.UUID, "--repo", target, "--timeout", "15s")
	if !stopped.Stopped || stopped.UUID != slow.UUID {
		t.Fatalf("stop result differs: %+v", stopped)
	}
	ended := waitFor(slow.UUID, func(run pipeline) bool { return run.State.Name == "COMPLETED" })
	if ended.State.Result.Name != "STOPPED" {
		t.Fatalf("explicit stop did not persist: %+v", ended)
	}
	s.mustJSON(&runs, "pipeline", "list", "--repo", target, "--all", "--limit", "1")
	seen := map[string]int{}
	for _, run := range runs.Values {
		seen[run.UUID]++
	}
	if len(runs.Values) != 2 || seen[fast.UUID] != 1 || seen[slow.UUID] != 1 {
		t.Fatalf("pipeline pagination missing/duplicated explicit runs: %+v", runs)
	}
}
