package bitbucket

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/aurokin/atlassian-cli/internal/apperr"
	"github.com/aurokin/atlassian-cli/internal/restutil"
)

// pipelinesListBase returns the pipelines collection path used for listing.
// Bitbucket requires the trailing slash on this endpoint.
func pipelinesListBase(workspace, repo string) string {
	return "/repositories/" + url.PathEscape(workspace) + "/" + url.PathEscape(repo) + "/pipelines/"
}

// pipelinesTriggerBase returns the pipelines collection path used for
// triggering a run (no trailing slash, per the Bitbucket API).
func pipelinesTriggerBase(workspace, repo string) string {
	return "/repositories/" + url.PathEscape(workspace) + "/" + url.PathEscape(repo) + "/pipelines"
}

// pipelinesQuery assembles the list query: page size, newest-first sort, and
// an optional status filter (Bitbucket's pipeline state name).
func pipelinesQuery(status string, limit int) url.Values {
	q := url.Values{}
	setLimit(q, limit)
	q.Set("sort", "-created_on")
	if status != "" {
		q.Set("status", status)
	}
	return q
}

// ListPipelines returns one page of a repository's pipeline runs, newest
// first (GET /repositories/{ws}/{repo}/pipelines/). status filters by pipeline
// state name (e.g. PENDING, IN_PROGRESS, COMPLETED); an empty status lists all.
func (c *Client) ListPipelines(ctx context.Context, workspace, repo, status string, limit int) (json.RawMessage, error) {
	return c.Get(ctx, restutil.WithQuery(pipelinesListBase(workspace, repo), pipelinesQuery(status, limit)))
}

// ListPipelinesAll follows a repository's pipeline listing to completion and
// returns an aggregated {"values": [...]} body.
func (c *Client) ListPipelinesAll(ctx context.Context, workspace, repo, status string, limit int) (json.RawMessage, error) {
	return c.followValues(ctx, restutil.WithQuery(pipelinesListBase(workspace, repo), pipelinesQuery(status, limit)))
}

// GetPipeline returns a single pipeline run by UUID
// (GET /repositories/{ws}/{repo}/pipelines/{uuid}). The UUID is normalized to
// the brace-wrapped form Bitbucket expects.
func (c *Client) GetPipeline(ctx context.Context, workspace, repo, uuid string) (json.RawMessage, error) {
	norm := NormalizePipelineUUID(uuid)
	if norm == "" {
		return nil, apperr.InvalidInput("a pipeline UUID is required")
	}
	return c.Get(ctx, pipelinesListBase(workspace, repo)+url.PathEscape(norm))
}

// GetPipelineByBuildNumber finds a pipeline run by its build number by paging
// newest-first and returning the matching raw value. It stops at
// restutil.MaxFollowPages and reports not_found when no run matches.
func (c *Client) GetPipelineByBuildNumber(ctx context.Context, workspace, repo string, buildNumber int) (json.RawMessage, error) {
	if buildNumber <= 0 {
		return nil, apperr.InvalidInput("a pipeline build number must be a positive integer")
	}
	next := restutil.WithQuery(pipelinesListBase(workspace, repo), pipelinesQuery("", 50))
	for page := 0; page < restutil.MaxFollowPages && next != ""; page++ {
		raw, err := c.Get(ctx, next)
		if err != nil {
			return nil, err
		}
		var pg struct {
			Values []json.RawMessage `json:"values"`
			Next   string            `json:"next"`
		}
		if err := json.Unmarshal(raw, &pg); err != nil {
			return nil, decodeError(err)
		}
		for _, item := range pg.Values {
			var probe struct {
				BuildNumber int `json:"build_number"`
			}
			if err := json.Unmarshal(item, &probe); err != nil {
				return nil, decodeError(err)
			}
			if probe.BuildNumber == buildNumber {
				return item, nil
			}
		}
		next = pg.Next
	}
	return nil, apperr.NotFoundOrNotVisible(fmt.Sprintf("pipeline #%d was not found", buildNumber))
}

