package navigator

// Tree implements Navigator using tree structure for left/right
// (parent or first child, whichever lies in that direction, since a
// bidirectional layout grows some subtrees leftwards) and box position for
// up/down.
type Tree[N Node[N]] struct{}

func (t Tree[N]) LeftOf(n N) N {
	return towards(n, -1)
}

func (t Tree[N]) RightOf(n N) N {
	return towards(n, 1)
}

// towards returns n's parent if it lies in direction dir (-1 left, +1
// right) from n, otherwise n's first child that does, or none (N's zero value) if neither
// does. Only the root can have children on both sides.
func towards[N Node[N]](n N, dir int) N {
	var none N
	x := n.Bounds().X
	if p := n.Parent(); p != none && (p.Bounds().X-x)*dir > 0 {
		return p
	}
	for _, c := range n.Children() {
		if (c.Bounds().X-x)*dir > 0 {
			return c
		}
	}
	return none
}

// Above/Below return the nearest node in the same tree column (same x)
// above or below n, by box position. Once n runs out of siblings in that
// direction, this naturally continues into cousins, and further cousins,
// since it searches the whole column rather than just n's siblings.
func (t Tree[N]) Above(n N) N {
	nb := n.Bounds()
	var none, best N
	var bestY int
	for c := range treeOf(n) {
		if c == n {
			continue
		}
		cb := c.Bounds()
		if cb.X != nb.X || cb.Y >= nb.Y {
			continue
		}
		if best == none || cb.Y > bestY {
			best, bestY = c, cb.Y
		}
	}
	return best
}

func (t Tree[N]) Below(n N) N {
	nb := n.Bounds()
	var none, best N
	var bestY int
	for c := range treeOf(n) {
		if c == n {
			continue
		}
		cb := c.Bounds()
		if cb.X != nb.X || cb.Y <= nb.Y {
			continue
		}
		if best == none || cb.Y < bestY {
			best, bestY = c, cb.Y
		}
	}
	return best
}
