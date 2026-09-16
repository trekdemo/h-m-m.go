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
	children []*node
	subtreeH int // cached: vertical span this node's whole subtree occupies
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

	n := &node{
		box: box{
			width:  w,
			height: len(lines),
			lines:  lines,
			color:  levelColors[depth%len(levelColors)],
		},
	}
	for _, childSpec := range spec.children {
		n.children = append(n.children, buildTree(childSpec, depth+1))
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
// child's left-middle edge.
func flattenTree(n *node, boxes *[]box, edges *[]edge) {
	*boxes = append(*boxes, n.box)
	for _, c := range n.children {
		*edges = append(*edges, edge{
			x1: n.box.x + n.box.width,
			y1: n.box.y + n.box.height/2,
			x2: c.box.x,
			y2: c.box.y + c.box.height/2,
		})
		flattenTree(c, boxes, edges)
	}
}

// buildDemoScene builds and lays out the demo tree, returning its boxes
// and connector edges in canvas coordinates.
func buildDemoScene() ([]box, []edge) {
	root := buildTree(demoTree, 0)
	layoutTree(root)

	var boxes []box
	var edges []edge
	flattenTree(root, &boxes, &edges)
	return boxes, edges
}

// drawLine plots a straight connector line from e.x1,e.y1 to e.x2,e.y2
// using Bresenham's algorithm, choosing a box-drawing character that
// matches the line's direction, and reports each point through set.
func drawLine(e edge, set func(x, y int, r rune, c lipgloss.Color)) {
	dx, dy := e.x2-e.x1, e.y2-e.y1

	ch := '─'
	switch {
	case dx == 0:
		ch = '│'
	case dy == 0:
		ch = '─'
	case (dy > 0) == (dx > 0):
		ch = '╲'
	default:
		ch = '╱'
	}

	x, y := e.x1, e.y1
	absDX, absDY := abs(dx), abs(dy)
	sx, sy := sign(dx), sign(dy)
	errTerm := absDX - absDY
	for {
		set(x, y, ch, edgeColor)
		if x == e.x2 && y == e.y2 {
			break
		}
		e2 := 2 * errTerm
		if e2 > -absDY {
			errTerm -= absDY
			x += sx
		}
		if e2 < absDX {
			errTerm += absDX
			y += sy
		}
	}
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

func sign(n int) int {
	switch {
	case n > 0:
		return 1
	case n < 0:
		return -1
	default:
		return 0
	}
}
