// Package layout decides where every box of a tree goes on the canvas,
// and how the connector between a parent and child runs, so the app can
// switch between layout algorithms without changing how it draws or
// navigates.
package layout

import "hmm/geom"

// Node is what a layout needs from the nodes it arranges: how big each
// box is, which children it shows, and a way to put it somewhere. Like
// navigator.Node, it's a constraint over the caller's own node type N, so
// one node type can serve every aspect of the app with a single Children
// method.
type Node[N any] interface {
	comparable
	Size() geom.Size
	Bounds() geom.Rect
	Children() []N // only the children on screen; a collapsed node has none
	SetPosition(geom.Point)

	// Overhang is how far something drawn beside the box, such as a
	// collapsed node's marker, sticks out on the side its children grow
	// on, so the layout keeps that room clear. It's 0 when nothing does.
	Overhang() int
}

// Layout is a layout algorithm. Arrange positions every node it can see
// from root, and is re-run over the whole tree after every edit: one box
// changing size, or a node being added, can shift every other box, and a
// full pass is cheap at mind map sizes and far simpler than patching
// positions. Route gives the connector from a parent to one of its
// children, as a polyline of horizontal and vertical runs in canvas
// coordinates, since only the layout knows which way the tree grows.
type Layout[N any] interface {
	Arrange(root N)
	Route(parent, child N) []geom.Point
}
