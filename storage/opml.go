// Package storage loads and saves the tree to disk. It currently only
// supports the OPML format.
package storage

import (
	"encoding/xml"
	"fmt"
	"os"
)

// Node is a lightweight, format-independent tree read from disk: a piece
// of text plus its children. It's what Load returns, decoupled from any
// particular file format's on-disk shape.
type Node struct {
	Text     string
	Children []Node
}

// Outline is the minimal shape a tree node must expose to be saved to
// disk. The main package's *node type implements it directly, so the live
// UI tree can be exported without first converting to an intermediate
// type.
type Outline interface {
	OutlineText() string
	OutlineChildren() []Outline
}

type opmlDocument struct {
	XMLName xml.Name `xml:"opml"`
	Version string   `xml:"version,attr"`
	Head    opmlHead `xml:"head"`
	Body    opmlBody `xml:"body"`
}

type opmlHead struct {
	Title string `xml:"title"`
}

type opmlBody struct {
	Outlines []opmlOutline `xml:"outline"`
}

type opmlOutline struct {
	Text     string        `xml:"text,attr"`
	Outlines []opmlOutline `xml:"outline"`
}

// LoadOPML reads the OPML file at path and returns its root outline as a
// Node tree. OPML allows a body to hold multiple top-level outlines, but
// this tool works with a single-rooted tree, so exactly one is required.
func LoadOPML(path string) (Node, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Node{}, fmt.Errorf("reading %s: %w", path, err)
	}

	var doc opmlDocument
	if err := xml.Unmarshal(data, &doc); err != nil {
		return Node{}, fmt.Errorf("parsing %s: %w", path, err)
	}

	if len(doc.Body.Outlines) != 1 {
		return Node{}, fmt.Errorf("%s: expected exactly one root outline, got %d", path, len(doc.Body.Outlines))
	}

	return outlineToNode(doc.Body.Outlines[0]), nil
}

func outlineToNode(o opmlOutline) Node {
	n := Node{Text: o.Text}
	for _, c := range o.Outlines {
		n.Children = append(n.Children, outlineToNode(c))
	}
	return n
}

// SaveOPML writes root, and its descendants, to path as an OPML 2.0
// document, overwriting any existing file.
func SaveOPML(path string, root Outline) error {
	doc := opmlDocument{
		Version: "2.0",
		Head:    opmlHead{Title: root.OutlineText()},
		Body:    opmlBody{Outlines: []opmlOutline{outlineToXML(root)}},
	}

	body, err := xml.MarshalIndent(doc, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding %s: %w", path, err)
	}

	out := append([]byte(xml.Header), body...)
	out = append(out, '\n')

	if err := os.WriteFile(path, out, 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}
	return nil
}

func outlineToXML(o Outline) opmlOutline {
	out := opmlOutline{Text: o.OutlineText()}
	for _, c := range o.OutlineChildren() {
		out.Outlines = append(out.Outlines, outlineToXML(c))
	}
	return out
}
