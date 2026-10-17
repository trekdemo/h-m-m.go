package render

import (
	lipgloss "charm.land/lipgloss/v2"
	uv "github.com/charmbracelet/ultraviolet"

	"hmm/geom"
)

// Canvas collects what to draw in canvas coordinates and renders the part
// of it that falls inside a viewport. Lines are drawn first, cell by cell,
// so boxes drawn over them always render cleanly on top.
type Canvas struct {
	view   geom.Rect
	lines  map[geom.Point]direction
	layers []*lipgloss.Layer
}

// NewCanvas starts an empty canvas showing view, the viewport's position
// and size in canvas coordinates.
func NewCanvas(view geom.Rect) *Canvas {
	return &Canvas{view: view, lines: map[geom.Point]direction{}}
}

// Line draws a connector along route, a polyline of horizontal and
// vertical segments.
//
// Every line's directions are accumulated per cell first: when sibling
// connectors share a trunk column, a cell can have lines coming from more
// than one direction (e.g. west, north, and south), which needs a junction
// character like ┤ rather than whichever single line was drawn last.
func (c *Canvas) Line(route []geom.Point) {
	if len(route) < 2 {
		return
	}
	for i, p := range route {
		switch {
		case i == 0:
			c.lines[p] |= axis(p, route[1])
		case i == len(route)-1:
			c.lines[p] |= axis(route[i-1], p)
		default:
			// A turn only runs towards its two neighbours, so it draws
			// as an elbow rather than a cross.
			c.lines[p] |= towards(p, route[i-1]) | towards(p, route[i+1])
		}
		if i+1 < len(route) {
			next := route[i+1]
			d := axis(p, next)
			for q := step(p, next); q != next; q = step(q, next) {
				c.lines[q] |= d
			}
		}
	}
}

// Draw places content, already rendered, with its top-left corner at r's.
// It's skipped when r is entirely off screen, which saves building layers
// for most of a large map.
func (c *Canvas) Draw(r geom.Rect, content string) {
	if !r.Overlaps(c.view) {
		return
	}
	c.layers = append(c.layers, lipgloss.NewLayer(content).X(r.X-c.view.X).Y(r.Y-c.view.Y))
}

// Render composites everything drawn so far into the viewport.
func (c *Canvas) Render() string {
	canvas := lipgloss.NewCanvas(c.view.Width, c.view.Height)
	for p, d := range c.lines {
		if !c.view.Contains(p.X, p.Y) {
			continue
		}
		canvas.SetCell(p.X-c.view.X, p.Y-c.view.Y, &uv.Cell{Content: string(dirToRune(d)), Width: 1, Style: uv.Style{Fg: edgeColor}})
	}
	canvas.Compose(lipgloss.NewCompositor(c.layers...))
	return canvas.Render()
}

// direction is a bitmask of the cardinal directions a line passes through
// a cell in. Multiple lines can contribute directions to the same cell, so
// cells are accumulated by OR-ing directions together before being
// converted to a single box-drawing rune.
type direction uint8

const (
	dirNorth direction = 1 << iota
	dirSouth
	dirEast
	dirWest
)

// towards is the direction from p to q, which share a row or a column.
func towards(p, q geom.Point) direction {
	switch {
	case q.X > p.X:
		return dirEast
	case q.X < p.X:
		return dirWest
	case q.Y > p.Y:
		return dirSouth
	case q.Y < p.Y:
		return dirNorth
	}
	return 0
}

// axis is both directions along the segment from p to q, which is what a
// cell in the middle of it, or at a loose end, draws as.
func axis(p, q geom.Point) direction {
	if p.Y == q.Y {
		return dirEast | dirWest
	}
	return dirNorth | dirSouth
}

// step moves p one cell towards q along their shared row or column.
func step(p, q geom.Point) geom.Point {
	switch {
	case q.X > p.X:
		p.X++
	case q.X < p.X:
		p.X--
	case q.Y > p.Y:
		p.Y++
	case q.Y < p.Y:
		p.Y--
	}
	return p
}

// dirToRune uses rounded elbows to match the boxes' rounded borders, but
// has to fall back to square junctions where three or four lines meet,
// since Unicode has no rounded T or cross. A lone direction (a stub end)
// still draws as a straight line rather than disappearing.
func dirToRune(d direction) rune {
	switch d {
	case dirNorth, dirSouth, dirNorth | dirSouth:
		return '│'
	case dirEast, dirWest, dirEast | dirWest:
		return '─'
	case dirNorth | dirEast:
		return '╰'
	case dirNorth | dirWest:
		return '╯'
	case dirSouth | dirEast:
		return '╭'
	case dirSouth | dirWest:
		return '╮'
	case dirNorth | dirSouth | dirEast:
		return '├'
	case dirNorth | dirSouth | dirWest:
		return '┤'
	case dirNorth | dirEast | dirWest:
		return '┴'
	case dirSouth | dirEast | dirWest:
		return '┬'
	case dirNorth | dirSouth | dirEast | dirWest:
		return '┼'
	default:
		return ' '
	}
}
