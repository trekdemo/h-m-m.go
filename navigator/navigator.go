// Package navigator answers directional movement queries ("what's the
// node to the left/right/above/below this one") over a set of positioned,
// optionally tree-structured nodes, so callers can swap navigation
// strategies without changing how they're invoked.
package navigator

import "iter"

// Rect is an axis-aligned box in canvas coordinates.
type Rect struct {
	X, Y, Width, Height int
}

// Center returns the rectangle's center point, as floats so distance/angle
// math doesn't need to round on every step.
func (r Rect) Center() (float64, float64) {
	return float64(r.X) + float64(r.Width)/2, float64(r.Y) + float64(r.Height)/2
}

// Contains reports whether the cell (x, y) lies inside the rectangle.
func (r Rect) Contains(x, y int) bool {
	return x >= r.X && x < r.X+r.Width && y >= r.Y && y < r.Y+r.Height
}

// NavNode is anything that can be navigated between: it knows its own screen
// position and, for tree-based navigation, its place in a tree.
type NavNode interface {
	Bounds() Rect
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
