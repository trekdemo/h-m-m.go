package main

import (
	"image/color"

	lipgloss "charm.land/lipgloss/v2"
)

var levelColors = []color.Color{
	lipgloss.Color("#FF6AC1"),
	lipgloss.Color("#4EA8DE"),
	lipgloss.Color("#FFB454"),
	lipgloss.Color("#7EE787"),
	lipgloss.Color("#B399FF"),
}

var edgeColor = lipgloss.Color("#484f58")

// levelColor returns the color for a tree depth, so each level reads as a
// distinct band.
func levelColor(depth int) color.Color {
	return levelColors[depth%len(levelColors)]
}

// nodeStyle is the shared box style for a node of the given color.
func nodeStyle(c color.Color) lipgloss.Style {
	return lipgloss.NewStyle().
		Width(boxTextWidth).
		Padding(0, 1).
		Foreground(c).
		Background(lipgloss.Darken(c, 0.8)).
		BorderForeground(c)
}

// selectedStyle derives the style drawn for the selected node.
func selectedStyle(base lipgloss.Style) lipgloss.Style {
	return base.Foreground(lipgloss.White).Bold(true)
}
