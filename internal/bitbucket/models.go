package bitbucket

import (
	"encoding/json"

	"github.com/aurokin/atlassian-cli/internal/restutil"
)

// CurrentUser is the subset of the authenticated Bitbucket account that human
// output renders (GET /user).
type CurrentUser struct {
	AccountID   string `json:"account_id"`
	DisplayName string `json:"display_name"`
	Username    string `json:"username,omitempty"`
	Nickname    string `json:"nickname,omitempty"`
	UUID        string `json:"uuid,omitempty"`
}

// Branch is a Bitbucket ref reduced to its name, the shape shared by a
// repository's main branch and ref listings.
type Branch struct {
	Name string `json:"name"`
}

// Project is a Bitbucket project — both the sub-object a repository belongs to
// and the standalone resource the `project` commands render.
type Project struct {
	Key         string `json:"key,omitempty"`
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	IsPrivate   bool   `json:"is_private,omitempty"`
	UUID        string `json:"uuid,omitempty"`
}

// ProjectPage is one page of a project listing. Bitbucket paginates with an
// absolute "next" URL; an empty Next marks the last page.
type ProjectPage struct {
	Values []Project `json:"values"`
	Next   string    `json:"next,omitempty"`
}

// Workspace is the subset of a Bitbucket workspace that human output renders.
type Workspace struct {
	Slug      string `json:"slug"`
	Name      string `json:"name,omitempty"`
	UUID      string `json:"uuid,omitempty"`
	IsPrivate bool   `json:"is_private,omitempty"`
	CreatedOn string `json:"created_on,omitempty"`
}

// WorkspacePage is one page of a workspace listing.
type WorkspacePage struct {
	Values []Workspace `json:"values"`
	Next   string      `json:"next,omitempty"`
}

// Repository is the subset of a Bitbucket repository that human output
// renders.
type Repository struct {
	UUID        string   `json:"uuid"`
	FullName    string   `json:"full_name"`
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	IsPrivate   bool     `json:"is_private"`
	MainBranch  *Branch  `json:"mainbranch,omitempty"`
	Project     *Project `json:"project,omitempty"`
}

// RepositoryPage is one page of a Bitbucket repository listing. Bitbucket
// paginates with an absolute "next" URL; an empty Next marks the last page.
type RepositoryPage struct {
	Values []Repository `json:"values"`
	Next   string       `json:"next,omitempty"`
}

// Account is a Bitbucket user reduced to the fields human output renders.
type Account struct {
	DisplayName string `json:"display_name,omitempty"`
	Nickname    string `json:"nickname,omitempty"`
	AccountID   string `json:"account_id,omitempty"`
}

// PullRequestRef is one side (source or destination) of a pull request,
// reduced to its branch name.
type PullRequestRef struct {
	Branch *Branch `json:"branch,omitempty"`
}

// PullRequest is the subset of a Bitbucket pull request that human output
// renders.
type PullRequest struct {
	ID          int             `json:"id"`
	Title       string          `json:"title"`
	State       string          `json:"state"`
	Author      *Account        `json:"author,omitempty"`
	Source      *PullRequestRef `json:"source,omitempty"`
	Destination *PullRequestRef `json:"destination,omitempty"`
}

// PullRequestPage is one page of a pull-request listing. Bitbucket paginates
// with an absolute "next" URL; an empty Next marks the last page.
type PullRequestPage struct {
	Values []PullRequest `json:"values"`
	Next   string        `json:"next,omitempty"`
}

// PipelineResult is the outcome of a finished pipeline state.
type PipelineResult struct {
	Name string `json:"name,omitempty"`
}

// PipelineState is a pipeline run's lifecycle state.
type PipelineState struct {
	Name   string          `json:"name,omitempty"`
	Result *PipelineResult `json:"result,omitempty"`
}

// PipelineTarget is what a pipeline run was triggered against.
type PipelineTarget struct {
	RefType string `json:"ref_type,omitempty"`
	RefName string `json:"ref_name,omitempty"`
}

// Pipeline is the subset of a Bitbucket pipeline run that human output
// renders.
type Pipeline struct {
	UUID        string          `json:"uuid"`
	BuildNumber int             `json:"build_number,omitempty"`
	State       *PipelineState  `json:"state,omitempty"`
	Target      *PipelineTarget `json:"target,omitempty"`
	Creator     *Account        `json:"creator,omitempty"`
	CreatedOn   string          `json:"created_on,omitempty"`
}

// PipelinePage is one page of a pipeline listing. Bitbucket paginates with an
// absolute "next" URL; an empty Next marks the last page.
type PipelinePage struct {
	Values []Pipeline `json:"values"`
	Next   string     `json:"next,omitempty"`
}

// IssueContent is an issue's body, reduced to its raw markup.
type IssueContent struct {
	Raw string `json:"raw,omitempty"`
}

// Issue is the subset of a Bitbucket repository issue that human output
// renders.
type Issue struct {
	ID        int           `json:"id"`
	Title     string        `json:"title"`
	State     string        `json:"state,omitempty"`
	Kind      string        `json:"kind,omitempty"`
	Priority  string        `json:"priority,omitempty"`
	Content   *IssueContent `json:"content,omitempty"`
	Reporter  *Account      `json:"reporter,omitempty"`
	Assignee  *Account      `json:"assignee,omitempty"`
	CreatedOn string        `json:"created_on,omitempty"`
}

// IssuePage is one page of an issue listing. Bitbucket paginates with an
// absolute "next" URL; an empty Next marks the last page.
type IssuePage struct {
	Values []Issue `json:"values"`
	Next   string  `json:"next,omitempty"`
}

// Decode unmarshals a raw Bitbucket API body into a model value, wrapping a
// decode failure as a structured error.
func Decode[T any](raw json.RawMessage) (T, error) {
	return restutil.Decode[T](raw, productName)
}
