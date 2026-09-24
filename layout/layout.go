// Package layout positions a tree of rectangular boxes on an integer grid
// as a bidirectional mindmap, like Mermaid's tidy-tree mindmap layout: the
// root sits at the origin and each of its children grows to the right or
// to the left of it, every side laid out as its own non-layered tidy tree
// (see tidy.go) rotated so depth runs horizontally.
//
// It only knows each node's size and place in the tree, never how a box is
// drawn: callers measure their boxes however they render them, and read
// the positions back through Node.
package layout

// Side is the direction a half of the tree grows away from the root in.
// The zero value means a root child hasn't been given a side yet.
type Side int

const (
	Right Side = 1
	Left  Side = -1
)

// Node is a box in the tree being laid out. Methods are prefixed so a type
// can implement Node alongside other tree interfaces (e.g. navigation or
// storage) without their names clashing.
type Node interface {
	// LayoutSize is the box's width and height, in grid cells.
	LayoutSize() (width, height int)
	// LayoutChildren are the node's children, in order.
	LayoutChildren() []Node
	// SetLayoutPosition receives the box's top-left corner.
	SetLayoutPosition(x, y int)
	// LayoutSide and SetLayoutSide store which side of the root a root
	// child grows on, so it stays there across layouts. They're only
	// called on the root's children.
	LayoutSide() Side
	SetLayoutSide(Side)
}

// Spacing is the room kept around boxes.
type Spacing struct {
	// Depth is the horizontal gap between a parent and its children.
	Depth int
	// Breadth is the minimum vertical gap between sibling subtrees.
	Breadth int
}

// Mindmap lays the tree out bidirectionally: root at (0, 0), and each of
// its children on the side it already has, or on a side chosen by
// SplitSides. The root stays at the origin across layouts, so editing
// never makes it jump.
func Mindmap(root Node, s Spacing) {
	root.SetLayoutPosition(0, 0)
	right, left := SplitSides(root.LayoutChildren())
	w, h := root.LayoutSize()
	layoutSide(w, h, right, Right, s)
	layoutSide(w, h, left, Left, s)
}

// SplitSides divides the root's children between the two sides, keeping
// each child on the side it already has so adding, removing or reordering
// siblings never moves a branch across. A child without one yet (every
// branch of a freshly loaded tree, or a newly added one) goes to whichever
// side currently has fewer branches, the right one on a tie: a fresh tree
// alternates right, left, right... like Mermaid's, and a root with a
// single child still reads left-to-right.
func SplitSides(children []Node) (right, left []Node) {
	nRight, nLeft := 0, 0
	for _, c := range children {
		switch c.LayoutSide() {
		case Right:
			nRight++
		case Left:
			nLeft++
		}
	}
	for _, c := range children {
		if c.LayoutSide() == 0 {
			if nRight <= nLeft {
				c.SetLayoutSide(Right)
				nRight++
			} else {
				c.SetLayoutSide(Left)
				nLeft++
			}
		}
		if c.LayoutSide() == Right {
			right = append(right, c)
		} else {
			left = append(left, c)
		}
	}
	return right, left
}

// layoutSide positions children (a subset of the root's children) and
// their descendants on one side of a rootW x rootH root at the origin.
// They're laid out as a tidy tree under a virtual root the size of the
// real one, rotated so the tidy tree's depth axis runs away from the root
// horizontally and its breadth axis runs down. Each child sits s.Depth
// past its own parent's far edge, not in a column shared with every other
// node at its depth, and sibling subtrees keep at least s.Breadth between
// them wherever they'd otherwise meet. The left side is the right side
// mirrored around the root, so a parent's children always line up on the
// edge facing it.
func layoutSide(rootW, rootH int, children []Node, dir Side, s Spacing) {
	if len(children) == 0 {
		return
	}

	var convert func(n Node, y float64) *tidyTree
	convert = func(n Node, y float64) *tidyTree {
		w, h := n.LayoutSize()
		t := &tidyTree{
			n:    n,
			boxW: w,
			y:    y,
			w:    float64(h + s.Breadth),
			h:    float64(w + s.Depth),
		}
		for _, c := range n.LayoutChildren() {
			t.c = append(t.c, convert(c, y+t.h))
		}
		return t
	}
	virtual := &tidyTree{
		w: float64(rootH + s.Breadth),
		h: float64(rootW + s.Depth),
	}
	for _, c := range children {
		virtual.c = append(virtual.c, convert(c, virtual.h))
	}

	tidyLayout(virtual)

	// Shift breadth so the virtual root lands exactly on the real one,
	// which centers the root on this side's first level.
	shift := -virtual.x
	var place func(t *tidyTree)
	place = func(t *tidyTree) {
		depth := int(t.y)
		x := depth
		if dir == Left {
			x = rootW - depth - t.boxW
		}
		t.n.SetLayoutPosition(x, floorInt(t.x+shift))
		for _, c := range t.c {
			place(c)
		}
	}
	for _, c := range virtual.c {
		place(c)
	}
}
