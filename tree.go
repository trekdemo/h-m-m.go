package main

import (
	"image/color"
	"iter"
	"slices"

	"hmm/geom"
	"hmm/layout"
	"hmm/navigator"
	"hmm/render"
	"hmm/storage"
)

// *node is the glue between the app's aspects: each aspect's package
// declares the slice of a node it needs (navigator.Node, layout.SidedNode,
// storage.Outline), and *node provides all of them. These declarations stop
// compiling if it ever stops doing so.
var (
	_ layout.Layout[*node]       = layout.Mindmap[*node]{}
	_ navigator.Navigator[*node] = navigator.Spatial[*node]{}
	_ navigator.Navigator[*node] = navigator.Tree[*node]{}
	_ storage.Outline            = (*node)(nil)
)

// node is one node of the map: its text and color, its place in the tree,
// and where the layout put its box. How the box looks is left to render;
// the node only keeps the size render measured for its text.
type node struct {
	text     string
	size     geom.Size // the box's size, kept in step with text by setText
	x, y     int       // the box's top-left corner, in canvas coordinates
	color    color.Color
	parent   *node
	children []*node
	side     layout.Side // which side of the root this grows on; only set on the root's children

	// collapsed hides the node's descendants from layout, drawing and
	// navigation.
	// It's view state, so it isn't saved, and it's kept on hidden nodes so
	// a nested collapse survives its ancestor being collapsed and expanded.
	collapsed bool
}

// canCollapse is false for leaves, which have nothing to hide, and for the
// root, which would hide the whole map.
func (n *node) canCollapse() bool {
	return n.parent != nil && len(n.children) > 0
}

// setText is the only way to change the text, so the cached size never
// goes stale. The size is cached because the layout asks for it often.
func (n *node) setText(text string) {
	n.text = text
	n.size = render.Size(text)
}

// Size and Bounds report the node's box, for the layout, the navigators
// and hit testing.
func (n *node) Size() geom.Size { return n.size }

func (n *node) Bounds() geom.Rect {
	return geom.Rect{X: n.x, Y: n.y, Width: n.size.Width, Height: n.size.Height}
}

// SetPosition, Side, SetSide and Overhang, with Size, Bounds and Children,
// make *node a layout.SidedNode.
func (n *node) SetPosition(p geom.Point) { n.x, n.y = p.X, p.Y }

func (n *node) Side() layout.Side     { return n.side }
func (n *node) SetSide(s layout.Side) { n.side = s }

// Overhang is the width of n's marker, which sits in the room past n's box
// where its children would be.
func (n *node) Overhang() int { return render.Width(n.marker()) }

// marker is drawn beside a collapsed node (see render.Marker), on the side
// its children would grow on. The count includes every hidden node, not
// just direct children, to show how much is tucked away. Expanded nodes
// have no marker.
func (n *node) marker() string {
	if !n.collapsed {
		return ""
	}
	return render.Marker(n.descendantCount(), n.growsLeft())
}

// markerBounds is where marker is drawn: level with the box's middle, the
// same row a connector to its children would leave from, and touching the
// box on the side the children grow on.
func (n *node) markerBounds() geom.Rect {
	w := n.Overhang()
	x := n.x + n.size.Width
	if n.growsLeft() {
		x = n.x - w
	}
	return geom.Rect{X: x, Y: n.y + n.size.Height/2, Width: w, Height: 1}
}

// growsLeft reports whether n's children are laid out to its left, which
// is the case for every node on the root's left side.
func (n *node) growsLeft() bool {
	return n.parent != nil && n.x < n.parent.x
}

func (n *node) descendantCount() int {
	count := 0
	for _, c := range n.children {
		count += 1 + c.descendantCount()
	}
	return count
}

// reveal expands every collapsed ancestor of n, and nothing else, so n
// becomes visible while the rest of the map keeps its collapsed state. It
// reports whether anything changed, so the caller can skip a needless
// relayout.
func reveal(n *node) bool {
	changed := false
	for p := n.parent; p != nil; p = p.parent {
		if p.collapsed {
			p.collapsed = false
			changed = true
		}
	}
	return changed
}

// All yields n and every visible node below it in preorder, so drawing,
// hit testing and centering read the tree itself rather than a flattened
// copy that would need re-syncing after every edit.
func (n *node) All() iter.Seq[*node] {
	return func(yield func(*node) bool) { n.walk(yield) }
}

func (n *node) walk(yield func(*node) bool) bool {
	if !yield(n) {
		return false
	}
	for _, c := range n.Children() {
		if !c.walk(yield) {
			return false
		}
	}
	return true
}

// Parent and Children make *node a navigator.Node.
func (n *node) Parent() *node { return n.parent }

