package main

import (
	"image/color"

	"hmm/layout"
	"hmm/navigator"
	"hmm/storage"

	lipgloss "charm.land/lipgloss/v2"
)

const (
	boxTextWidth = 30 // max characters per line inside a box
	colGap       = 6  // horizontal gap between a parent and its children (room for connector lines)
	rowGap       = 1  // minimum vertical gap kept between sibling subtrees
)

// spacing is the room the layout keeps around boxes.
var spacing = layout.Spacing{Depth: colGap, Breadth: rowGap}

// box is a piece of text rendered inside a rounded border, positioned
// somewhere on the virtual canvas.
type box struct {
	x, y          int // top-left position in canvas coordinates
	width, height int // rendered size, including the border
	text          string
	style         lipgloss.Style // normal border
	selStyle      lipgloss.Style // double border, drawn for the selected node
}

// node is a treeSpec after box rendering and layout: it carries its
// rendered box plus the tree structure needed to lay it out and draw
// connector lines to its children.
type node struct {
	color    color.Color
	box      box
	parent   *node
	children []*node
	idx      int         // index into the flattened boxes/nodes slices
	side     layout.Side // which side of the root this grows on; only set on the root's children
}

// LayoutSize, LayoutChildren, SetLayoutPosition, LayoutSide and
// SetLayoutSide implement layout.Node: the layout only sees each box's
// rendered size, and hands back where the box goes.
func (n *node) LayoutSize() (int, int) { return n.box.width, n.box.height }

func (n *node) LayoutChildren() []layout.Node {
	if len(n.children) == 0 {
		return nil
	}
	out := make([]layout.Node, len(n.children))
	for i, c := range n.children {
		out[i] = c
	}
	return out
}

func (n *node) SetLayoutPosition(x, y int) { n.box.x, n.box.y = x, y }

func (n *node) LayoutSide() layout.Side { return n.side }

func (n *node) SetLayoutSide(s layout.Side) { n.side = s }

// Bounds, Parent, and Children implement navigator.Node, letting node be
// navigated over without the navigator package knowing about tree.go's
// layout or rendering details.
func (n *node) Bounds() navigator.Rect {
	return navigator.Rect{X: n.box.x, Y: n.box.y, Width: n.box.width, Height: n.box.height}
}

func (n *node) Parent() navigator.NavNode {
	if n.parent == nil {
		return nil
	}
	return n.parent
}

func (n *node) Children() []navigator.NavNode {
	if len(n.children) == 0 {
		return nil
	}
	out := make([]navigator.NavNode, len(n.children))
	for i, c := range n.children {
		out[i] = c
	}
	return out
}

// OutlineText and OutlineChildren implement storage.Outline, letting a node
// be saved to OPML directly, without converting to an intermediate type.
func (n *node) OutlineText() string { return n.box.text }

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

// navNodes converts a flat node slice into navigator.Node, for building a
// navigator over the whole tree's nodes.
func navNodes(nodes []*node) []navigator.NavNode {
	out := make([]navigator.NavNode, len(nodes))
	for i, n := range nodes {
		out[i] = n
	}
	return out
}

// edge is a connector line between a parent box and a child box, in canvas
// coordinates, always running left to right (x1 < x2).
type edge struct {
	x1, y1, x2, y2 int
}

// newNode builds a single node's box (wrapping text to boxTextWidth, drawn
// in the given color) without attaching it to a tree, for reuse by both
// buildTree and callers that insert a node after the initial build (e.g.
// adding a sibling).
func newNode(text string, c color.Color) *node {
	style := nodeStyle(c)
	selStyle := selectedStyle(style)

	rendered := style.Render(text)

	return &node{
		color: c,
		box: box{
			width:    lipgloss.Width(rendered),
			height:   lipgloss.Height(rendered),
			text:     text,
			style:    style,
			selStyle: selStyle,
		},
	}
}

// childColor picks the color for a new child of parent: each child of the
// root gets the next palette color, deeper nodes inherit their parent's.
func childColor(parent *node) color.Color {
	if parent.parent == nil {
		return levelColor(len(parent.children))
	}
	return parent.color
}

// buildTree renders each spec node into a box (wrapping text to
// boxTextWidth) and returns the corresponding node tree. The root uses
// rootColor and colors from the spec take precedence; see childColor for how
// nodes without one are colored.
func buildTree(spec storage.Node) *node {
	return buildSubtree(spec, newNode(spec.Text, colorOr(spec.Color, rootColor)))
}

func buildSubtree(spec storage.Node, n *node) *node {
	for _, childSpec := range spec.Children {
		child := newNode(childSpec.Text, colorOr(childSpec.Color, childColor(n)))
		child.parent = n // set before recursing: childColor reads the parent chain
		n.children = append(n.children, buildSubtree(childSpec, child))
	}
	return n
}

// addSiblingAfter inserts a new, empty-text node into n's parent's children
// right after n, and returns it. It returns nil if n has no parent (the
// root has no siblings).
func addSiblingAfter(n *node) *node {
	if n.parent == nil {
		return nil
	}
	sib := newNode("", childColor(n.parent))
	sib.parent = n.parent
	sib.side = n.side
	siblings := n.parent.children
	i := 0
	for ; i < len(siblings); i++ {
		if siblings[i] == n {
			break
		}
	}
	siblings = append(siblings, nil)
	copy(siblings[i+2:], siblings[i+1:])
	siblings[i+1] = sib
	n.parent.children = siblings
	return sib
}

