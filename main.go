// Command bubbletea-exp renders a pannable canvas containing a tree of
// text boxes, laid out with a tree-specialized Sugiyama-style algorithm.
// Click and drag with the mouse to pan the canvas, like dragging a map.
package main

import (
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/trekdemo/bubbletea-exp/navigator"
)

// box is a piece of text rendered inside a rounded border, positioned
// somewhere on the virtual canvas.
type box struct {
	x, y          int // top-left position in canvas coordinates
	width, height int // rendered size, including the border
	lines         []string
	selLines      []string // same text, drawn with a double border for the selected node
	color         lipgloss.Color
}

type model struct {
	viewportW, viewportH int
	offsetX, offsetY     int // top-left of the viewport, in canvas coordinates
	boxes                []box
	edges                []edge
	nodes                []*node
	nav                  navigator.Navigator
	selected             int // index into boxes/nodes of the selected node

	dragging     bool
	lastX, lastY int
}

func newModel() model {
	boxes, edges, nodes := buildDemoScene()
	nav := navigator.Spatial{Nodes: navNodes(nodes)}
	return model{boxes: boxes, edges: edges, nodes: nodes, nav: nav, selected: 0}
}

// currentNode returns the currently selected node, or nil if the selection
// is out of range.
func (m model) currentNode() *node {
	if m.selected < 0 || m.selected >= len(m.nodes) {
		return nil
	}
	return m.nodes[m.selected]
}

// nodeAt returns the index of the box at canvas coordinates (x, y), or -1
// if no box covers that point.
func (m model) nodeAt(x, y int) int {
	for i, b := range m.boxes {
		if x >= b.x && x < b.x+b.width && y >= b.y && y < b.y+b.height {
			return i
		}
	}
	return -1
}

// ensureSelectedVisible shifts the viewport by the minimum amount needed
// so the selected box is fully visible, so keyboard navigation never
// selects a node the user can't see.
func (m *model) ensureSelectedVisible() {
	if m.selected < 0 || m.selected >= len(m.boxes) {
		return
	}
	b := m.boxes[m.selected]
	if b.x < m.offsetX {
		m.offsetX = b.x
	} else if b.x+b.width > m.offsetX+m.viewportW {
		m.offsetX = b.x + b.width - m.viewportW
	}
	if b.y < m.offsetY {
		m.offsetY = b.y
	} else if b.y+b.height > m.offsetY+m.viewportH {
		m.offsetY = b.y + b.height - m.viewportH
	}
}

func rectsOverlap(ax, ay, aw, ah, bx, by, bw, bh int) bool {
	return ax < bx+bw && ax+aw > bx && ay < by+bh && ay+ah > by
}

// boxesCentroid returns the center point of the bounding box that encloses
// all boxes, so the initial viewport can be aimed at where the tree
// actually is.
func boxesCentroid(boxes []box) (int, int) {
	if len(boxes) == 0 {
		return 0, 0
	}

	minX, minY := boxes[0].x, boxes[0].y
	maxX, maxY := boxes[0].x+boxes[0].width, boxes[0].y+boxes[0].height
	for _, b := range boxes[1:] {
		minX = min(minX, b.x)
		minY = min(minY, b.y)
		maxX = max(maxX, b.x+b.width)
		maxY = max(maxY, b.y+b.height)
	}
	return (minX + maxX) / 2, (minY + maxY) / 2
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q", "esc":
			return m, tea.Quit
		case "left", "h":
			if n := m.currentNode(); n != nil {
				if prev := m.nav.LeftOf(n); prev != nil {
					m.selected = prev.(*node).idx
				}
			}
		case "right", "l":
			if n := m.currentNode(); n != nil {
				if next := m.nav.RightOf(n); next != nil {
					m.selected = next.(*node).idx
				}
			}
		case "up", "k":
			if n := m.currentNode(); n != nil {
				if prev := m.nav.Above(n); prev != nil {
					m.selected = prev.(*node).idx
				}
			}
		case "down", "j":
			if n := m.currentNode(); n != nil {
				if next := m.nav.Below(n); next != nil {
					m.selected = next.(*node).idx
				}
			}
		}
		m.ensureSelectedVisible()

	case tea.WindowSizeMsg:
		firstResize := m.viewportW == 0 && m.viewportH == 0
		m.viewportW = msg.Width
		m.viewportH = msg.Height
		// Start the view centered on the tree itself, not the canvas
		// origin: the tree can extend arbitrarily far right and down.
		if firstResize {
			cx, cy := boxesCentroid(m.boxes)
			m.offsetX = cx - m.viewportW/2
			m.offsetY = cy - m.viewportH/2
		}

	case tea.MouseMsg:
		switch msg.Action {
		case tea.MouseActionPress:
			if msg.Button == tea.MouseButtonLeft {
				if hit := m.nodeAt(msg.X+m.offsetX, msg.Y+m.offsetY); hit >= 0 {
					m.selected = hit
				}
				m.dragging = true
				m.lastX, m.lastY = msg.X, msg.Y
			}
		case tea.MouseActionRelease:
			m.dragging = false
		case tea.MouseActionMotion:
			if m.dragging {
				dx := msg.X - m.lastX
				dy := msg.Y - m.lastY
				m.offsetX -= dx
				m.offsetY -= dy
				m.lastX, m.lastY = msg.X, msg.Y
			}
		}
	}

	return m, nil
}

