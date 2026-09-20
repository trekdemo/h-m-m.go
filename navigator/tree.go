package navigator

// Tree implements Navigator using tree structure for left/right
// (parent/first child) and box position for up/down.
type Tree struct {
	Nodes []Node
}

func (t Tree) LeftOf(n Node) Node {
	return n.Parent()
}

func (t Tree) RightOf(n Node) Node {
	children := n.Children()
	if len(children) == 0 {
		return nil
	}
	return children[0]
}

// Above/Below return the nearest node in the same tree column (same x)
// above or below n, by box position. Once n runs out of siblings in that
// direction, this naturally continues into cousins, and further cousins,
// since it searches the whole column rather than just n's siblings.
func (t Tree) Above(n Node) Node {
	nb := n.Bounds()
	var best Node
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

func (t Tree) Below(n Node) Node {
	nb := n.Bounds()
	var best Node
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
