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
)

// box is a piece of text rendered inside a rounded border, positioned
// somewhere on the virtual canvas.
type box struct {
	x, y          int // top-left position in canvas coordinates
	width, height int // rendered size, including the border
	lines         []string
	color         lipgloss.Color
}

type model struct {
	viewportW, viewportH int
	offsetX, offsetY     int // top-left of the viewport, in canvas coordinates
	boxes                []box
	edges                []edge

	dragging     bool
	lastX, lastY int
}

func newModel() model {
	boxes, edges := buildDemoScene()
	return model{boxes: boxes, edges: edges}
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
		}

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

	for _, e := range m.edges {
		drawLine(e, set)
	}

	for _, b := range m.boxes {
		sx, sy := b.x-m.offsetX, b.y-m.offsetY
		if sx+b.width <= 0 || sx >= m.viewportW || sy+b.height <= 0 || sy >= m.viewportH {
			continue // fully off screen
		}
		for li, line := range b.lines {
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
