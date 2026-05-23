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

// Branch is a Bitbucket ref reduced to the fields human output renders. The
// minimal {name} shape is shared by a repository's main branch and a pull
// request's source/destination; a branch listing also carries the tip commit
// in Target.
type Branch struct {
	Name   string  `json:"name"`
	Target *Commit `json:"target,omitempty"`
}

// BranchPage is one page of a branch listing. Bitbucket paginates with an
// absolute "next" URL; an empty Next marks the last page.
type BranchPage struct {
	Values []Branch `json:"values"`
	Next   string   `json:"next,omitempty"`
}

// Tag is a Bitbucket annotated or lightweight tag reduced to the fields human
// output renders. Target is the commit the tag points at.
type Tag struct {
	Name    string  `json:"name"`
	Message string  `json:"message,omitempty"`
	Date    string  `json:"date,omitempty"`
	Target  *Commit `json:"target,omitempty"`
}

// TagPage is one page of a tag listing. Bitbucket paginates with an absolute
// "next" URL; an empty Next marks the last page.
type TagPage struct {
	Values []Tag  `json:"values"`
	Next   string `json:"next,omitempty"`
}

// CommitSummary is a commit's rendered message body.
type CommitSummary struct {
	Raw string `json:"raw,omitempty"`
}

// CommitAuthor is the author of a commit: the raw "Name <email>" string and,
// when Bitbucket can map it, the linked account.
type CommitAuthor struct {
	Raw  string   `json:"raw,omitempty"`
	User *Account `json:"user,omitempty"`
}

// Commit is the subset of a Bitbucket commit that human output renders. It is
// shared by the commit commands and by the Target of a branch or tag.
type Commit struct {
	Hash    string         `json:"hash,omitempty"`
	Date    string         `json:"date,omitempty"`
	Message string         `json:"message,omitempty"`
	Summary *CommitSummary `json:"summary,omitempty"`
	Author  *CommitAuthor  `json:"author,omitempty"`
}

// CommitPage is one page of a commit listing. Bitbucket paginates with an
// absolute "next" URL; an empty Next marks the last page.
type CommitPage struct {
	Values []Commit `json:"values"`
	Next   string   `json:"next,omitempty"`
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

// SourceEntry is one entry in a repository directory listing. Type is
// "commit_file" or "commit_directory"; Size is the byte size of a file.
type SourceEntry struct {
	Path string `json:"path"`
	Type string `json:"type"`
	Size int64  `json:"size,omitempty"`
}

// SourcePage is one page of a directory listing. Bitbucket paginates with an
// absolute "next" URL; an empty Next marks the last page.
type SourcePage struct {
	Values []SourceEntry `json:"values"`
	Next   string        `json:"next,omitempty"`
}

// CommentContent is a comment's body, reduced to its raw markup.
type CommentContent struct {
	Raw string `json:"raw,omitempty"`
}

// PullRequestComment is the subset of a pull-request comment that human output
// renders. Deleted comments carry a true Deleted flag and an empty content.
type PullRequestComment struct {
	ID        int             `json:"id"`
	Content   *CommentContent `json:"content,omitempty"`
	User      *Account        `json:"user,omitempty"`
	CreatedOn string          `json:"created_on,omitempty"`
	Deleted   bool            `json:"deleted,omitempty"`
}

// PullRequestCommentPage is one page of a pull-request comment listing.
type PullRequestCommentPage struct {
	Values []PullRequestComment `json:"values"`
	Next   string               `json:"next,omitempty"`
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

// DeploymentState is a deployment's lifecycle state.
type DeploymentState struct {
	Name string `json:"name,omitempty"`
	Type string `json:"type,omitempty"`
}

// DeploymentRelease is the release (pipeline run) a deployment shipped.
type DeploymentRelease struct {
	Name string `json:"name,omitempty"`
}

// Deployment is the subset of a Bitbucket deployment that human output renders.
type Deployment struct {
	UUID        string             `json:"uuid,omitempty"`
	State       *DeploymentState   `json:"state,omitempty"`
	Environment *Environment       `json:"environment,omitempty"`
	Release     *DeploymentRelease `json:"release,omitempty"`
}

// DeploymentPage is one page of a deployment listing. Bitbucket paginates with
// an absolute "next" URL; an empty Next marks the last page.
type DeploymentPage struct {
	Values []Deployment `json:"values"`
	Next   string       `json:"next,omitempty"`
}

// Environment is the subset of a Bitbucket deployment environment that human
// output renders.
type Environment struct {
	UUID string `json:"uuid,omitempty"`
	Name string `json:"name,omitempty"`
	Slug string `json:"slug,omitempty"`
	Type string `json:"type,omitempty"`
	Rank int    `json:"rank,omitempty"`
}

// EnvironmentPage is one page of an environment listing. Bitbucket paginates
// with an absolute "next" URL; an empty Next marks the last page.
type EnvironmentPage struct {
	Values []Environment `json:"values"`
	Next   string        `json:"next,omitempty"`
}

// Decode unmarshals a raw Bitbucket API body into a model value, wrapping a
// decode failure as a structured error.
func Decode[T any](raw json.RawMessage) (T, error) {
	return restutil.Decode[T](raw, productName)
}