// addChild appends a new, empty-text node as n's last child and returns it.
func addChild(n *node) *node {
	child := newNode("", childColor(n))
	child.parent = n
	n.children = append(n.children, child)
	return child
}

// prevSibling returns the node immediately before n in its parent's
// children, or nil if n has no parent or is already its first child.
func prevSibling(n *node) *node {
	if n.parent == nil {
		return nil
	}
	siblings := n.parent.children
	for i, c := range siblings {
		if c == n {
			if i == 0 {
				return nil
			}
			return siblings[i-1]
		}
	}
	return nil
}

// moveSibling swaps n with its nearest sibling in direction delta (-1
// earlier, +1 later) that grows on the same side of the root, leaving it
// selected in its new position. Below the root's children every sibling
// shares n's side, so that's simply the adjacent one; among the root's
// children it skips over the other side's, so the node moves up or down
// without ever jumping across. It's a no-op (returning false) if n has no
// parent or there's no such sibling.
func moveSibling(n *node, delta int) bool {
	if n.parent == nil {
		return false
	}
	siblings := n.parent.children
	i := 0
	for ; i < len(siblings); i++ {
		if siblings[i] == n {
			break
		}
	}
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

// removeNode detaches n (and, since they're only reachable through n, its
// whole subtree) from its parent's children and returns the parent. It
// returns nil without modifying the tree if n has no parent, since the
// root can't be removed.
func removeNode(n *node) *node {
	if n.parent == nil {
		return nil
	}
	parent := n.parent
	siblings := parent.children
	for i, c := range siblings {
		if c == n {
			parent.children = append(siblings[:i], siblings[i+1:]...)
			break
		}
	}
	return parent
}

// layoutTree positions every node of the tree as a bidirectional mindmap
// around the root (see layout.Mindmap).
func layoutTree(root *node) { layout.Mindmap(root, spacing) }

// flattenTree walks the laid-out tree, collecting every box plus a
// connector edge between each parent and child (see connector). It also assigns each node its flat index and
// collects the nodes themselves, in the same order as boxes, so callers
// can navigate the tree structure (parent/children/siblings) by index.
func flattenTree(n *node, boxes *[]box, edges *[]edge, nodes *[]*node) {
	n.idx = len(*boxes)
	*boxes = append(*boxes, n.box)
	*nodes = append(*nodes, n)
	for _, c := range n.children {
		*edges = append(*edges, connector(n, c))
		flattenTree(c, boxes, edges, nodes)
	}
}

// connector returns the edge between parent p and child c. Edges always
// run left to right, so on the left side of the tree it goes from the
// child's right-middle edge to the parent's left-middle edge, mirroring
// the right side's parent-to-child edge cell for cell (including which
// end's first cell is hidden under a box border).
func connector(p, c *node) edge {
	if c.box.x < p.box.x {
		return edge{
			x1: c.box.x + c.box.width - 1,
			y1: c.box.y + c.box.height/2,
			x2: p.box.x - 1,
			y2: p.box.y + p.box.height/2,
		}
	}
	return edge{
		x1: p.box.x + p.box.width,
		y1: p.box.y + p.box.height/2,
		x2: c.box.x,
		y2: c.box.y + c.box.height/2,
	}
}

// buildScene builds and lays out a tree from spec, returning the root node
// (so callers can re-layout it later, e.g. after an edit changes a box's
// size), its boxes and connector edges in canvas coordinates, plus the
// underlying nodes (in the same order as boxes) for tree-structured
// navigation.
func buildScene(spec storage.Node) (root *node, boxes []box, edges []edge, nodes []*node) {
	root = buildTree(spec)
	layoutTree(root)
	flattenTree(root, &boxes, &edges, &nodes)
	return root, boxes, edges, nodes
}

// relayout re-runs the tree layout on root (e.g. after a box's text edit
// changed its width/height) and re-flattens it, returning fresh boxes,
// edges, and nodes in the same order/idx as before, since flattening walks
// the tree structure the same way every time.
func relayout(root *node) (boxes []box, edges []edge, nodes []*node) {
	layoutTree(root)
	flattenTree(root, &boxes, &edges, &nodes)
	return boxes, edges, nodes
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

// edgeCell is a single character position belonging to a drawn edge, and
// the direction(s) the line runs through it. Accumulating these (via OR)
// per position, across all edges, is what lets branching/merging lines
// render as proper junction characters instead of one edge's stroke
// silently overwriting another's.
type edgeCell struct {
	x, y int
	dir  direction
}

// edgeCells computes the cells of an orthogonal (elbow) connector from
// e.x1,e.y1 to e.x2,e.y2: out horizontally from the left box, a vertical
// jog at the midpoint between the two columns, then horizontally into the
// right box. Siblings sharing a parent and column also share their jog's x
// position, so they read as branches off one vertical trunk.
//
// Each cell only carries the direction(s) this one edge threads through
// it (e.g. a plain horizontal run is east+west, an elbow is two adjacent
// directions); which box-drawing rune a cell ends up as depends on
// merging in every edge that touches it, so that step is deferred to
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

// dirToRune converts an accumulated set of directions at a cell into the
// single box-drawing character that represents all of them: a straight
// line, a rounded elbow where exactly two adjacent directions meet, or a
// square junction (├ ┤ ┬ ┴ ┼) where three or four lines meet.
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
