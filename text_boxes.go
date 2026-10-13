package main

import (
	"iter"

	"hmm/navigator"

	lipgloss "charm.land/lipgloss/v2"
)

// textBox is a piece of text rendered, positioned somewhere on the virtual canvas.
type textBox struct {
	text     string
	rendered string // text in style, kept in step by setText
	x, y     int    // top-left position in canvas coordinates
	style    lipgloss.Style
	selStyle lipgloss.Style // drawn for the selected node
}

// setText is the only way to change the text, so the cached rendering
// never goes stale.
func (b *textBox) setText(text string) {
	b.text = text
	b.rendered = b.style.Render(text)
}

func (b textBox) Render() string { return b.rendered }

// RenderAsSelected isn't cached: only one box is selected at a time, so
// it costs a single render per frame.
func (b textBox) RenderAsSelected() string { return b.selStyle.Render(b.text) }

func (b textBox) Width() int  { return lipgloss.Width(b.Render()) }
func (b textBox) Height() int { return lipgloss.Height(b.Render()) }

// Bounds reports the box's position and rendered size in canvas coordinates.
func (b textBox) Bounds() navigator.Rect {
	return navigator.Rect{X: b.x, Y: b.y, Width: b.Width(), Height: b.Height()}
}

// BoxesCentroid returns the center point of the bounding box that encloses
// all nodes' boxes, so the initial viewport can be aimed at where the tree
// actually is.
func BoxesCentroid(nodes iter.Seq[*node]) (int, int) {
	first := true
	var minX, minY, maxX, maxY int
	for n := range nodes {
		r := n.Bounds()
		if first {
			minX, minY, maxX, maxY = r.X, r.Y, r.X+r.Width, r.Y+r.Height
			first = false
			continue
		}
		minX = min(minX, r.X)
		minY = min(minY, r.Y)
		maxX = max(maxX, r.X+r.Width)
		maxY = max(maxY, r.Y+r.Height)
	}
	return (minX + maxX) / 2, (minY + maxY) / 2
}
