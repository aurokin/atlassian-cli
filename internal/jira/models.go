package jira

import (
	"encoding/json"

	"github.com/aurokin/atlassian-cli/internal/apperr"
)

// User is the subset of a Jira user that human output renders.
type User struct {
	AccountID    string `json:"accountId"`
	DisplayName  string `json:"displayName"`
	EmailAddress string `json:"emailAddress"`
}

// NamedField is a Jira sub-object reduced to its display name, the shape
// shared by status, issue type, and priority.
type NamedField struct {
	Name string `json:"name"`
}

// Project is the subset of a Jira project that human output renders.
type Project struct {
	ID             string `json:"id"`
	Key            string `json:"key"`
	Name           string `json:"name"`
	ProjectTypeKey string `json:"projectTypeKey"`
	Lead           *User  `json:"lead"`
}

// ProjectPage is a page of project-search results.
type ProjectPage struct {
	Values []Project `json:"values"`
	Total  int       `json:"total"`
	IsLast bool      `json:"isLast"`
}

// IssueFields is the subset of an issue's fields that human output renders.
type IssueFields struct {
	Summary   string      `json:"summary"`
	Status    *NamedField `json:"status"`
	IssueType *NamedField `json:"issuetype"`
	Priority  *NamedField `json:"priority"`
	Assignee  *User       `json:"assignee"`
	Reporter  *User       `json:"reporter"`
	Created   string      `json:"created"`
	Updated   string      `json:"updated"`
}

// Issue is the subset of a Jira issue that human output renders.
type Issue struct {
	ID     string      `json:"id"`
	Key    string      `json:"key"`
	Fields IssueFields `json:"fields"`
}

// IssuePage is a page of JQL search results. The Jira /search/jql endpoint
// paginates with a nextPageToken and reports completion via isLast.
type IssuePage struct {
	Issues        []Issue `json:"issues"`
	NextPageToken string  `json:"nextPageToken"`
	IsLast        bool    `json:"isLast"`
}

// Comment is the subset of a Jira comment that human output renders. Body is
// kept raw because Jira Cloud v3 stores it as an Atlassian Document Format
// object.
type Comment struct {
	ID      string          `json:"id"`
	Author  *User           `json:"author"`
	Created string          `json:"created"`
	Updated string          `json:"updated"`
	Body    json.RawMessage `json:"body"`
}

// CommentPage is a page of an issue's comments.
type CommentPage struct {
	Comments   []Comment `json:"comments"`
	Total      int       `json:"total"`
	StartAt    int       `json:"startAt"`
	MaxResults int       `json:"maxResults"`
}

// Decode unmarshals a raw Jira response body into a model value, wrapping a
// decode failure as a structured error.
func Decode[T any](raw json.RawMessage) (T, error) {
	var v T
	if err := json.Unmarshal(raw, &v); err != nil {
		return v, apperr.New("response_decode_failed",
			"could not decode the Jira API response: "+err.Error())
	}
	return v, nil
}
