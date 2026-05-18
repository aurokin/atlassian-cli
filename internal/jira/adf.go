package jira

import (
	"encoding/json"
	"strings"
)

// adfNode is one node of an Atlassian Document Format tree. Only the fields
// needed to recover plain text are decoded.
type adfNode struct {
	Type    string    `json:"type"`
	Text    string    `json:"text"`
	Content []adfNode `json:"content"`
}

// TextOf extracts plain text from an Atlassian Document Format body,
// best-effort. A body that is already a JSON string is returned verbatim;
// block-level nodes are separated by newlines. An empty or unparseable body
// yields the empty string.
func TextOf(body json.RawMessage) string {
	if len(body) == 0 {
		return ""
	}
	var s string
	if json.Unmarshal(body, &s) == nil {
		return s
	}
	var root adfNode
	if json.Unmarshal(body, &root) != nil {
		return ""
	}
	return strings.TrimSpace(adfText(root))
}

// adfText walks an ADF node, concatenating descendant text and inserting a
// newline before each block-level child after the first.
func adfText(n adfNode) string {
	switch n.Type {
	case "text":
		return n.Text
	case "hardBreak":
		return "\n"
	}
	var b strings.Builder
	for i, child := range n.Content {
		if i > 0 && adfIsBlock(child.Type) {
			b.WriteByte('\n')
		}
		b.WriteString(adfText(child))
	}
	return b.String()
}

// adfIsBlock reports whether an ADF node type is block-level, and so should
// start on its own line in the recovered text.
func adfIsBlock(t string) bool {
	switch t {
	case "paragraph", "heading", "blockquote", "codeBlock", "rule",
		"bulletList", "orderedList", "listItem", "mediaSingle", "mediaGroup",
		"panel", "table":
		return true
	}
	return false
}