func (m model) View() string {
	if m.viewportW == 0 || m.viewportH == 0 {
		return ""
	}

	grid, color := m.buildGrid()

	lines := make([]string, m.viewportH)
	for i := range grid {
		lines[i] = renderRow(grid[i], color[i])
	}
	return strings.Join(lines, "\n")
}

// buildGrid composites the connector edges and boxes currently visible in
// the viewport into a plain (ANSI-free) rune grid, alongside a parallel
// grid recording the color each cell should be drawn in ("" for none).
// Edges are drawn first so box borders always render cleanly on top.
func (m model) buildGrid() ([][]rune, [][]string) {
	grid := make([][]rune, m.viewportH)
	color := make([][]string, m.viewportH)
	for i := range grid {
		grid[i] = make([]rune, m.viewportW)
		color[i] = make([]string, m.viewportW)
		for j := range grid[i] {
			grid[i][j] = ' '
		}
	}

	set := func(x, y int, r rune, c lipgloss.Color) {
		sx, sy := x-m.offsetX, y-m.offsetY
		if sx < 0 || sx >= m.viewportW || sy < 0 || sy >= m.viewportH {
			return
		}
		grid[sy][sx] = r
		color[sy][sx] = string(c)
	}

	// Draw every edge's straight segments first, then every edge's corners:
	// sibling edges sharing a trunk column can have one's vertical stroke
	// pass through another's corner cell, so corners must always be drawn
	// last to avoid being overwritten by an unrelated straight segment.
	var allCorners []edgeCell
	for _, e := range m.edges {
		straights, corners := edgeCells(e)
		for _, p := range straights {
			set(p.x, p.y, p.ch, edgeColor)
		}
		allCorners = append(allCorners, corners...)
	}
	for _, p := range allCorners {
		set(p.x, p.y, p.ch, edgeColor)
	}

	for i, b := range m.boxes {
		sx, sy := b.x-m.offsetX, b.y-m.offsetY
		if sx+b.width <= 0 || sx >= m.viewportW || sy+b.height <= 0 || sy >= m.viewportH {
			continue // fully off screen
		}
		lines := b.lines
		if i == m.selected {
			lines = b.selLines
		}
		for li, line := range lines {
			col := b.x
			for _, r := range line {
				set(col, b.y+li, r, b.color)
				col++
			}
		}
	}

	return grid, color
}

// renderRow turns a row of runes and their per-cell colors into a string,
// colorizing contiguous same-color runs so ANSI escapes are never split
// across cells.
func renderRow(runes []rune, colors []string) string {
	var sb strings.Builder
	n := len(runes)
	for i := 0; i < n; {
		start := i
		c := colors[i]
		for i < n && colors[i] == c {
			i++
		}
		run := string(runes[start:i])
		if c == "" {
			sb.WriteString(run)
		} else {
			sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color(c)).Render(run))
		}
	}
	return sb.String()
}

func main() {
	p := tea.NewProgram(
		newModel(),
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "error running program:", err)
		os.Exit(1)
	}
}
