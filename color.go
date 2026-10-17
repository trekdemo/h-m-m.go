package main

import (
	"fmt"
	"image/color"

	lipgloss "charm.land/lipgloss/v2"
)

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
