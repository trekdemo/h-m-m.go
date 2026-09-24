package navigator

import "testing"

type testNode struct {
	r        Rect
	parent   NavNode
	children []NavNode
}

func (t *testNode) Bounds() Rect        { return t.r }
func (t *testNode) Parent() NavNode     { return t.parent }
func (t *testNode) Children() []NavNode { return t.children }

// adopt links child under parent.
func adopt(parent, child *testNode) {
	child.parent = parent
	parent.children = append(parent.children, child)
}

func TestSpatialNavigatorVariableWidthColumn(t *testing.T) {
	// A left-aligned column of siblings with very different widths, plus a
	// far-off box whose corner would sneak into a center-based cone.
	short := &testNode{r: Rect{X: 10, Y: 0, Width: 6, Height: 1}}
	long := &testNode{r: Rect{X: 10, Y: 2, Width: 40, Height: 1}}
	short2 := &testNode{r: Rect{X: 10, Y: 4, Width: 6, Height: 1}}
	far := &testNode{r: Rect{X: 45, Y: 8, Width: 10, Height: 1}}
	nav := SpatialNavigator{Nodes: []NavNode{short, long, short2, far}}

	if got := nav.Below(short); got != long {
		t.Errorf("Below(short) = %+v, want long", got)
	}
	if got := nav.Below(long); got != short2 {
		t.Errorf("Below(long) = %+v, want short2", got)
	}
	if got := nav.Above(short2); got != long {
		t.Errorf("Above(short2) = %+v, want long", got)
	}
	if got := nav.Above(long); got != short {
		t.Errorf("Above(long) = %+v, want short", got)
	}
}

func TestSpatialNavigatorPrefersStraightAhead(t *testing.T) {
	cur := &testNode{r: Rect{X: 0, Y: 5, Width: 10, Height: 1}}
	ahead := &testNode{r: Rect{X: 20, Y: 5, Width: 10, Height: 1}}
	diagonal := &testNode{r: Rect{X: 16, Y: 9, Width: 10, Height: 1}}
	nav := SpatialNavigator{Nodes: []NavNode{cur, ahead, diagonal}}

	if got := nav.RightOf(cur); got != ahead {
		t.Errorf("RightOf(cur) = %+v, want ahead", got)
	}
	if got := nav.LeftOf(ahead); got != cur {
		t.Errorf("LeftOf(ahead) = %+v, want cur", got)
	}
}

func TestSpatialNavigatorHorizontalSkipsXOverlap(t *testing.T) {
	// A short node, a long cousin below it that reaches past its right
	// edge, and the short node's child in the next column.
	cur := &testNode{r: Rect{X: 0, Y: 0, Width: 6, Height: 1}}
	cousin := &testNode{r: Rect{X: 0, Y: 2, Width: 30, Height: 1}}
	child := &testNode{r: Rect{X: 34, Y: 0, Width: 8, Height: 1}}
	adopt(cur, child)
	nav := SpatialNavigator{Nodes: []NavNode{cur, cousin, child}}

	if got := nav.RightOf(cur); got != child {
		t.Errorf("RightOf(cur) = %+v, want child", got)
	}
	if got := nav.LeftOf(child); got != cur {
		t.Errorf("LeftOf(child) = %+v, want cur", got)
	}
}

func TestSpatialNavigatorHorizontalPrefersParentOutsideCone(t *testing.T) {
	// The bottom child of a tall family: its parent is far above, outside
	// the cone, while a cousin sits straight to the left.
	parent := &testNode{r: Rect{X: 0, Y: 10, Width: 8, Height: 1}}
	cousin := &testNode{r: Rect{X: 0, Y: 20, Width: 8, Height: 1}}
	child := &testNode{r: Rect{X: 12, Y: 20, Width: 8, Height: 1}}
	adopt(parent, child)
	nav := SpatialNavigator{Nodes: []NavNode{parent, cousin, child}}

	if got := nav.LeftOf(child); got != parent {
		t.Errorf("LeftOf(child) = %+v, want parent", got)
	}
}
