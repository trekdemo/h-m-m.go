// Package navigator answers directional movement queries ("what's the
// node to the left/right/above/below this one") over a set of positioned,
// optionally tree-structured nodes, so callers can swap navigation
// strategies without changing how they're invoked.
package navigator

// Rect is an axis-aligned box in canvas coordinates.
type Rect struct {
	X, Y, Width, Height int
}

// Center returns the rectangle's center point, as floats so distance/angle
// math doesn't need to round on every step.
func (r Rect) Center() (float64, float64) {
	return float64(r.X) + float64(r.Width)/2, float64(r.Y) + float64(r.Height)/2
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
