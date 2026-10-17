package main

import (
	"hmm/layout"
	"hmm/storage"
)

// mindmap is the layout newModel uses, for tests that lay out a tree
// without a model.
var mindmap = layout.Mindmap[*node]{}

func arrange(root *node) { mindmap.Arrange(root) }

// buildScene builds a freshly loaded spec's tree and lays it out, so it's
// ready to draw.
func buildScene(spec storage.Node) *node {
	root := buildTree(spec)
	arrange(root)
	return root
}
