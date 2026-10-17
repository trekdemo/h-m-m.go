package main

import (
	"fmt"
	"image/color"
	"iter"
	"slices"

	lipgloss "charm.land/lipgloss/v2"

	"hmm/geom"
	"hmm/navigator"
	"hmm/storage"
)

const (
	colGap = 6 // horizontal gap between a parent and its children (room for connector lines)
	rowGap = 1 // minimum vertical gap kept between sibling subtrees
)

// node is a treeSpec after box rendering and layout: it carries its
// rendered box plus the tree structure needed to lay it out and draw
// connector lines to its children.
type node struct {
	textBox
	color    color.Color
	parent   *node
	children []*node
	side     side // which side of the root this grows on; only set on the root's children

	// collapsed hides the node's descendants from layout, drawing and
	// navigation.
	// It's view state, so it isn't saved, and it's kept on hidden nodes so
	// a nested collapse survives its ancestor being collapsed and expanded.
	collapsed bool
}

// visibleChildren is what everything on screen sees of n's children:
// none while n is collapsed. Edits and saving use children directly,
// because hidden nodes are still part of the map.
func (n *node) visibleChildren() []*node {
	if n.collapsed {
		return nil
	}
	return n.children
}

// canCollapse is false for leaves, which have nothing to hide, and for the
// root, which would hide the whole map.
func (n *node) canCollapse() bool {
	return n.parent != nil && len(n.children) > 0
}

// marker is the stub and count drawn beside a collapsed node, so it
// doesn't pass for a leaf: "─ +5" on the side its children would grow on,
// mirrored to "+5 ─" on the left. The count includes every hidden node,
// not just direct children, to show how much is tucked away. Expanded
// nodes have no marker.
func (n *node) marker() string {
	if !n.collapsed {
		return ""
	}
	count := fmt.Sprintf("+%d", n.descendantCount())
	if n.growsLeft() {
		return count + " ─"
	}
	return "─ " + count
}

// markerBounds is where marker is drawn: level with the box's middle, the
// same row a connector to its children would leave from, and touching the
// box on the side the children grow on.
func (n *node) markerBounds() geom.Rect {
	w := lipgloss.Width(n.marker())
	x := n.x + n.Width()
	if n.growsLeft() {
		x = n.x - w
	}
	return geom.Rect{X: x, Y: n.y + n.Height()/2, Width: w, Height: 1}
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
	for _, c := range n.visibleChildren() {
		if !c.walk(yield) {
			return false
		}
	}
	return true
}

func (n *node) Parent() navigator.NavNode {
	if n.parent == nil {
		return nil
	}
	return n.parent
}

// Children only reports visible children, so navigators treat a collapsed
// node as a leaf and never select a hidden node.
func (n *node) Children() []navigator.NavNode {
	visible := n.visibleChildren()
	if len(visible) == 0 {
		return nil
	}
	out := make([]navigator.NavNode, len(visible))
	for i, c := range visible {
		out[i] = c
	}
	return out
}

// OutlineText, OutlineColor, and OutlineChildren implement storage.Outline,
// so a node is saved to OPML directly instead of being converted to an
// intermediate type first. Like Children, OutlineChildren copies into a new
// slice because Go can't use a []*node as a slice of interfaces. Unlike
// Children, it includes hidden children, since collapsing must never drop
// nodes from the file.
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
// are left zero for the caller and layoutTree to fill in.
func newNode(text string, c color.Color) *node {
	n := &node{
		color:   c,
		textBox: textBox{style: nodeStyle(c)},
	}
	n.setText(text)
	return n
}

// childColor gives every top-level branch its own color so branches are
// easy to tell apart, while descendants inherit theirs so a branch reads as
// one unit. A child of the root is colored by how many siblings it already
// has, which steps through the palette as branches are added.
func childColor(parent *node) color.Color {
	if parent.parent == nil {
		return branchColor(len(parent.children))
	}
	return parent.color
}