// Children is what everything on screen sees of n's children: none while
// n is collapsed, so navigators and the layout treat a collapsed node as a
// leaf and never reach a hidden node. Edits and saving use the children
// field directly, because hidden nodes are still part of the map.
func (n *node) Children() []*node {
	if n.collapsed {
		return nil
	}
	return n.children
}

// OutlineText, OutlineColor, and OutlineChildren implement storage.Outline,
// so a node is saved to OPML directly instead of being converted to an
// intermediate type first. OutlineChildren copies into a new slice because
// Go can't use a []*node as a slice of interfaces. Unlike Children, it
// includes hidden children, since collapsing must never drop nodes from
// the file.
func (n *node) OutlineText() string { return n.text }

func (n *node) OutlineColor() string { return hexColor(n.color) }

func (n *node) OutlineChildren() []storage.Outline {
	if len(n.children) == 0 {
		return nil
	}
	out := make([]storage.Outline, len(n.children))
	for i, c := range n.children {
		out[i] = c
	}
	return out
}

// newNode is shared by the initial build and by edits that insert nodes
// later, so both produce identically styled boxes. Position and tree links
// are left zero for the caller and the layout to fill in.
func newNode(text string, c color.Color) *node {
	n := &node{color: c}
	n.setText(text)
	return n
}

// childColor gives every top-level branch its own color so branches are
// easy to tell apart, while descendants inherit theirs so a branch reads as
// one unit. A child of the root is colored by how many siblings it already
// has, which steps through the palette as branches are added.
func childColor(parent *node) color.Color {
	if parent.parent == nil {
		return render.BranchColor(len(parent.children))
	}
	return parent.color
}

// buildTree turns a loaded spec into the mutable node tree the app edits
// and lays out. Only the root is special-cased, falling back to rootColor
// when the spec has no color; buildSubtree handles everything below it.
func buildTree(spec storage.Node) *node {
	return buildSubtree(spec, newNode(spec.Text, colorOr(spec.Color, render.RootColor)))
}

// buildSubtree recurses depth-first, preferring each spec node's own color
// and falling back to childColor so files saved without colors still get
// colored branches.
func buildSubtree(spec storage.Node, n *node) *node {
	for _, childSpec := range spec.Children {
		child := newNode(childSpec.Text, colorOr(childSpec.Color, childColor(n)))
		child.parent = n // set before recursing: childColor reads the parent chain
		n.children = append(n.children, buildSubtree(childSpec, child))
	}
	return n
}

// addSiblingAfter inserts right after n so the new node appears next to
// where the user is looking. It copies n's side so a new root branch stays
// on the same half of the map as n. The root has no siblings, so it falls
// back to adding a child rather than doing nothing. The text starts empty
// for the user to type into.
func addSiblingAfter(n *node) *node {
	if n.parent == nil {
		return addChild(n)
	}
	sib := newNode("", childColor(n.parent))
	sib.parent = n.parent
	sib.side = n.side
	i := slices.Index(n.parent.children, n)
	n.parent.children = slices.Insert(n.parent.children, i+1, sib)
	return sib
}

// addChild appends at the end, matching how outliners add children. The
// child's side is left unset, so a new root branch is placed on whichever
// side the layout finds less crowded next time it runs.
func addChild(n *node) *node {
	child := newNode("", childColor(n))
	child.parent = n
	n.children = append(n.children, child)
	return child
}

// prevSibling lets deletion move the selection to the node just above the
// removed one, keeping the user near where they were. It returns nil for
// the root and for a first child, leaving the caller to fall back to the
// parent.
func prevSibling(n *node) *node {
	if n.parent == nil {
		return nil
	}
	i := slices.Index(n.parent.children, n)
	if i <= 0 {
		return nil
	}
	return n.parent.children[i-1]
}

// moveSibling reorders by swapping n with the nearest sibling in direction
// delta that grows on the same side of the root, so the node moves up or
// down on screen without ever jumping across the root. Below the root's
// children every sibling shares n's side, so that's simply the adjacent
// one. It reports whether anything moved so the caller can skip a
// needless relayout.
func moveSibling(n *node, delta int) bool {
	if n.parent == nil {
		return false
	}
	siblings := n.parent.children
	i := slices.Index(siblings, n)
	j := i + delta
	for j >= 0 && j < len(siblings) && siblings[j].side != n.side {
		j += delta
	}
	if j < 0 || j >= len(siblings) {
		return false
	}
	siblings[i], siblings[j] = siblings[j], siblings[i]
	return true
}

// removeNode only needs to drop n from its parent's children: the whole
// subtree is reachable solely through n, so it goes with it. The root
// can't be removed, so it's refused with a nil return.
func removeNode(n *node) *node {
	if n.parent == nil {
		return nil
	}
	parent := n.parent
	parent.children = slices.DeleteFunc(parent.children, func(c *node) bool { return c == n })
	return parent
}
