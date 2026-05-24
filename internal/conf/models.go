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

// PageBody holds the body representations a v2 page GET returns. The v2
// body-format query param takes a single value, so only the representation
// named by the request is populated: a storage GET fills Storage, an
// atlas_doc_format GET fills AtlasDocFormat. Pages authored in the modern
// editor have no storage body, so a title-only edit must fall back to the
// atlas_doc_format representation to preserve the body.
type PageBody struct {
	Storage        PageRepresentation `json:"storage"`
	AtlasDocFormat PageRepresentation `json:"atlas_doc_format"`
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

// Ancestor is one entry in a page's ancestor chain (its breadcrumb). The v2
// ancestors endpoint returns minimal objects — only an id and a content type —
// so richer detail (e.g. titles) needs a follow-up GET on each ancestor.
type Ancestor struct {
	ID   string `json:"id"`
	Type string `json:"type"`
}

// AncestorList is a page of ancestor results.
type AncestorList struct {
	Results []Ancestor `json:"results"`
}

// Version is one entry in a page's version history. It is distinct from
// PageVersion (the bare {number} stamp an edit echoes back); a history entry
// also carries who changed it, when, and the optional change message.
type Version struct {
	Number    int    `json:"number"`
	Message   string `json:"message,omitempty"`
	AuthorID  string `json:"authorId,omitempty"`
	CreatedAt string `json:"createdAt,omitempty"`
	MinorEdit bool   `json:"minorEdit,omitempty"`
}

// VersionList is a page of version-history results.
type VersionList struct {
	Results []Version `json:"results"`
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
