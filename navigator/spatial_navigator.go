package navigator

import "math"

// SpatialNavigator implements Navigator using box geometry: each direction is a
// unit vector, and the nearest node within a 45-degree cone around that
// vector wins, so "right" only ever finds boxes that actually read as
// being to the right, not merely at a smaller x.
type SpatialNavigator struct {
	Nodes []NavNode
}

// nearest finds the node whose box center is closest to n's, among those
// that lie in the cone around (dirX, dirY) from n's center. A candidate's
// offset is decomposed into a component along the direction (primary) and
// a component perpendicular to it (lateral); the cone is exactly the set
// of points where the lateral offset doesn't exceed the primary one, i.e.
// within 45 degrees of the direction. Among cone members, plain Euclidean
// distance picks the closest.
func (s SpatialNavigator) nearest(n NavNode, dirX, dirY float64) NavNode {
	cx, cy := n.Bounds().Center()

	var best NavNode
	bestScore := math.Inf(1)
	for _, c := range s.Nodes {
		if c == n {
			continue
		}
		px, py := c.Bounds().Center()
		dx, dy := px-cx, py-cy

		primary := dx*dirX + dy*dirY
		if primary <= 0 {
			continue // not in this direction at all
		}
		lateral := dx*dirY - dy*dirX // perpendicular component
		if math.Abs(lateral) > primary {
			continue // outside the 45-degree cone
		}

		if score := math.Hypot(primary, lateral); score < bestScore {
			bestScore = score
			best = c
		}
	}
	return best
}

func (s SpatialNavigator) LeftOf(n NavNode) NavNode  { return s.nearest(n, -1, 0) }
func (s SpatialNavigator) RightOf(n NavNode) NavNode { return s.nearest(n, 1, 0) }
func (s SpatialNavigator) Above(n NavNode) NavNode   { return s.nearest(n, 0, -1) }
func (s SpatialNavigator) Below(n NavNode) NavNode   { return s.nearest(n, 0, 1) }
