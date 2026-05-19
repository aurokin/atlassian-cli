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

// ProjectPage is a page of project-search results. Only the values are
// modelled; pagination cursors are deferred until --all support lands.
type ProjectPage struct {
	Values []Project `json:"values"`
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

// IssuePage is a page of JQL search results. Only the issues are modelled;
// the /search/jql nextPageToken cursor is deferred until --all support lands.
type IssuePage struct {
	Issues []Issue `json:"issues"`
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

// CommentPage is a page of an issue's comments. Only the comments are
// modelled; pagination cursors are deferred until --all support lands.
type CommentPage struct {
	Comments []Comment `json:"comments"`
}

// Transition is one workflow transition available on an issue.
type Transition struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// TransitionList is the set of transitions available on an issue from its
// current status.
type TransitionList struct {
	Transitions []Transition `json:"transitions"`
}

// Watchers is the watcher list returned by GET /issue/{idOrKey}/watchers.
type Watchers struct {
	IsWatching bool   `json:"isWatching"`
	WatchCount int    `json:"watchCount"`
	Watchers   []User `json:"watchers"`
}

// LinkType is one entry from GET /issueLinkType. Inward and Outward are the
// human phrases that describe each direction of the relationship (e.g.
// "is blocked by" / "blocks").
type LinkType struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Inward  string `json:"inward"`
	Outward string `json:"outward"`
}

// LinkTypeList is the body of GET /issueLinkType.
type LinkTypeList struct {
	Types []LinkType `json:"issueLinkTypes"`
}

// Worklog is the subset of a Jira worklog entry that human output renders.
// Comment is kept raw because Jira Cloud v3 stores it as an ADF object.
type Worklog struct {
	ID               string          `json:"id"`
	Author           *User           `json:"author"`
	Created          string          `json:"created"`
	Updated          string          `json:"updated"`
	Started          string          `json:"started"`
	TimeSpent        string          `json:"timeSpent"`
	TimeSpentSeconds int             `json:"timeSpentSeconds"`
	Comment          json.RawMessage `json:"comment"`
}

// WorklogPage is a page of an issue's worklogs.
type WorklogPage struct {
	Worklogs []Worklog `json:"worklogs"`
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
