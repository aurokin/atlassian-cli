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

// Project is the Bitbucket project a repository belongs to, reduced to the
// fields human output renders.
type Project struct {
	Key  string `json:"key,omitempty"`
	Name string `json:"name,omitempty"`
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

// Decode unmarshals a raw Bitbucket API body into a model value, wrapping a
// decode failure as a structured error.
func Decode[T any](raw json.RawMessage) (T, error) {
	return restutil.Decode[T](raw, productName)
}
