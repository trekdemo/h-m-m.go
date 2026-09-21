// Command bubbletea-exp renders a pannable canvas containing a tree of
// text boxes, laid out with a tree-specialized Sugiyama-style algorithm.
// Click and drag with the mouse to pan the canvas, like dragging a map.
package main

import (
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"
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

// editorMode is the current interaction mode, borrowed from modal editors
// like vi: normalMode drives navigation and selection, editMode redirects
// key presses into editing the selected box's text.
type editorMode int

const (
	normalMode editorMode = iota
	editMode
)

type model struct {
	viewportW, viewportH int
	offsetX, offsetY     int // top-left of the viewport, in canvas coordinates
	root                 *node
	boxes                []box
	edges                []edge
	nodes                []*node
	selected             int // index into boxes/nodes of the selected node

	mode     editorMode
	editText []rune // text buffer being edited, replaces the selected box's text on commit
	editCurs int    // cursor position within editText, in runes

	dragging     bool
	lastX, lastY int
}

func (m model) Init() tea.Cmd { return nil }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		if m.mode == editMode {
			m.updateEditMode(msg)
			break
		}

		nav := navigator.SpatialNavigator{Nodes: navNodes(m.nodes)}

		switch msg.String() {
		case "ctrl+c", "q", "esc":
			return m, tea.Quit
		case "i", "a":
			m.enterEditMode()
		case "left", "h":
			if n := m.currentNode(); n != nil {
				if target := nav.LeftOf(n); target != nil {
					m.setSelectedNode(target.(*node))
				}
			}
		case "right", "l":
			if n := m.currentNode(); n != nil {
				if target := nav.RightOf(n); target != nil {
					m.setSelectedNode(target.(*node))
				}
			}
		case "up", "k":
			if n := m.currentNode(); n != nil {
				if target := nav.Above(n); target != nil {
					m.setSelectedNode(target.(*node))
				}
			}
		case "down", "j":
			if n := m.currentNode(); n != nil {
				if target := nav.Below(n); target != nil {
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

// enterEditMode copies the selected node's text into the edit buffer and
// switches to editMode, so subsequent key presses edit that buffer instead
// of driving navigation.
func (m *model) enterEditMode() {
	n := m.currentNode()
	if n == nil {
		return
	}
	m.mode = editMode
	m.editText = []rune(n.box.text)
	m.editCurs = len(m.editText)
}

// applyEdit commits the edit buffer back onto the selected node's box,
// re-rendering it (and updating its cached width/height) with the new
// text, then re-lays-out the whole tree, since a changed box size can
// require every other box to shift. Called after every buffer mutation so
// the layout redraws live as the user types, not just once on commit.
func (m *model) applyEdit() {
	n := m.currentNode()
	if n == nil {
		return
	}
	n.box.text = string(m.editText)
	rendered := n.box.style.Render(n.box.text)
	n.box.width = lipgloss.Width(rendered)
	n.box.height = lipgloss.Height(rendered)

	m.boxes, m.edges, m.nodes = relayout(m.root)
	m.setSelectedNode(m.nodes[m.selected])
}

// updateEditMode handles a key press while in editMode: Esc returns to
// normalMode, navigation keys move the cursor, and any key that changes
// the buffer's text (typing, backspace, delete, enter) also re-applies the
// edit so the layout stays current.
func (m *model) updateEditMode(msg tea.KeyPressMsg) {
	switch msg.String() {
	case "esc":
		m.mode = normalMode
	case "backspace":
		if m.editCurs > 0 {
			m.editText = append(m.editText[:m.editCurs-1], m.editText[m.editCurs:]...)
			m.editCurs--
			m.applyEdit()
		}
	case "delete":
		if m.editCurs < len(m.editText) {
			m.editText = append(m.editText[:m.editCurs], m.editText[m.editCurs+1:]...)
			m.applyEdit()
		}
	case "left":
		if m.editCurs > 0 {
			m.editCurs--
		}
	case "right":
		if m.editCurs < len(m.editText) {
			m.editCurs++
		}
	case "home":
		m.editCurs = 0
	case "end":
		m.editCurs = len(m.editText)
	case "enter":
		m.insertAtCursor("\n")
		m.applyEdit()
	default:
		if msg.Text != "" {
			m.insertAtCursor(msg.Text)
			m.applyEdit()
		}
	}
}

// insertAtCursor splices s into the edit buffer at the cursor and advances
// the cursor past it.
func (m *model) insertAtCursor(s string) {
	runes := []rune(s)
	buf := make([]rune, 0, len(m.editText)+len(runes))
	buf = append(buf, m.editText[:m.editCurs]...)
	buf = append(buf, runes...)
	buf = append(buf, m.editText[m.editCurs:]...)
	m.editText = buf
	m.editCurs += len(runes)
}

// editingDisplayText renders the edit buffer with a cursor glyph spliced
// in at the current cursor position, for display while editMode is active.
func (m model) editingDisplayText() string {
	if m.editCurs >= len(m.editText) {
		return string(m.editText) + "▏"
	}
	return string(m.editText[:m.editCurs]) + "▏" + string(m.editText[m.editCurs:])
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
		text := b.text
		if i == m.selected {
			style = b.selStyle
			if m.mode == editMode {
				text = m.editingDisplayText()
			}
		}
		layers = append(layers, lipgloss.NewLayer(style.Render(text)).X(sx).Y(sy))
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
	root, boxes, edges, nodes := buildDemoScene()
	return model{
		root: root, boxes: boxes, edges: edges, nodes: nodes, selected: 0,
	}
}

func main() {
	p := tea.NewProgram(newModel())
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "error running program:", err)
		os.Exit(1)
	}
}
