// Package navigator answers directional movement queries ("what's the
// node to the left/right/above/below this one") over a set of positioned,
// optionally tree-structured nodes, so callers can swap navigation
// strategies without changing how they're invoked.
package navigator

import (
	"iter"

	"hmm/geom"
)

// NavNode is anything that can be navigated between: it knows its own screen
// position and, for tree-based navigation, its place in a tree.
type NavNode interface {
	Bounds() geom.Rect
	Parent() NavNode
	Children() []NavNode
}

// Navigator answers directional movement queries from a node: which node
// should the selection move to on left/right/up/down. Different
// implementations can back this with tree structure, box geometry, or
// anything else, so callers (e.g. keyboard handling) don't need to change
// when the navigation strategy does.
type Navigator interface {
	LeftOf(n NavNode) NavNode
	RightOf(n NavNode) NavNode
	Above(n NavNode) NavNode
	Below(n NavNode) NavNode
}

// treeOf yields every node in the tree n belongs to, in preorder from the
// root, so navigators can find candidates from n alone instead of being
// handed a copy of the node set that goes stale after every edit.
func treeOf(n NavNode) iter.Seq[NavNode] {
	for n.Parent() != nil {
		n = n.Parent()
	}
	return func(yield func(NavNode) bool) { walk(n, yield) }
}

func walk(n NavNode, yield func(NavNode) bool) bool {
	if !yield(n) {
		return false
	}
	for _, c := range n.Children() {
		if !walk(c, yield) {
			return false
		}
	}
	return true
}