// TriggerPipeline starts a new pipeline run for a ref
// (POST /repositories/{ws}/{repo}/pipelines). refType defaults to "branch".
func (c *Client) TriggerPipeline(ctx context.Context, workspace, repo, refType, refName string) (json.RawMessage, error) {
	refType = strings.TrimSpace(refType)
	if refType == "" {
		refType = "branch"
	}
	if strings.TrimSpace(refName) == "" {
		return nil, apperr.InvalidInput("a ref name is required to trigger a pipeline")
	}
	body := map[string]any{
		"target": map[string]any{
			"type":     "pipeline_ref_target",
			"ref_type": refType,
			"ref_name": refName,
		},
	}
	return c.Send(ctx, "POST", pipelinesTriggerBase(workspace, repo), body)
}

// pipelineUUIDBase returns the path of a single pipeline run by UUID, the base
// for its sub-resources (steps, stop). uuid must already be brace-normalized.
func pipelineUUIDBase(workspace, repo, uuid string) string {
	return pipelinesListBase(workspace, repo) + url.PathEscape(uuid)
}

// StopPipeline stops an in-progress pipeline run
// (POST .../pipelines/{uuid}/stopPipeline). The API returns no body.
func (c *Client) StopPipeline(ctx context.Context, workspace, repo, uuid string) error {
	norm := NormalizePipelineUUID(uuid)
	if norm == "" {
		return apperr.InvalidInput("a pipeline UUID is required")
	}
	_, err := c.Send(ctx, "POST", pipelineUUIDBase(workspace, repo, norm)+"/stopPipeline", nil)
	return err
}

// ListPipelineSteps returns one page of a pipeline run's steps
// (GET .../pipelines/{uuid}/steps/). uuid is normalized to the brace-wrapped
// form Bitbucket expects.
func (c *Client) ListPipelineSteps(ctx context.Context, workspace, repo, uuid string, limit int) (json.RawMessage, error) {
	norm := NormalizePipelineUUID(uuid)
	if norm == "" {
		return nil, apperr.InvalidInput("a pipeline UUID is required")
	}
	q := url.Values{}
	setLimit(q, limit)
	return c.Get(ctx, restutil.WithQuery(pipelineUUIDBase(workspace, repo, norm)+"/steps/", q))
}

// ListPipelineStepsAll follows a pipeline's step listing to completion and
// returns an aggregated {"values": [...]} body.
func (c *Client) ListPipelineStepsAll(ctx context.Context, workspace, repo, uuid string, limit int) (json.RawMessage, error) {
	norm := NormalizePipelineUUID(uuid)
	if norm == "" {
		return nil, apperr.InvalidInput("a pipeline UUID is required")
	}
	q := url.Values{}
	setLimit(q, limit)
	return c.followValues(ctx, restutil.WithQuery(pipelineUUIDBase(workspace, repo, norm)+"/steps/", q))
}

// GetPipelineStepLog returns the raw log output of a pipeline step
// (GET .../pipelines/{uuid}/steps/{stepUUID}/log). Both UUIDs are normalized to
// the brace-wrapped form. A step that has produced no log yet yields an empty
// body or a not_found from the API.
func (c *Client) GetPipelineStepLog(ctx context.Context, workspace, repo, uuid, stepUUID string) ([]byte, error) {
	norm := NormalizePipelineUUID(uuid)
	stepNorm := NormalizePipelineUUID(stepUUID)
	if norm == "" || stepNorm == "" {
		return nil, apperr.InvalidInput("a pipeline UUID and step UUID are required")
	}
	return c.GetAccepting(ctx,
		pipelineUUIDBase(workspace, repo, norm)+"/steps/"+url.PathEscape(stepNorm)+"/log", "*/*")
}

// NormalizePipelineUUID wraps a bare pipeline UUID in the braces Bitbucket
// expects, leaving an already-wrapped or empty value unchanged.
func NormalizePipelineUUID(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if strings.HasPrefix(value, "{") && strings.HasSuffix(value, "}") {
		return value
	}
	return "{" + strings.Trim(value, "{}") + "}"
}

// PipelineRef reports whether ref is a positive build number (returning it) or
// should be treated as a UUID.
func PipelineRef(ref string) (buildNumber int, isBuildNumber bool) {
	n, err := strconv.Atoi(strings.TrimSpace(ref))
	if err == nil && n > 0 {
		return n, true
	}
	return 0, false
}
