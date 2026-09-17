package main

import "strings"

import "github.com/charmbracelet/lipgloss"

const (
	boxTextWidth = 20 // max characters per line inside a box
	colGap       = 6  // horizontal gap between tree columns (room for connector lines)
	rowGap       = 1  // minimum vertical gap kept between sibling subtrees
)

var levelColors = []lipgloss.Color{
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
	subtreeH int // cached: vertical span this node's whole subtree occupies
	idx      int // index into the flattened boxes/nodes slices
}

// siblingIndex returns n's position among its parent's children, or -1 if
// n is the root (has no parent).
func siblingIndex(n *node) int {
	if n.parent == nil {
		return -1
	}
	for i, c := range n.parent.children {
		if c == n {
			return i
		}
	}
	return -1
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
	plainStyle := lipgloss.NewStyle().
		Width(boxTextWidth).
		Padding(0, 1).
		Border(lipgloss.RoundedBorder())

	lines := strings.Split(plainStyle.Render(spec.text), "\n")
	w := 0
	for _, l := range lines {
		if lipgloss.Width(l) > w {
			w = lipgloss.Width(l)
		}
	}

	selStyle := lipgloss.NewStyle().
		Width(boxTextWidth).
		Padding(0, 1).
		Border(lipgloss.DoubleBorder())
	selLines := strings.Split(selStyle.Render(spec.text), "\n")

	n := &node{
		box: box{
			width:    w,
			height:   len(lines),
			lines:    lines,
			selLines: selLines,
			color:    levelColors[depth%len(levelColors)],
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

	var computeSubtreeHeights func(n *node) int
	computeSubtreeHeights = func(n *node) int {
		if len(n.children) == 0 {
			n.subtreeH = n.box.height
			return n.subtreeH
		}
		total := 0
		for i, c := range n.children {
			if i > 0 {
				total += rowGap
			}
			total += computeSubtreeHeights(c)
		}
		n.subtreeH = max(total, n.box.height)
		return n.subtreeH
	}
	computeSubtreeHeights(root)

	var place func(n *node, top int)
	place = func(n *node, top int) {
		if len(n.children) == 0 {
			n.box.y = top + (n.subtreeH-n.box.height)/2
			return
		}
		childTop := top
		for _, c := range n.children {
			place(c, childTop)
			childTop += c.subtreeH + rowGap
		}
		first, last := n.children[0], n.children[len(n.children)-1]
		mid := (first.box.y + first.box.height/2 + last.box.y + last.box.height/2) / 2
		n.box.y = mid - n.box.height/2
	}
	place(root, 0)
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

// buildDemoScene builds and lays out the demo tree, returning its boxes
// and connector edges in canvas coordinates, plus the underlying nodes
// (in the same order as boxes) for tree-structured navigation.
func buildDemoScene() ([]box, []edge, []*node) {
	root := buildTree(demoTree, 0)
	layoutTree(root)

	var boxes []box
	var edges []edge
	var nodes []*node
	flattenTree(root, &boxes, &edges, &nodes)
	return boxes, edges, nodes
}

// edgeCell is a single character position belonging to a drawn edge.
type edgeCell struct {
	x, y int
	ch   rune
}

// edgeCells computes an orthogonal (elbow) connector from e.x1,e.y1 to
// e.x2,e.y2: out horizontally from the parent, a vertical jog at the
// midpoint between the two columns, then horizontally into the child.
// Corners use rounded box-drawing characters to match the box borders.
// Siblings sharing a parent and column also share their jog's x
// position, so they read as branches off one vertical trunk.
//
// Straight cells and corner cells are returned separately: when several
// sibling edges share a trunk column, one sibling's vertical stroke can
// pass directly through another's corner cell, so callers must draw every
// edge's straight cells first and only then draw corners on top, or a
// longer sibling's trunk stroke will stomp a shorter sibling's elbow.
func edgeCells(e edge) (straights, corners []edgeCell) {
	if e.y1 == e.y2 {
		for x := e.x1; x <= e.x2; x++ {
			straights = append(straights, edgeCell{x, e.y1, '─'})
		}
		return straights, nil
	}

	midX := (e.x1 + e.x2) / 2
	for x := e.x1; x < midX; x++ {
		straights = append(straights, edgeCell{x, e.y1, '─'})
	}
	for x := midX + 1; x <= e.x2; x++ {
		straights = append(straights, edgeCell{x, e.y2, '─'})
	}

	dy := 1
	corner1, corner2 := '╮', '╰' // going down: west+south, then north+east
	if e.y2 < e.y1 {
		dy = -1
		corner1, corner2 = '╯', '╭' // going up: west+north, then south+east
	}
	corners = append(corners, edgeCell{midX, e.y1, corner1})
	for y := e.y1 + dy; y != e.y2; y += dy {
		straights = append(straights, edgeCell{midX, y, '│'})
	}
	corners = append(corners, edgeCell{midX, e.y2, corner2})

	return straights, corners
}
