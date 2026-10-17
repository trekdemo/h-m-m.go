// Package navigator answers directional movement queries ("what's the
// node to the left/right/above/below this one") over a set of positioned,
// optionally tree-structured nodes, so callers can swap navigation
// strategies without changing how they're invoked.
package navigator

import (
	"iter"

	"hmm/geom"
)

// Node is what a navigator needs from the nodes it moves between: where
// its box is and its place in a tree. It's a constraint over the caller's
// own node type N rather than a plain interface, so Parent and Children
// return the caller's type directly: a node type can satisfy this and other
// packages' node constraints with one Children method, without copying its
// children into a slice of interfaces, and callers get their own type back
// without a type assertion.
//
// A root's Parent, and a "nothing in that direction" result, is N's zero
// value (nil for a pointer type).
type Node[N any] interface {
	comparable
	Bounds() geom.Rect
	Parent() N
	Children() []N
}

// Navigator answers directional movement queries from a node: which node
// should the selection move to on left/right/up/down. Different
// implementations can back this with tree structure, box geometry, or
// anything else, so callers (e.g. keyboard handling) don't need to change
// when the navigation strategy does.
type Navigator[N any] interface {
	LeftOf(n N) N
	RightOf(n N) N
	Above(n N) N
	Below(n N) N
}

// treeOf yields every node in the tree n belongs to, in preorder from the
// root, so navigators can find candidates from n alone instead of being
// handed a copy of the node set that goes stale after every edit.
func treeOf[N Node[N]](n N) iter.Seq[N] {
	var none N
	for n.Parent() != none {
		n = n.Parent()
	}
	return func(yield func(N) bool) { walk(n, yield) }
}

func walk[N Node[N]](n N, yield func(N) bool) bool {
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
