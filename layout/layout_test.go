package layout

import (
	"math/rand"
	"testing"
)

// box is a minimal Node: a size to lay out and the position it gets.
type box struct {
	x, y, w, h int
	side       Side
	children   []*box
}

func (b *box) LayoutSize() (int, int) { return b.w, b.h }

func (b *box) LayoutChildren() []Node {
	out := make([]Node, len(b.children))
	for i, c := range b.children {
		out[i] = c
	}
	return out
}

func (b *box) SetLayoutPosition(x, y int) { b.x, b.y = x, y }
func (b *box) LayoutSide() Side           { return b.side }
func (b *box) SetLayoutSide(s Side)       { b.side = s }

func sized(w, h int, children ...*box) *box {
	return &box{w: w, h: h, children: children}
}

func each(b *box, f func(*box)) {
	f(b)
	for _, c := range b.children {
		each(c, f)
	}
}

var testSpacing = Spacing{Depth: 6, Breadth: 1}

func TestMindmapRandomTrees(t *testing.T) {
	s := testSpacing
	rng := rand.New(rand.NewSource(1))
	var grow func(depth int) *box
	grow = func(depth int) *box {
		var children []*box
		if depth < 5 {
			for range rng.Intn(5) {
				children = append(children, grow(depth+1))
			}
		}
		return sized(3+rng.Intn(30), 1+rng.Intn(6), children...)
	}

	for iter := range 200 {
		root := grow(0)
		root.x, root.y = 17, -4 // must be reset to the origin
		Mindmap(root, s)
		if root.x != 0 || root.y != 0 {
			t.Fatalf("iter %d: root at (%d, %d), want (0, 0)", iter, root.x, root.y)
		}

		var all []*box
		each(root, func(b *box) { all = append(all, b) })

		for i, a := range all {
			for _, b := range all[i+1:] {
				// Boxes sharing any column must also keep Breadth between them.
				if a.x < b.x+b.w && b.x < a.x+a.w &&
					a.y < b.y+b.h+s.Breadth && b.y < a.y+a.h+s.Breadth {
					t.Fatalf("iter %d: boxes too close: %+v vs %+v", iter, *a, *b)
				}
			}
		}

		each(root, func(n *box) {
			if len(n.children) == 0 {
				return
			}
			for _, c := range n.children {
				right := c.x == n.x+n.w+s.Depth
				left := c.x+c.w+s.Depth == n.x
				if !right && !left {
					t.Fatalf("iter %d: child %+v not Depth past its parent %+v", iter, *c, *n)
				}
			}
			if n == root {
				return // centered per side, not across both
			}
			first, last := n.children[0], n.children[len(n.children)-1]
			span := first.y + last.y + last.h
			if d := 2*n.y + n.h - span; d < -2 || d > 2 {
				t.Fatalf("iter %d: parent %+v not centered on children %+v..%+v", iter, *n, *first, *last)
			}
		})
	}
}

func TestMindmapIsNonLayered(t *testing.T) {
	wide := sized(30, 1, sized(5, 1))
	narrow := sized(5, 1, sized(5, 1))
	root := sized(5, 1, sized(5, 1, wide, narrow))
	Mindmap(root, testSpacing)

	gw, gn := wide.children[0], narrow.children[0]
	if gw.x == gn.x {
		t.Fatalf("grandchildren share a column (x=%d); each should sit just past its own parent", gw.x)
	}
	if want := narrow.x + narrow.w + testSpacing.Depth; gn.x != want {
		t.Fatalf("narrow parent's child at x=%d, want %d", gn.x, want)
	}
}

func TestMindmapSpacesSmallSiblingsEvenly(t *testing.T) {
	tall := func() *box {
		var kids []*box
		for range 8 {
			kids = append(kids, sized(5, 3))
		}
		return sized(5, 3, kids...)
	}
	mid := []*box{sized(5, 3), sized(5, 3), sized(5, 3)}
	parent := sized(5, 3, append(append([]*box{tall()}, mid...), tall())...)
	root := sized(5, 3, parent)
	Mindmap(root, testSpacing)

	var gaps []int
	for i := 1; i < len(parent.children); i++ {
		prev, cur := parent.children[i-1], parent.children[i]
		gaps = append(gaps, cur.y-(prev.y+prev.h))
	}
	for _, g := range gaps {
		if g-gaps[0] < -1 || g-gaps[0] > 1 {
			t.Fatalf("gaps between siblings = %v, want them (nearly) equal", gaps)
		}
	}
	if gaps[0] <= testSpacing.Breadth {
		t.Fatalf("gaps = %v: the small siblings were packed, not spread", gaps)
	}
}

func TestMindmapMirrorsLeftSide(t *testing.T) {
	right := sized(7, 2, sized(4, 1), sized(9, 3))
	left := sized(7, 2, sized(4, 1), sized(9, 3))
	root := sized(11, 3, right, left)
	Mindmap(root, testSpacing)

	if right.side != Right || left.side != Left {
		t.Fatalf("sides = %v, %v, want Right, Left", right.side, left.side)
	}
	// Mirroring around the root maps x to root.w - x - w.
	pairs := [][2]*box{{right, left}}
	for i := range right.children {
		pairs = append(pairs, [2]*box{right.children[i], left.children[i]})
	}
	for _, p := range pairs {
		r, l := p[0], p[1]
		if l.x != root.w-r.x-r.w || l.y != r.y {
			t.Errorf("left %+v doesn't mirror right %+v", *l, *r)
		}
	}
	// Left-side children are right-aligned on the edge facing their parent.
	for _, c := range left.children {
		if c.x+c.w+testSpacing.Depth != left.x {
			t.Errorf("left child %+v not right-aligned against %+v", *c, *left)
		}
	}
}

func TestSplitSidesKeepsAssignedSides(t *testing.T) {
	a, b, c, d := sized(1, 1), sized(1, 1), sized(1, 1), sized(1, 1)
	a.side, b.side = Left, Left
	right, left := SplitSides([]Node{a, b, c, d})
	if len(right) != 2 || len(left) != 2 || right[0] != c || right[1] != d {
		t.Fatalf("right = %v, left = %v; want new children c, d balancing onto the right", right, left)
	}
	if a.side != Left || b.side != Left {
		t.Fatal("already-assigned sides changed")
	}
}
