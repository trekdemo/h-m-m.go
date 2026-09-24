package layout

import "math"

// This file ports the non-layered tidy tree algorithm from A. van der
// Ploeg, "Drawing Non-layered Tidy Trees in Linear Time" (Software:
// Practice and Experience, 2014), the same algorithm Mermaid's tidy-tree
// layout runs on (via the non-layered-tidy-tree-layout package).
//
// It works in its own top-down frame: y is depth (how far a node is from
// the root) and x is breadth (where it sits among its siblings). Nodes can
// have any size, and a child sits directly below its own parent rather
// than in a shared row per depth, so a big node only pushes its own
// descendants further away. layoutSide rotates the result so depth runs
// horizontally.

// tidyTree is one node being laid out.
type tidyTree struct {
	w, h float64 // breadth and depth size, including the gaps around the box
	y    float64 // depth coordinate, fixed before layout
	x    float64 // breadth coordinate, the layout's result
	c    []*tidyTree
	n    Node // the node this lays out, nil for a virtual root
	boxW int  // n's own width, without the gap, for mirroring the left side

	prelim, mod, shift, change float64
	tl, tr                     *tidyTree // left and right threads
	el, er                     *tidyTree // extreme left and right nodes
	msel, mser                 float64   // sum of modifiers at el and er
}

// tidyLayout assigns x to every node of t: siblings never overlap (keeping
// each node's full w, gaps included, clear of its neighbors) and every
// parent is centered over the span of its children. It runs in O(n).
func tidyLayout(t *tidyTree) {
	firstWalk(t)
	secondWalk(t, 0)
}

func (t *tidyTree) bottom() float64 { return t.y + t.h }

// firstWalk sets each node's prelim position relative to its parent,
// post-order, pushing each child subtree right of its left siblings just
// far enough to clear them at every depth they share.
func firstWalk(t *tidyTree) {
	if len(t.c) == 0 {
		setExtremes(t)
		return
	}
	firstWalk(t.c[0])
	ih := updateIYL(t.c[0].el.bottom(), 0, nil)
	for i := 1; i < len(t.c); i++ {
		firstWalk(t.c[i])
		minY := t.c[i].er.bottom()
		separate(t, i, ih)
		ih = updateIYL(minY, i, ih)
	}
	positionRoot(t)
	setExtremes(t)
}

func setExtremes(t *tidyTree) {
	if len(t.c) == 0 {
		t.el, t.er = t, t
		t.msel, t.mser = 0, 0
		return
	}
	first, last := t.c[0], t.c[len(t.c)-1]
	t.el, t.msel = first.el, first.msel
	t.er, t.mser = last.er, last.mser
}

// separate walks down the right contour of t's children before i and the
// left contour of child i together, moving child i right wherever the two
// overlap, then threads the shorter contour onto the longer one.
func separate(t *tidyTree, i int, ih *iyl) {
	// Right contour node of the left siblings and its sum of modifiers.
	sr := t.c[i-1]
	mssr := sr.mod
	// Left contour node of the current subtree and its sum of modifiers.
	cl := t.c[i]
	mscl := cl.mod
	for sr != nil && cl != nil {
		if sr.bottom() > ih.lowY {
			ih = ih.next
		}
		// How far the left side of cl is left of the right side of sr.
		if dist := mssr + sr.prelim + sr.w - (mscl + cl.prelim); dist > 0 {
			mscl += dist
			moveSubtree(t, i, ih.index, dist)
		}
		sy, cy := sr.bottom(), cl.bottom()
		if sy <= cy {
			sr = nextRightContour(sr)
			if sr != nil {
				mssr += sr.mod
			}
		}
		if sy >= cy {
			cl = nextLeftContour(cl)
			if cl != nil {
				mscl += cl.mod
			}
		}
	}
	// The current subtree is deeper than its left siblings, or the other
	// way around: thread the shallower one's contour onto the deeper one.
	if sr == nil && cl != nil {
		setLeftThread(t, i, cl, mscl)
	} else if sr != nil && cl == nil {
		setRightThread(t, i, sr, mssr)
	}
}

// moveSubtree moves child i right by dist, and spreads the same move
// evenly across the siblings between it and si (the sibling it collided
// with), so small subtrees between two big ones end up evenly spaced
// rather than bunched against one side.
func moveSubtree(t *tidyTree, i, si int, dist float64) {
	t.c[i].mod += dist
	t.c[i].msel += dist
	t.c[i].mser += dist
	if si != i-1 {
		nr := float64(i - si)
		t.c[si+1].shift += dist / nr
		t.c[i].shift -= dist / nr
		t.c[i].change -= dist - dist/nr
	}
}

func nextLeftContour(t *tidyTree) *tidyTree {
	if len(t.c) == 0 {
		return t.tl
	}
	return t.c[0]
}

func nextRightContour(t *tidyTree) *tidyTree {
	if len(t.c) == 0 {
		return t.tr
	}
	return t.c[len(t.c)-1]
}

func setLeftThread(t *tidyTree, i int, cl *tidyTree, modsumcl float64) {
	li := t.c[0].el
	li.tl = cl
	// Adjust mod so the modifier sum along the thread is right, and prelim
	// so li itself doesn't move.
	diff := (modsumcl - cl.mod) - t.c[0].msel
	li.mod += diff
	li.prelim -= diff
	t.c[0].el, t.c[0].msel = t.c[i].el, t.c[i].msel
}

func setRightThread(t *tidyTree, i int, sr *tidyTree, modsumsr float64) {
	ri := t.c[i].er
	ri.tr = sr
	diff := (modsumsr - sr.mod) - t.c[i].mser
	ri.mod += diff
	ri.prelim -= diff
	t.c[i].er, t.c[i].mser = t.c[i-1].er, t.c[i-1].mser
}

// positionRoot centers t over the span from its first child's near edge
// to its last child's far edge.
func positionRoot(t *tidyTree) {
	first, last := t.c[0], t.c[len(t.c)-1]
	t.prelim = (first.prelim+first.mod+last.mod+last.prelim+last.w)/2 - t.w/2
}

// secondWalk turns the relative prelim/mod positions into absolute x,
// pre-order, applying the spacing moveSubtree spread across siblings.
func secondWalk(t *tidyTree, modsum float64) {
	modsum += t.mod
	t.x = t.prelim + modsum
	addChildSpacing(t)
	for _, c := range t.c {
		secondWalk(c, modsum)
	}
}

func addChildSpacing(t *tidyTree) {
	var d, modsumdelta float64
	for _, c := range t.c {
		d += c.shift
		modsumdelta += d + c.change
		c.mod += modsumdelta
	}
}

// iyl is a linked list of the left siblings that still show on the
// combined right contour, with the lowest depth each one reaches, so
// separate knows which sibling a collision is with.
type iyl struct {
	lowY  float64
	index int
	next  *iyl
}

func updateIYL(minY float64, i int, ih *iyl) *iyl {
	// Drop siblings hidden behind the new subtree.
	for ih != nil && minY >= ih.lowY {
		ih = ih.next
	}
	return &iyl{lowY: minY, index: i, next: ih}
}

// floorInt rounds a layout coordinate down to a cell. Flooring (rather
// than rounding) every coordinate keeps integer-sized gaps intact: if
// b >= a+k for an integer k, then floor(b) >= floor(a)+k.
func floorInt(v float64) int { return int(math.Floor(v)) }
