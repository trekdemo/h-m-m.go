package main

import (
	"fmt"
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

var rootColor = lipgloss.Color("#E6EDF3")

var edgeColor = lipgloss.Color("#484f58")

// levelColor returns the color for a tree depth, so each level reads as a
// distinct band.
func levelColor(depth int) color.Color {
	return levelColors[depth%len(levelColors)]
}

// nodeStyle is the shared box style for a node of the given color.
func nodeStyle(c color.Color) lipgloss.Style {
	return lipgloss.NewStyle().
		Padding(0, 1).
		Foreground(c).
		Background(lipgloss.Darken(c, 0.8))
}

// selectedStyle derives the style drawn for the selected node.
func selectedStyle(base lipgloss.Style) lipgloss.Style {
	return base.Background(lipgloss.Yellow).Foreground(lipgloss.Black).Bold(true)
}

// hexColor formats c as "#rrggbb".
func hexColor(c color.Color) string {
	r, g, b, _ := c.RGBA()
	return fmt.Sprintf("#%02x%02x%02x", r>>8, g>>8, b>>8)
}

// colorOr parses hex ("#rrggbb") into a color, returning fallback when hex
// is empty or malformed.
func colorOr(hex string, fallback color.Color) color.Color {
	if len(hex) != 7 || hex[0] != '#' {
		return fallback
	}
	if _, err := fmt.Sscanf(hex[1:], "%06x", new(int)); err != nil {
		return fallback
	}
	return lipgloss.Color(hex)
}
