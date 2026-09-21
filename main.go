// Command bubbletea-exp renders a pannable canvas containing a tree of
// text boxes, laid out with a tree-specialized Sugiyama-style algorithm.
// Click and drag with the mouse to pan the canvas, like dragging a map.
package main

import (
	"fmt"
	"os"

	lipgloss "charm.land/lipgloss/v2"
	tea "charm.land/bubbletea/v2"
	uv "github.com/charmbracelet/ultraviolet"

	"github.com/trekdemo/bubbletea-exp/navigator"
)

// box is a piece of text rendered inside a rounded border, positioned
// somewhere on the virtual canvas.
type box struct {
	x, y          int // top-left position in canvas coordinates
	width, height int // rendered size, including the border
	text          string
	style         lipgloss.Style // normal border
	selStyle      lipgloss.Style // double border, drawn for the selected node
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

func (m model) Init() tea.Cmd { return nil }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "ctrl+c", "q", "esc":
			return m, tea.Quit
		case "left", "h":
			if n := m.currentNode(); n != nil {
				if target := m.nav.LeftOf(n); target != nil {
					m.setSelectedNode(target.(*node))
				}
			}
		case "right", "l":
			if n := m.currentNode(); n != nil {
				if target := m.nav.RightOf(n); target != nil {
					m.setSelectedNode(target.(*node))
				}
			}
		case "up", "k":
			if n := m.currentNode(); n != nil {
				if target := m.nav.Above(n); target != nil {
					m.setSelectedNode(target.(*node))
				}
			}
		case "down", "j":
			if n := m.currentNode(); n != nil {
				if target := m.nav.Below(n); target != nil {
					m.setSelectedNode(target.(*node))
				}
			}
		}

	case tea.WindowSizeMsg:
		m.viewportW = msg.Width
		m.viewportH = msg.Height
		// Center on the tree on resize
		// origin: the tree can extend arbitrarily far right and down.
		cx, cy := boxesCentroid(m.boxes)
		m.offsetX = cx - m.viewportW/2
		m.offsetY = cy - m.viewportH/2

	case tea.MouseClickMsg:
		if msg.Button == tea.MouseLeft {
			if hit := m.nodeAt(msg.X, msg.Y); hit >= 0 {
				m.selected = hit
			}
			m.dragging = true
			m.lastX, m.lastY = msg.X, msg.Y
		}

	case tea.MouseReleaseMsg:
		m.dragging = false

	case tea.MouseMotionMsg:
		if m.dragging {
			dx := msg.X - m.lastX
			dy := msg.Y - m.lastY
			m.offsetX -= dx
			m.offsetY -= dy
			m.lastX, m.lastY = msg.X, msg.Y
		}
	}

	return m, nil
}

func (m model) View() tea.View {
	v := tea.NewView("")
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion

	if m.viewportW == 0 || m.viewportH == 0 {
		return v
	}

	v.Content = m.buildCanvas().Render()
	return v
}

func (m *model) setSelectedNode(n *node) {
	if n == nil {
		return
	}

	m.selected = n.idx

	// Shift the viewport by the minimum amount needed so the selected box is
	// fully visible, so keyboard navigation never selects a node the user can't
	// see.
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

func (m model) currentNode() *node {
	if m.selected < 0 || m.selected >= len(m.nodes) {
		return nil
	}
	return m.nodes[m.selected]
}

// nodeAt returns the index of the box at viewport coordinates (x, y), or -1
// if no box covers that point.
func (m model) nodeAt(x, y int) int {
	x, y = x+m.offsetX, y+m.offsetY
	for i, b := range m.boxes {
		if x >= b.x && x < b.x+b.width && y >= b.y && y < b.y+b.height {
			return i
		}
	}
	return -1
}

// buildCanvas composites the connector edges and boxes currently visible in
// the viewport onto a lipgloss.Canvas sized to the viewport. Edges are
// drawn first, cell by cell, so box borders always render cleanly on top.
func (m model) buildCanvas() *lipgloss.Canvas {
	canvas := lipgloss.NewCanvas(m.viewportW, m.viewportH)

	// Accumulate every edge's directions per canvas cell first: when
	// sibling edges share a trunk column, a cell can have lines coming
	// from more than one direction (e.g. west, north, and south), which
	// needs a junction character like ┤ rather than whichever single
	// edge's rune happened to be drawn last.
	dirs := map[point]direction{}
	for _, e := range m.edges {
		for _, c := range edgeCells(e) {
			dirs[point{c.x, c.y}] |= c.dir
		}
	}
	for p, d := range dirs {
		sx, sy := p.x-m.offsetX, p.y-m.offsetY
		if sx < 0 || sx >= m.viewportW || sy < 0 || sy >= m.viewportH {
			continue
		}
		canvas.SetCell(sx, sy, &uv.Cell{Content: string(dirToRune(d)), Width: 1, Style: uv.Style{Fg: edgeColor}})
	}

	var layers []*lipgloss.Layer
	for i, b := range m.boxes {
		sx, sy := b.x-m.offsetX, b.y-m.offsetY
		if sx+b.width <= 0 || sx >= m.viewportW || sy+b.height <= 0 || sy >= m.viewportH {
			continue // fully off screen
		}
		style := b.style
		if i == m.selected {
			style = b.selStyle
		}
		layers = append(layers, lipgloss.NewLayer(style.Render(b.text)).X(sx).Y(sy))
	}
	canvas.Compose(lipgloss.NewCompositor(layers...))

	return canvas
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

func newModel() model {
	boxes, edges, nodes := buildDemoScene()
	nav := navigator.SpatialNavigator{Nodes: navNodes(nodes)}
	return model{
		boxes: boxes, edges: edges, nodes: nodes, nav: nav, selected: 0,
	}
}

func main() {
	p := tea.NewProgram(newModel())
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "error running program:", err)
		os.Exit(1)
	}
}
