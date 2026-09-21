package navigator

// TreeNavigator implements Navigator using tree structure for left/right
// (parent/first child) and box position for up/down.
type TreeNavigator struct {
	Nodes []Node
}

func (t TreeNavigator) LeftOf(n Node) Node {
	return n.Parent()
}

func (t TreeNavigator) RightOf(n Node) Node {
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
func (t TreeNavigator) Above(n Node) Node {
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

func (t TreeNavigator) Below(n Node) Node {
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
