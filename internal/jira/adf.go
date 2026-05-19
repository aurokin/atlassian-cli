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

// adfMaxDepth bounds the ADF walk. Real documents are shallow and the JSON
// decoder already caps nesting, so this only guards a pathological body from
// driving the recursion into a stack overflow.
const adfMaxDepth = 100

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
	return strings.TrimSpace(adfText(root, 0))
}

// adfText walks an ADF node, concatenating descendant text and inserting a
// newline before each block-level child after the first. Recursion past
// adfMaxDepth is abandoned so a pathologically nested body cannot exhaust the
// stack.
func adfText(n adfNode, depth int) string {
	switch n.Type {
	case "text":
		return n.Text
	case "hardBreak":
		return "\n"
	}
	if depth >= adfMaxDepth {
		return ""
	}
	var b strings.Builder
	for i, child := range n.Content {
		if i > 0 && adfIsBlock(child.Type) {
			b.WriteByte('\n')
		}
		b.WriteString(adfText(child, depth+1))
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

// adfDoc and its children are the minimal Atlassian Document Format shapes
// DocOf emits — just enough to carry plain text as an issue description or a
// comment body.
type adfDoc struct {
	Type    string         `json:"type"`
	Version int            `json:"version"`
	Content []adfParagraph `json:"content"`
}

type adfParagraph struct {
	Type    string    `json:"type"`
	Content []adfSpan `json:"content,omitempty"`
}

type adfSpan struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// DocOf wraps plain text in a minimal ADF document, suitable as an issue
// description or comment body. Each line becomes its own paragraph; a blank
// line becomes an empty paragraph, since an ADF text node cannot be empty.
func DocOf(text string) json.RawMessage {
	doc := adfDoc{Type: "doc", Version: 1}
	for _, line := range strings.Split(text, "\n") {
		para := adfParagraph{Type: "paragraph"}
		if line != "" {
			para.Content = []adfSpan{{Type: "text", Text: line}}
		}
		doc.Content = append(doc.Content, para)
	}
	b, _ := json.Marshal(doc)
	return json.RawMessage(b)
}
