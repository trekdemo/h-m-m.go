package main

import (
	"image/color"
)

import (
	lipgloss "charm.land/lipgloss/v2"

	"github.com/trekdemo/bubbletea-exp/navigator"
)

const (
	boxTextWidth = 20 // max characters per line inside a box
	colGap       = 6  // horizontal gap between tree columns (room for connector lines)
	rowGap       = 1  // minimum vertical gap kept between sibling subtrees
)

var levelColors = []color.Color{
	lipgloss.Color("#FF6AC1"),
	lipgloss.Color("#4EA8DE"),
	lipgloss.Color("#FFB454"),
	lipgloss.Color("#7EE787"),
	lipgloss.Color("#B399FF"),
}

var edgeColor = lipgloss.Color("#484f58")

// treeSpec is the static definition of the demo hierarchy: a node's text
// and its children, before any layout has happened.
type treeSpec struct {
	text     string
	children []treeSpec
}

var demoTree = treeSpec{
	text: "Bubble Tea",
	children: []treeSpec{
		{
			text: "Rendering",
			children: []treeSpec{
				{text: "Lip Gloss styling"},
				{text: "Rounded borders"},
				{text: "Word wrap at 20 chars max"},
			},
		},
		{
			text: "Layout",
			children: []treeSpec{
				{
					text: "Sugiyama framework",
					children: []treeSpec{
						{text: "Layer assignment"},
						{text: "Crossing minimization"},
						{text: "Coordinate assignment"},
					},
				},
				{text: "Tree depth becomes column"},
			},
		},
		{
			text: "Interaction",
			children: []treeSpec{
				{text: "Click and drag"},
				{text: "Pans like a map"},
			},
		},
		{text: "Go is fun"},
	},
}

// node is a treeSpec after box rendering and layout: it carries its
// rendered box plus the tree structure needed to lay it out and draw
// connector lines to its children.
type node struct {
	box      box
	parent   *node
	children []*node
	idx      int // index into the flattened boxes/nodes slices
}

// Bounds, Parent, and Children implement navigator.Node, letting node be
// navigated over without the navigator package knowing about tree.go's
// layout or rendering details.
func (n *node) Bounds() navigator.Rect {
	return navigator.Rect{X: n.box.x, Y: n.box.y, Width: n.box.width, Height: n.box.height}
}

func (n *node) Parent() navigator.Node {
	if n.parent == nil {
		return nil
	}
	return n.parent
}

func (n *node) Children() []navigator.Node {
	if len(n.children) == 0 {
		return nil
	}
	out := make([]navigator.Node, len(n.children))
	for i, c := range n.children {
		out[i] = c
	}
	return out
}

// navNodes converts a flat node slice into navigator.Node, for building a
// navigator over the whole tree's nodes.
func navNodes(nodes []*node) []navigator.Node {
	out := make([]navigator.Node, len(nodes))
	for i, n := range nodes {
		out[i] = n
	}
	return out
}

// edge is a straight connector line from a parent box to a child box, in
// canvas coordinates.
type edge struct {
	x1, y1, x2, y2 int
}

// buildTree renders each spec node into a box (wrapping text to
// boxTextWidth, no color yet) and returns the corresponding node tree.
// Color is assigned by depth so each tree level reads as a distinct band.
func buildTree(spec treeSpec, depth int) *node {
	color := levelColors[depth%len(levelColors)]

	base := lipgloss.NewStyle().
		Width(boxTextWidth).
		Padding(0, 1).
		Foreground(color).
		BorderForeground(color)
	style := base.Border(lipgloss.RoundedBorder())
	selStyle := base.Border(lipgloss.DoubleBorder())

	rendered := style.Render(spec.text)

	n := &node{
		box: box{
			width:    lipgloss.Width(rendered),
			height:   lipgloss.Height(rendered),
			text:     spec.text,
			style:    style,
			selStyle: selStyle,
		},
	}
	for _, childSpec := range spec.children {
		child := buildTree(childSpec, depth+1)
		child.parent = n
		n.children = append(n.children, child)
	}
	return n
}

// layoutTree runs the tree-specialized Sugiyama steps: layer assignment
// (depth -> column/x), then a coordinate-assignment pass (subtree-band
// packing -> y) that centers each parent over the vertical span of its
// children without ever letting two boxes overlap.
func layoutTree(root *node) {
	colWidth := map[int]int{}
	var measureColumns func(n *node, depth int)
	measureColumns = func(n *node, depth int) {
		if n.box.width > colWidth[depth] {
			colWidth[depth] = n.box.width
		}
		for _, c := range n.children {
			measureColumns(c, depth+1)
		}
	}
	measureColumns(root, 0)

	colX := map[int]int{0: 0}
	for d := 1; d <= len(colWidth); d++ {
		colX[d] = colX[d-1] + colWidth[d-1] + colGap
	}

	var assignX func(n *node, depth int)
	assignX = func(n *node, depth int) {
		n.box.x = colX[depth]
		for _, c := range n.children {
			assignX(c, depth+1)
		}
	}
	assignX(root, 0)

	layoutContour(root, 0)
}

