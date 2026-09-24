package navigator

// TreeNavigator implements Navigator using tree structure for left/right
// (parent or first child, whichever lies in that direction, since a
// bidirectional layout grows some subtrees leftwards) and box position for
// up/down.
type TreeNavigator struct {
	Nodes []NavNode
}

func (t TreeNavigator) LeftOf(n NavNode) NavNode {
	return towards(n, -1)
}

func (t TreeNavigator) RightOf(n NavNode) NavNode {
	return towards(n, 1)
}

// towards returns n's parent if it lies in direction dir (-1 left, +1
// right) from n, otherwise n's first child that does, or nil if neither
// does. Only the root can have children on both sides.
func towards(n NavNode, dir int) NavNode {
	x := n.Bounds().X
	if p := n.Parent(); p != nil && (p.Bounds().X-x)*dir > 0 {
		return p
	}
	for _, c := range n.Children() {
		if (c.Bounds().X-x)*dir > 0 {
			return c
		}
	}
	return nil
}

// Above/Below return the nearest node in the same tree column (same x)
// above or below n, by box position. Once n runs out of siblings in that
// direction, this naturally continues into cousins, and further cousins,
// since it searches the whole column rather than just n's siblings.
func (t TreeNavigator) Above(n NavNode) NavNode {
	nb := n.Bounds()
	var best NavNode
	var bestY int
	for _, c := range t.Nodes {
		if c == n {
			continue
		}
		cb := c.Bounds()
		if cb.X != nb.X || cb.Y >= nb.Y {
			continue
		}
		if best == nil || cb.Y > bestY {
			best, bestY = c, cb.Y
		}
	}
	return best
}

func (t TreeNavigator) Below(n NavNode) NavNode {
	nb := n.Bounds()
	var best NavNode
	var bestY int
	for _, c := range t.Nodes {
		if c == n {
			continue
		}
		cb := c.Bounds()
		if cb.X != nb.X || cb.Y <= nb.Y {
			continue
		}
		if best == nil || cb.Y < bestY {
			best, bestY = c, cb.Y
		}
	}
	return best
}
