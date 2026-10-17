// Package render turns positioned boxes and connector routes into terminal
// output with lipgloss. It knows nothing about trees: callers decide what
// to draw and where, in canvas coordinates, and render decides how it
// looks.
package render

import (
	"fmt"
	"image/color"

	lipgloss "charm.land/lipgloss/v2"

	"hmm/geom"
)

var branchColors = []color.Color{
	lipgloss.Color("#FF6AC1"),
	lipgloss.Color("#4EA8DE"),
	lipgloss.Color("#FFB454"),
	lipgloss.Color("#7EE787"),
	lipgloss.Color("#B399FF"),
}

// RootColor is the root's color when the map file doesn't give one.
var RootColor = lipgloss.Color("#E6EDF3")

var edgeColor = lipgloss.Color("#484f58")

// BranchColor returns the palette color for the root's i-th top-level
// branch, cycling through the palette so neighbouring branches read as
// distinct.
func BranchColor(i int) color.Color {
	return branchColors[i%len(branchColors)]
}

// boxStyle is the shape every box shares, whatever its colors. Node and
// Selected both build on it, and Size measures with it, so selecting a box
// can never make it a different size from the one it was laid out at.
var boxStyle = lipgloss.NewStyle().Padding(0, 1)

var selectedStyle = boxStyle.
	Foreground(lipgloss.Black).
	Background(lipgloss.Yellow).
	Bold(true)

// Size is how many cells a box showing text covers.
func Size(text string) geom.Size {
	r := boxStyle.Render(text)
	return geom.Size{Width: lipgloss.Width(r), Height: lipgloss.Height(r)}
}

// Node draws text as an unselected box in color c.
func Node(text string, c color.Color) string {
	return boxStyle.
		Foreground(c).
		Background(lipgloss.Darken(c, 0.8)).
		Render(text)
}

// Selected draws text as the selected box, whatever its color.
func Selected(text string) string { return selectedStyle.Render(text) }

// Marker is the stub and count drawn beside a collapsed node, so it
// doesn't pass for a leaf: "─ +5" on the side its children would grow on,
// mirrored to "+5 ─" when they grow to the left.
func Marker(hidden int, left bool) string {
	count := fmt.Sprintf("+%d", hidden)
	if left {
		return count + " ─"
	}
	return "─ " + count
}

// Tint draws s in color c, for text that isn't a box, such as a marker.
func Tint(s string, c color.Color) string {
	return lipgloss.NewStyle().Foreground(c).Render(s)
}

// Width is how many cells wide s is drawn.
func Width(s string) int { return lipgloss.Width(s) }