// span is the vertical extent, at one tree depth, occupied by (part of) a
// subtree, in coordinates local to that subtree's own top.
type span struct{ min, max int }

// contour maps depth -> the vertical span a subtree occupies at that
// depth. Two subtrees can only ever collide at a depth (column) they both
// have nodes in, since box.x is fixed per depth; a leaf's contour has a
// single entry (its own depth), so comparing it against a neighboring
// subtree only ever checks that shared depth, not the neighbor's deeper
// descendants.
type contour map[int]span

// mergeInto folds add (shifted by offset) into base, widening any shared
// depth's span and copying over any depth base doesn't have yet.
func mergeInto(base contour, add contour, offset int) {
	for d, s := range add {
		shifted := span{s.min + offset, s.max + offset}
		if existing, ok := base[d]; ok {
			base[d] = span{min(existing.min, shifted.min), max(existing.max, shifted.max)}
		} else {
			base[d] = shifted
		}
	}
}

// requiredOffset returns how far down `next` must be shifted so that, at
// every depth it shares with `placed`, it clears placed's bottom edge by
// rowGap. Depths only one of the two subtrees occupies impose no
// constraint at all.
func requiredOffset(placed contour, next contour) int {
	offset := 0
	for d, s := range next {
		if p, ok := placed[d]; ok {
			if need := p.max + rowGap - s.min; need > offset {
				offset = need
			}
		}
	}
	return offset
}

// shiftSubtree moves n and all of its descendants down by dy.
func shiftSubtree(n *node, dy int) {
	n.box.y += dy
	for _, c := range n.children {
		shiftSubtree(c, dy)
	}
}

// layoutContour assigns each node a y coordinate local to the whole tree's
// origin. It packs a node's children as tightly as their contours allow:
// two children are only pushed apart at depths where they both actually
// have boxes, so a childless node never gets shoved down to make room for
// a sibling's grandchildren sitting in an unrelated column.
func layoutContour(n *node, depth int) contour {
	if len(n.children) == 0 {
		n.box.y = 0
		return contour{depth: {0, n.box.height}}
	}

	placed := contour{}
	for i, c := range n.children {
		childContour := layoutContour(c, depth+1)
		offset := 0
		if i > 0 {
			offset = requiredOffset(placed, childContour)
		}
		shiftSubtree(c, offset)
		mergeInto(placed, childContour, offset)
	}

	first, last := n.children[0], n.children[len(n.children)-1]
	mid := (first.box.y + first.box.height/2 + last.box.y + last.box.height/2) / 2
	n.box.y = mid - n.box.height/2
	mergeInto(placed, contour{depth: {0, n.box.height}}, n.box.y)

	// Normalize so the subtree's own top (across n and all descendants)
	// sits at local y=0, matching the convention leaf subtrees return.
	top := 0
	for _, s := range placed {
		if s.min < top {
			top = s.min
		}
	}
	if top != 0 {
		shiftSubtree(n, -top)
		shifted := contour{}
		mergeInto(shifted, placed, -top)
		placed = shifted
	}
	return placed
}

// flattenTree walks the laid-out tree, collecting every box plus a
// straight connector edge from each parent's right-middle edge to each
// child's left-middle edge. It also assigns each node its flat index and
// collects the nodes themselves, in the same order as boxes, so callers
// can navigate the tree structure (parent/children/siblings) by index.
func flattenTree(n *node, boxes *[]box, edges *[]edge, nodes *[]*node) {
	n.idx = len(*boxes)
	*boxes = append(*boxes, n.box)
	*nodes = append(*nodes, n)
	for _, c := range n.children {
		*edges = append(*edges, edge{
			x1: n.box.x + n.box.width,
			y1: n.box.y + n.box.height/2,
			x2: c.box.x,
			y2: c.box.y + c.box.height/2,
		})
		flattenTree(c, boxes, edges, nodes)
	}
}

// buildDemoScene builds and lays out the demo tree, returning the root node
// (so callers can re-layout it later, e.g. after an edit changes a box's
// size), its boxes and connector edges in canvas coordinates, plus the
// underlying nodes (in the same order as boxes) for tree-structured
// navigation.
func buildDemoScene() (root *node, boxes []box, edges []edge, nodes []*node) {
	root = buildTree(demoTree, 0)
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
// e.x1,e.y1 to e.x2,e.y2: out horizontally from the parent, a vertical
// jog at the midpoint between the two columns, then horizontally into the
// child. Siblings sharing a parent and column also share their jog's x
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
