package layout

import "hmm/geom"

const (
	ColGap = 6 // horizontal gap between a parent and its children (room for connector lines)
	RowGap = 1 // minimum vertical gap kept between sibling subtrees
)

// Side is the direction a half of the tree grows away from the root in.
// The zero value means a root child hasn't been given a side yet.
type Side int

const (
	Right Side = 1
	Left  Side = -1
)

// SidedNode is a Node that remembers which side of the root it grows on.
// Mindmap sets the side of each of the root's children the first time it
// sees them and keeps it from then on, so the node, not the layout, holds
// it: tree edits can then keep a branch on its side, such as a new
// sibling joining the half of the map its neighbour is on.
type SidedNode[N any] interface {
	Node[N]
	Side() Side
	SetSide(Side)
}

// Mindmap grows the map in both directions from the root, like Mermaid's
// tidy-tree mindmap layout, so it stays compact instead of sprawling to
// one side. It pins the root at the origin, so editing never makes it
// jump, splits the root's branches between the sides (see splitSides),
// and lays out each side as its own tidy tree (see layoutSide).
type Mindmap[N SidedNode[N]] struct{}

func (Mindmap[N]) Arrange(root N) {
	root.SetPosition(geom.Point{})
	right, left := splitSides(root.Children())
	layoutSide(root, right, Right)
	layoutSide(root, left, Left)
}

// Route joins the facing middles of a parent and child box with an elbow
// that jogs vertically halfway between them. Siblings share a parent end
// and so a jog column, so their jogs read as branches off one vertical
// trunk. The child end starts one cell inside the child's box, hidden
// under it, so the line meets the box without a gap; the left side
// mirrors the right cell for cell.
func (Mindmap[N]) Route(parent, child N) []geom.Point {
	p, c := parent.Bounds(), child.Bounds()
	x1, x2 := p.X+p.Width, c.X
	if c.X < p.X {
		x1, x2 = p.X-1, c.X+c.Width-1
	}
	y1, y2 := p.Y+p.Height/2, c.Y+c.Height/2
	if y1 == y2 {
		return []geom.Point{{X: x1, Y: y1}, {X: x2, Y: y2}}
	}
	mid := (x1 + x2) / 2
	return []geom.Point{{X: x1, Y: y1}, {X: mid, Y: y1}, {X: mid, Y: y2}, {X: x2, Y: y2}}
}

// splitSides divides the root's children between the two sides, keeping
// each child on the side it already has so adding, removing or reordering
// siblings never moves a branch across while the app runs. Sides aren't
// saved, so a child without one yet (every branch of a freshly loaded
// tree, or a newly added one) goes to whichever side currently has fewer
// branches, the right one on a tie: a loaded tree alternates right, left,
// right... like Mermaid's, and a root with a single child still reads
// left-to-right.
func splitSides[N SidedNode[N]](children []N) (right, left []N) {
	nRight, nLeft := 0, 0
	for _, c := range children {
		switch c.Side() {
		case Right:
			nRight++
		case Left:
			nLeft++
		}
	}
	for _, c := range children {
		if c.Side() == 0 {
			if nRight <= nLeft {
				c.SetSide(Right)
				nRight++
			} else {
				c.SetSide(Left)
				nLeft++
			}
		}
		if c.Side() == Right {
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
// sibling subtrees pack as closely as RowGap allows. The tidy layout grows
// downward, so it's fed boxes with their axes swapped (a box's height as
// tidy width, its width as tidy depth) and the result is rotated back. The
// branches hang under a virtual root the size of the real one, so the real
// root lands centered on its first level. The left side mirrors the right
// around the root, so a parent's children always line up on the edge
// facing it and their connectors meet at a shared trunk.
func layoutSide[N SidedNode[N]](root N, children []N, dir Side) {
	if len(children) == 0 {
		return
	}

	// nodes maps each tidyTree's id back to the node it lays out.
	var nodes []N
	var convert func(n N, y float64) *tidyTree
	convert = func(n N, y float64) *tidyTree {
		size := n.Size()
		t := &tidyTree{
			id: len(nodes),
			y:  y,
			w:  float64(size.Height + RowGap),
			h:  float64(size.Width + reach(n)),
		}
		nodes = append(nodes, n)
		for _, c := range n.Children() {
			t.c = append(t.c, convert(c, y+t.h))
		}
		return t
	}
	r := root.Bounds()
	virtual := &tidyTree{
		id: -1,
		w:  float64(r.Height + RowGap),
		h:  float64(r.Width + ColGap),
	}
	for _, c := range children {
		virtual.c = append(virtual.c, convert(c, virtual.h))
	}

	tidyLayout(virtual)

	// Shift breadth so the virtual root lands exactly on the real one,
	// which centers the root on this side's first level.
	shift := float64(r.Y) - virtual.x
	var place func(t *tidyTree)
	place = func(t *tidyTree) {
		n := nodes[t.id]
		depth := int(t.y)
		x := r.X + depth
		if dir == Left {
			x = r.X + r.Width - depth - n.Size().Width
		}
		n.SetPosition(geom.Point{X: x, Y: floorInt(t.x + shift)})
		for _, c := range t.c {
			place(c)
		}
	}
	for _, c := range virtual.c {
		place(c)
	}
}

// reach is how far past its box a node claims room in the layout. It's
// usually ColGap, the room its connectors need. Anything overhanging into
// that room, such as a collapsed node's marker, claims its own width plus
// a cell of space if that's more, keeping it clear of neighbouring
// subtrees that the tidy layout would otherwise pack right up against it.
func reach[N Node[N]](n N) int {
	return max(ColGap, n.Overhang()+1)
}
