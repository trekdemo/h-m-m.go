package layout

import (
	"math/rand"
	"testing"

	"hmm/geom"
)

// testNode is the smallest SidedNode: a box of a given size, so layout
// tests can use sizes and shapes the app's text boxes never produce.
type testNode struct {
	size     geom.Size
	pos      geom.Point
	side     Side
	overhang int
	children []*testNode
}

func (n *testNode) Size() geom.Size          { return n.size }
func (n *testNode) Children() []*testNode    { return n.children }
func (n *testNode) SetPosition(p geom.Point) { n.pos = p }
func (n *testNode) Overhang() int            { return n.overhang }
func (n *testNode) Side() Side               { return n.side }
func (n *testNode) SetSide(s Side)           { n.side = s }

func (n *testNode) Bounds() geom.Rect {
	return geom.Rect{X: n.pos.X, Y: n.pos.Y, Width: n.size.Width, Height: n.size.Height}
}

// box builds a w by h node with the given children.
func box(w, h int, children ...*testNode) *testNode {
	return &testNode{size: geom.Size{Width: w, Height: h}, children: children}
}

var mindmap = Mindmap[*testNode]{}

func arrange(root *testNode) { mindmap.Arrange(root) }

// eachDescendant calls f on n and every node below it.
func eachDescendant(n *testNode, f func(*testNode)) {
	f(n)
	for _, c := range n.children {
		eachDescendant(c, f)
	}
}

func TestConnectorMirrorsAcrossSides(t *testing.T) {
	root := box(4, 1, box(5, 1), box(3, 1))
	arrange(root)
	right, left := root.children[0], root.children[1]

	// Cell x on the right mirrors to cell mirror-x on the left, where
	// mirror is the root's first and last cell summed.
	mirror := root.pos.X + root.pos.X + root.size.Width - 1
	r, l := mindmap.Route(root, right), mindmap.Route(root, left)
	rStart, rEnd := r[0], r[len(r)-1]
	lStart, lEnd := l[0], l[len(l)-1]
	if rStart.X+lStart.X != mirror || rEnd.X+lEnd.X != mirror {
		t.Fatalf("connectors not mirrored around the root: right %+v, left %+v", r, l)
	}
	if rTrunk, lTrunk := (rStart.X+rEnd.X)/2, (lStart.X+lEnd.X)/2; rTrunk+lTrunk != mirror {
		t.Fatalf("trunks not mirrored: right x=%d, left x=%d", rTrunk, lTrunk)
	}
}

func TestTidyLayoutRandomTrees(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	var grow func(depth int) *testNode
	grow = func(depth int) *testNode {
		var children []*testNode
		if depth < 5 {
			for range rng.Intn(5) {
				children = append(children, grow(depth+1))
			}
		}
		return box(3+rng.Intn(30), 1+rng.Intn(6), children...)
	}

	for iter := range 200 {
		root := grow(0)
		arrange(root)

		var all []*testNode
		eachDescendant(root, func(n *testNode) { all = append(all, n) })

		for i, a := range all {
			for _, b := range all[i+1:] {
				// Boxes sharing any column must also keep RowGap between them.
				if a.pos.X < b.pos.X+b.size.Width && b.pos.X < a.pos.X+a.size.Width &&
					a.pos.Y < b.pos.Y+b.size.Height+RowGap && b.pos.Y < a.pos.Y+a.size.Height+RowGap {
					t.Fatalf("iter %d: boxes too close: %+v vs %+v", iter, a, b)
				}
			}
		}

		eachDescendant(root, func(n *testNode) {
			if n == root || len(n.children) == 0 {
				return
			}
			for _, c := range n.children {
				right := c.pos.X == n.pos.X+n.size.Width+ColGap
				left := c.pos.X+c.size.Width+ColGap == n.pos.X
				if !right && !left {
					t.Fatalf("iter %d: child %+v not ColGap past its parent %+v", iter, c, n)
				}
			}
			first, last := n.children[0], n.children[len(n.children)-1]
			span := first.pos.Y + last.pos.Y + last.size.Height
			if d := 2*n.pos.Y + n.size.Height - span; d < -2 || d > 2 {
				t.Fatalf("iter %d: parent %+v not centered on children %+v..%+v", iter, n, first, last)
			}
		})
	}
}

func TestTidyLayoutIsNonLayered(t *testing.T) {
	wide := box(30, 1, box(5, 1))
	narrow := box(5, 1, box(5, 1))
	root := box(5, 1, box(5, 1, wide, narrow))
	arrange(root)

	gw, gn := wide.children[0], narrow.children[0]
	if gw.pos.X == gn.pos.X {
		t.Fatalf("grandchildren share a column (x=%d); each should sit just past its own parent", gw.pos.X)
	}
	if gn.pos.X != narrow.pos.X+narrow.size.Width+ColGap {
		t.Fatalf("narrow parent's child at x=%d, want %d", gn.pos.X, narrow.pos.X+narrow.size.Width+ColGap)
	}
}

func TestTidyLayoutSpacesSmallSiblingsEvenly(t *testing.T) {
	tall := func() *testNode {
		var kids []*testNode
		for range 8 {
			kids = append(kids, box(5, 3))
		}
		return box(5, 3, kids...)
	}
	mid := []*testNode{box(5, 3), box(5, 3), box(5, 3)}
	parent := box(5, 3, append(append([]*testNode{tall()}, mid...), tall())...)
	root := box(5, 3, parent)
	arrange(root)

	var gaps []int
	for i := 1; i < len(parent.children); i++ {
		prev, cur := parent.children[i-1], parent.children[i]
		gaps = append(gaps, cur.pos.Y-(prev.pos.Y+prev.size.Height))
	}
	for _, g := range gaps {
		if g-gaps[0] < -1 || g-gaps[0] > 1 {
			t.Fatalf("gaps between siblings = %v, want them (nearly) equal", gaps)
		}
	}
	if gaps[0] <= RowGap {
		t.Fatalf("gaps = %v: the small siblings were packed, not spread", gaps)
	}
}
