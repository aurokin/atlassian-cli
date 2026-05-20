package conf

import (
	"encoding/json"

	"github.com/aurokin/atlassian-cli/internal/restutil"
)

// User is the subset of a Confluence user that human output renders.
type User struct {
	AccountID   string `json:"accountId"`
	DisplayName string `json:"displayName"`
	Email       string `json:"email"`
}

// Space is the subset of a Confluence space that human output renders.
type Space struct {
	ID   string `json:"id"`
	Key  string `json:"key"`
	Name string `json:"name"`
	Type string `json:"type"`
}

// SpacePage is a page of space-list results.
type SpacePage struct {
	Results []Space `json:"results"`
}

// PageVersion is the version stamp of a Confluence page. The number is
// required to edit a page: an update must carry the current number plus one.
type PageVersion struct {
	Number int `json:"number"`
}

// Page is the subset of a Confluence page that human output renders, plus the
// fields a versioned edit needs to reconstruct the page.
type Page struct {
	ID      string      `json:"id"`
	Title   string      `json:"title"`
	Status  string      `json:"status"`
	SpaceID string      `json:"spaceId"`
	Version PageVersion `json:"version"`
	Body    PageBody    `json:"body"`
}

// PageBody holds the body representations a v2 page GET returns. Only the
// representation named by the request's body-format parameter is populated;
// the CLI requests storage, so Storage carries the current body.
type PageBody struct {
	Storage PageRepresentation `json:"storage"`
}

// PageRepresentation is a page body expressed in one representation.
type PageRepresentation struct {
	Representation string `json:"representation"`
	Value          string `json:"value"`
}

// PageList is a page of page-list results.
type PageList struct {
	Results []Page `json:"results"`
}

// Comment is the subset of a Confluence footer comment that human output
// renders, plus the fields a versioned edit needs to reconstruct it.
type Comment struct {
	ID      string      `json:"id"`
	Status  string      `json:"status"`
	Title   string      `json:"title"`
	PageID  string      `json:"pageId"`
	Version PageVersion `json:"version"`
	Body    PageBody    `json:"body"`
}

// CommentList is a page of footer-comment results.
type CommentList struct {
	Results []Comment `json:"results"`
}

// Label is a single Confluence content label. Only the fields human output
// renders are modeled; the label id is intentionally omitted so the decoder
// is agnostic to whether the API serializes it as a string or a number, and
// because labels are addressed by name throughout the command surface. The
// raw id is still available under --json, which passes the body through
// verbatim.
type Label struct {
	Name   string `json:"name"`
	Prefix string `json:"prefix"`
}

// LabelList is a page of label results.
type LabelList struct {
	Results []Label `json:"results"`
}

// Attachment is the subset of a Confluence attachment that human output
// renders. DownloadLink locates the binary; it is rooted at the Confluence
// context path, not the v2 API base.
type Attachment struct {
	ID           string `json:"id"`
	Title        string `json:"title"`
	MediaType    string `json:"mediaType"`
	FileSize     int64  `json:"fileSize"`
	Status       string `json:"status"`
	DownloadLink string `json:"downloadLink"`
}

// AttachmentList is a page of attachment results.
type AttachmentList struct {
	Results []Attachment `json:"results"`
}

// SearchContent is the content object carried by a CQL search hit.
type SearchContent struct {
	ID    string `json:"id"`
	Type  string `json:"type"`
	Title string `json:"title"`
}

// SearchResult is one CQL search hit.
type SearchResult struct {
	Content SearchContent `json:"content"`
}

// SearchResults is a CQL search response.
type SearchResults struct {
	Results []SearchResult `json:"results"`
}

// Decode unmarshals a raw Confluence response body into a model value,
// wrapping a decode failure as a structured error.
func Decode[T any](raw json.RawMessage) (T, error) {
	return restutil.Decode[T](raw, productName)
}
