package main

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestModelSmoke(t *testing.T) {
	m := newModel()

	mm, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = mm.(model)

	mm, _ = m.Update(tea.MouseClickMsg{X: 10, Y: 10, Button: tea.MouseLeft})
	m = mm.(model)

	mm, _ = m.Update(tea.MouseMotionMsg{X: 15, Y: 12})
	m = mm.(model)
	if m.offsetX == 0 && m.offsetY == 0 {
		t.Fatalf("expected offset to change after drag")
	}

	mm, _ = m.Update(tea.MouseReleaseMsg{X: 15, Y: 12})
	m = mm.(model)
	if m.dragging {
		t.Fatalf("expected dragging to be false after release")
	}

	out := m.View()
	if len(out.Content) == 0 {
		t.Fatalf("expected non-empty view")
	}

	for i, a := range m.boxes {
		for j, b := range m.boxes {
			if i == j {
				continue
			}
			if rectsOverlap(a.x, a.y, a.width, a.height, b.x, b.y, b.width, b.height) {
				t.Fatalf("boxes %d and %d overlap", i, j)
			}
		}
	}
}
