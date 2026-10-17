package render

import (
	"testing"

	"hmm/geom"
)

func TestLineDrawsElbowsAndJunctions(t *testing.T) {
	c := NewCanvas(geom.Rect{Width: 10, Height: 5})
	// Two routes from one parent end that share a trunk column, one
	// jogging up and one jogging down.
	c.Line([]geom.Point{{X: 0, Y: 2}, {X: 3, Y: 2}, {X: 3, Y: 0}, {X: 6, Y: 0}})
	c.Line([]geom.Point{{X: 0, Y: 2}, {X: 3, Y: 2}, {X: 3, Y: 4}, {X: 6, Y: 4}})

	want := map[geom.Point]rune{
		{X: 0, Y: 2}: '─', // loose end
		{X: 3, Y: 0}: '╭', // elbow turning east at the top
		{X: 3, Y: 1}: '│',
		{X: 3, Y: 2}: '┤', // both routes' turns meet on the trunk
		{X: 3, Y: 4}: '╰',
		{X: 6, Y: 4}: '─',
	}
	for p, r := range want {
		if got := dirToRune(c.lines[p]); got != r {
			t.Errorf("cell %+v = %q, want %q", p, got, r)
		}
	}
}

func TestDrawSkipsBoxesOffScreen(t *testing.T) {
	c := NewCanvas(geom.Rect{X: 10, Y: 10, Width: 20, Height: 5})
	c.Draw(geom.Rect{X: 0, Y: 0, Width: 10, Height: 1}, "gone")
	c.Draw(geom.Rect{X: 29, Y: 14, Width: 10, Height: 1}, "kept")
	if len(c.layers) != 1 {
		t.Fatalf("got %d layers, want only the one overlapping the viewport", len(c.layers))
	}
}
