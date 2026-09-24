package navigator

import (
	"math"
	"slices"
)

// SpatialNavigator implements Navigator using box geometry: each direction is a
// unit vector, and the nearest node within a 45-degree cone around that
// vector wins, so "right" only ever finds boxes that actually read as
// being to the right, not merely at a smaller x.
type SpatialNavigator struct {
	Nodes []NavNode
}

// nearest finds the closest node in direction (dirX, dirY) from n, measured
// edge to edge rather than center to center, so wide and narrow boxes next to
// each other are treated as neighbours regardless of where their centers are.
// Moving left or right skips any box that overlaps n on the x axis, and
// prefers n's parent or children over any other box in that direction (they
// needn't be inside the cone), so the move follows the tree rather than
// landing on a cousin whose box happens to reach closer.
//
// For each candidate, primary is how far its near edge lies beyond n's center
// along the direction (it must be positive: the whole box is on that side),
// and lateral is the gap between the two boxes' spans across the direction
// (zero when they overlap, e.g. siblings stacked in a column). Candidates
// whose lateral gap exceeds the primary distance fall outside a 45-degree
// cone and are ignored. Lateral offset weighs double, so a box straight
// ahead beats a slightly nearer one off to the side.
func (s SpatialNavigator) nearest(n NavNode, dirX, dirY float64) NavNode {
	cur := n.Bounds()
	cx, cy := cur.Center()

	var best, bestRelative NavNode
	bestScore, bestRelativeScore := math.Inf(1), math.Inf(1)
	for _, c := range s.Nodes {
		if c == n {
			continue
		}
		r := c.Bounds()

		var primary, lateral float64
		if dirX != 0 {
			if spanGap(cur.X, cur.Width, r.X, r.Width) == 0 {
				continue // overlaps n's column: a cousin, not a parent or child
			}
			near := float64(r.X) // left edge, when moving right
			if dirX < 0 {
				near = float64(r.X + r.Width)
			}
			primary = (near - cx) * dirX
			lateral = spanGap(cur.Y, cur.Height, r.Y, r.Height)
		} else {
			near := float64(r.Y) // top edge, when moving down
			if dirY < 0 {
				near = float64(r.Y + r.Height)
			}
			primary = (near - cy) * dirY
			lateral = spanGap(cur.X, cur.Width, r.X, r.Width)
		}

		if primary <= 0 {
			continue // not entirely in this direction
		}
		score := primary + 2*lateral

		if dirX != 0 && isRelative(n, c) {
			if score < bestRelativeScore {
				bestRelativeScore = score
				bestRelative = c
			}
			continue
		}

		if lateral > primary {
			continue // outside the 45-degree cone
		}
		if score < bestScore {
			bestScore = score
			best = c
		}
	}
	if bestRelative != nil {
		return bestRelative
	}
	return best
}

// isRelative reports whether c is n's parent or one of its children.
func isRelative(n, c NavNode) bool {
	if c == n.Parent() {
		return true
	}
	return slices.Contains(n.Children(), c)
}

// spanGap returns the distance between the 1D spans [a, a+aLen) and
// [b, b+bLen), or 0 if they overlap.
func spanGap(a, aLen, b, bLen int) float64 {
	switch {
	case b >= a+aLen:
		return float64(b - (a + aLen))
	case a >= b+bLen:
		return float64(a - (b + bLen))
	}
	return 0
}

func (s SpatialNavigator) LeftOf(n NavNode) NavNode  { return s.nearest(n, -1, 0) }
func (s SpatialNavigator) RightOf(n NavNode) NavNode { return s.nearest(n, 1, 0) }
func (s SpatialNavigator) Above(n NavNode) NavNode   { return s.nearest(n, 0, -1) }
func (s SpatialNavigator) Below(n NavNode) NavNode   { return s.nearest(n, 0, 1) }