// buildTree turns a loaded spec into the mutable node tree the app edits
// and lays out. Only the root is special-cased, falling back to rootColor
// when the spec has no color; buildSubtree handles everything below it.
func buildTree(spec storage.Node) *node {
	return buildSubtree(spec, newNode(spec.Text, colorOr(spec.Color, rootColor)))
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
// side splitSides finds less crowded at the next layout.
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

// layoutTree grows the map in both directions from the root, like
// Mermaid's tidy-tree mindmap layout, so it stays compact instead of
// sprawling to one side. It pins the root at the origin, so editing never
// makes it jump, splits the root's branches between the sides (see
// splitSides), and lays out each side as its own tidy tree (see
// layoutSide). Callers re-run it over the whole tree after every edit,
// because one box changing size or a node being added can shift every
// other box; a full pass is cheap at mindmap sizes and far simpler than
// patching positions.
func layoutTree(root *node) {
	root.x, root.y = 0, 0
	right, left := splitSides(root.children)
	layoutSide(root, right, sideRight)
	layoutSide(root, left, sideLeft)
}

// side is the direction a half of the tree grows away from the root in.
// The zero value means a root child hasn't been given a side yet.
type side int

const (
	sideRight side = 1
	sideLeft  side = -1
)

// splitSides divides the root's children between the two sides, keeping
// each child on the side it already has so adding, removing or reordering
// siblings never moves a branch across while the app runs. Sides aren't
// saved, so a child without one yet (every branch of a freshly loaded
// tree, or a newly added one) goes to whichever side currently has fewer
// branches, the right one on a tie: a loaded tree alternates right, left,
// right... like Mermaid's, and a root with a single child still reads
// left-to-right.
func splitSides(children []*node) (right, left []*node) {
	nRight, nLeft := 0, 0
	for _, c := range children {
		switch c.side {
		case sideRight:
			nRight++
		case sideLeft:
			nLeft++
		}
	}
	for _, c := range children {
		if c.side == 0 {
			if nRight <= nLeft {
				c.side = sideRight
				nRight++
			} else {
				c.side = sideLeft
				nLeft++
			}
		}
		if c.side == sideRight {
			right = append(right, c)
		} else {
			left = append(left, c)
		}
	}
	return right, left
}

// layoutSide lays out one side's branches as a non-layered tidy tree (see
// tidy.go), so each child sits its parent's reach past the parent's far
// edge rather than in a column shared with every node at its depth, and
// sibling subtrees pack as closely as rowGap allows. The tidy layout grows
// downward, so it's fed boxes with their axes swapped (a box's height as
// tidy width, its width as tidy depth) and the result is rotated back. The
// branches hang under a virtual root the size of the real one, so the real
// root lands centered on its first level. The left side mirrors the right
// around the root, so a parent's children always line up on the edge
// facing it and their connectors meet at a shared trunk.
func layoutSide(root *node, children []*node, dir side) {
	if len(children) == 0 {
		return
	}

	var convert func(n *node, y float64) *tidyTree
	convert = func(n *node, y float64) *tidyTree {
		t := &tidyTree{
			n: n,
			y: y,
			w: float64(n.Height() + rowGap),
			h: float64(n.Width() + reach(n)),
		}
		for _, c := range n.visibleChildren() {
			t.c = append(t.c, convert(c, y+t.h))
		}
		return t
	}
	virtual := &tidyTree{
		w: float64(root.Height() + rowGap),
		h: float64(root.Width() + colGap),
	}
	for _, c := range children {
		virtual.c = append(virtual.c, convert(c, virtual.h))
	}

	tidyLayout(virtual)

	// Shift breadth so the virtual root lands exactly on the real one,
	// which centers the root on this side's first level.
	shift := float64(root.y) - virtual.x
	var place func(t *tidyTree)
	place = func(t *tidyTree) {
		n := t.n
		n.y = floorInt(t.x + shift)
		depth := int(t.y)
		if dir == sideRight {
			n.x = root.x + depth
		} else {
			n.x = root.x + root.Width() - depth - n.Width()
		}
		for _, c := range t.c {
			place(c)
		}
	}
	for _, c := range virtual.c {
		place(c)
	}
}

// reach is how far past its box a node claims room in the layout. It's
// usually colGap, the room its connectors need. A collapsed node's marker
// sits in that room too, so a marker wider than colGap claims its own
// width plus a cell of space instead, keeping it clear of neighbouring
// subtrees that the tidy layout would otherwise pack right up against it.
func reach(n *node) int {
	return max(colGap, lipgloss.Width(n.marker())+1)
}

// connector joins the facing middles of a parent and child box. Because
// edgeCells assumes edges run left to right, a child on the left side
// starts the edge at the child, mirroring the right side's edge cell for
// cell (including which end's first cell is hidden under a box border).
func connector(p, c *node) edge {
	if c.x < p.x {
		return edge{
			x1: c.x + c.Width() - 1,
			y1: c.y + c.Height()/2,
			x2: p.x - 1,
			y2: p.y + p.Height()/2,
		}
	}
	return edge{
		x1: p.x + p.Width(),
		y1: p.y + p.Height()/2,
		x2: c.x,
		y2: c.y + c.Height()/2,
	}
}

// buildScene is the one-call setup for a freshly loaded spec: build the
// tree and lay it out, so it's ready to draw.
func buildScene(spec storage.Node) *node {
	root := buildTree(spec)
	layoutTree(root)
	return root
}

// point is a canvas coordinate, used as a map key to accumulate the
// directions of every edge that touches a given cell.
type point struct{ x, y int }

// direction is a bitmask of the cardinal directions a line passes through
// a cell in. Multiple edges can contribute directions to the same cell
// (e.g. siblings branching off a shared trunk column), so cells are
// accumulated by OR-ing directions together before being converted to a
// single box-drawing rune.
type direction uint8

const (
	dirNorth direction = 1 << iota
	dirSouth
	dirEast
	dirWest
)

// edge is a connector line between a parent box and a child box, in canvas
// coordinates, always running left to right (x1 < x2).
type edge struct {
	x1, y1, x2, y2 int
}

// edgeCell is a single character position belonging to a drawn edge, and
// the direction(s) the line runs through it. Accumulating these (via OR)
// per position, across all edges, is what lets branching/merging lines
// render as proper junction characters instead of one edge's stroke
// silently overwriting another's.
type edgeCell struct {
	x, y int
	dir  direction
}

// edgeCells draws connectors as elbows that jog vertically halfway
// between the two boxes. Siblings share a parent edge and a column, so
// their jogs land on the same x and read as branches off one vertical
// trunk.
//
// Each cell only records the directions this one edge passes through it,
// not a rune, because the right character depends on every edge touching
// that cell; the caller ORs them together and resolves the rune with
// dirToRune.
func edgeCells(e edge) []edgeCell {
	if e.y1 == e.y2 {
		var cells []edgeCell
		for x := e.x1; x <= e.x2; x++ {
			cells = append(cells, edgeCell{x, e.y1, dirEast | dirWest})
		}
		return cells
	}

	midX := (e.x1 + e.x2) / 2
	var cells []edgeCell
	for x := e.x1; x < midX; x++ {
		cells = append(cells, edgeCell{x, e.y1, dirEast | dirWest})
	}
	for x := midX + 1; x <= e.x2; x++ {
		cells = append(cells, edgeCell{x, e.y2, dirEast | dirWest})
	}

	dy := 1
	corner1, corner2 := dirWest|dirSouth, dirNorth|dirEast // going down
	if e.y2 < e.y1 {
		dy = -1
		corner1, corner2 = dirWest|dirNorth, dirSouth|dirEast // going up
	}
	cells = append(cells, edgeCell{midX, e.y1, corner1})
	for y := e.y1 + dy; y != e.y2; y += dy {
		cells = append(cells, edgeCell{midX, y, dirNorth | dirSouth})
	}
	cells = append(cells, edgeCell{midX, e.y2, corner2})

	return cells
}

// dirToRune uses rounded elbows to match the boxes' rounded borders, but
// has to fall back to square junctions where three or four lines meet,
// since Unicode has no rounded T or cross. A lone direction (a stub end)
// still draws as a straight line rather than disappearing.
func dirToRune(d direction) rune {
	switch d {
	case dirNorth, dirSouth, dirNorth | dirSouth:
		return '│'
	case dirEast, dirWest, dirEast | dirWest:
		return '─'
	case dirNorth | dirEast:
		return '╰'
	case dirNorth | dirWest:
		return '╯'
	case dirSouth | dirEast:
		return '╭'
	case dirSouth | dirWest:
		return '╮'
	case dirNorth | dirSouth | dirEast:
		return '├'
	case dirNorth | dirSouth | dirWest:
		return '┤'
	case dirNorth | dirEast | dirWest:
		return '┴'
	case dirSouth | dirEast | dirWest:
		return '┬'
	case dirNorth | dirSouth | dirEast | dirWest:
		return '┼'
	default:
		return ' '
	}
}
