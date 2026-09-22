// Command bubbletea-exp renders a pannable canvas containing a tree of
// text boxes, laid out with a tree-specialized Sugiyama-style algorithm.
// Click and drag with the mouse to pan the canvas, like dragging a map.
package main

import (
	"fmt"
	"os"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"
	uv "github.com/charmbracelet/ultraviolet"

	"github.com/trekdemo/bubbletea-exp/navigator"
	"github.com/trekdemo/bubbletea-exp/storage"
)

// defaultTreePath is loaded on startup when no file path is given on the
// command line.
const defaultTreePath = "docs/tutorial.opml"

// editorMode is the current interaction mode, borrowed from modal editors
// like vi: normalMode drives navigation and selection, editMode redirects
// key presses into editing the selected box's text.
type editorMode int

const (
	normalMode editorMode = iota
	editMode
)

type model struct {
	windowW, windowH     int // full terminal size, as reported by the last WindowSizeMsg
	viewportW, viewportH int // canvas area, i.e. the window minus the help view
	offsetX, offsetY     int // top-left of the viewport, in canvas coordinates
	root                 *node
	boxes                []box
	edges                []edge
	nodes                []*node
	selected             int // index into boxes/nodes of the selected node

	savePath  string // OPML file that a SaveMsg writes the tree back to
	statusMsg string // last save result, shown below the canvas until overwritten

	mode     editorMode
	editText []rune // text buffer being edited, replaces the selected box's text on commit
	editCurs int    // cursor position within editText, in runes

	help help.Model

	dragging     bool
	lastX, lastY int
}

func (m model) Init() tea.Cmd { return nil }

// SaveMsg signals that the tree should be written back to disk. Send it with
// [Save], following the same pattern as [tea.Quit] and [tea.QuitMsg].
type SaveMsg struct{}

// Save returns a command that produces a [SaveMsg].
func Save() tea.Msg {
	return SaveMsg{}
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case SaveMsg:
		m.writeToFile()

	case tea.KeyPressMsg:
		if m.mode == editMode {
			cmd := m.updateEditMode(msg)
			m.syncViewportSize()
			return m, cmd
		}

		nav := navigator.SpatialNavigator{Nodes: navNodes(m.nodes)}

		switch {
		case key.Matches(msg, normalModeKeys.Quit):
			return m, tea.Quit
		case key.Matches(msg, normalModeKeys.EnterEdit):
			m.enterEditMode()
		case key.Matches(msg, normalModeKeys.AddSibling):
			if n := m.addSibling(); n != nil {
				m.setSelectedNode(n)
				m.enterEditMode()
			}
		case key.Matches(msg, normalModeKeys.AddChild):
			if n := m.addChild(); n != nil {
				m.setSelectedNode(n)
				m.enterEditMode()
			}
		case key.Matches(msg, normalModeKeys.Left):
			if n := m.currentNode(); n != nil {
				if target := nav.LeftOf(n); target != nil {
					m.setSelectedNode(target.(*node))
				}
			}
		case key.Matches(msg, normalModeKeys.Right):
			if n := m.currentNode(); n != nil {
				if target := nav.RightOf(n); target != nil {
					m.setSelectedNode(target.(*node))
				}
			}
		case key.Matches(msg, normalModeKeys.Up):
			if n := m.currentNode(); n != nil {
				if target := nav.Above(n); target != nil {
					m.setSelectedNode(target.(*node))
				}
			}
		case key.Matches(msg, normalModeKeys.Down):
			if n := m.currentNode(); n != nil {
				if target := nav.Below(n); target != nil {
					m.setSelectedNode(target.(*node))
				}
			}
		case key.Matches(msg, normalModeKeys.MoveSibUp):
			if n := m.currentNode(); n != nil && moveSibling(n, -1) {
				m.boxes, m.edges, m.nodes = relayout(m.root)
				m.setSelectedNode(n)
			}
		case key.Matches(msg, normalModeKeys.MoveSibDown):
			if n := m.currentNode(); n != nil && moveSibling(n, 1) {
				m.boxes, m.edges, m.nodes = relayout(m.root)
				m.setSelectedNode(n)
			}
		case key.Matches(msg, normalModeKeys.ToggleHelp):
			m.help.ShowAll = !m.help.ShowAll
		}
		m.syncViewportSize()

	case tea.WindowSizeMsg:
		m.windowW = msg.Width
		m.windowH = msg.Height
		m.syncViewportSize()
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
	if m.statusMsg != "" {
		v.Content += "\n" + m.statusMsg
	}
	if helpView := m.help.View(m.currentKeyMap()); helpView != "" {
		v.Content += "\n" + helpView
	}
	return v
}

// currentKeyMap returns the keybindings active for the current mode, so the
// help view always reflects what a key press will actually do.
func (m model) currentKeyMap() help.KeyMap {
	if m.mode == editMode {
		return editModeKeys
	}
	return normalModeKeys
}

// syncViewportSize recomputes the canvas viewport size from the last known
// window size and the current help view's height, so the help view never
// overlaps the canvas: it fully occupies the bottom of the screen and the
// canvas shrinks to fit above it.
func (m *model) syncViewportSize() {
	m.help.SetWidth(m.windowW)
	reserved := lipgloss.Height(m.help.View(m.currentKeyMap()))
	if m.statusMsg != "" {
		reserved++
	}
	m.viewportW = m.windowW
	m.viewportH = max(0, m.windowH-reserved)
}

// writeToFile writes the tree back to m.savePath as OPML, recording the outcome in
// m.statusMsg so the view can show it.
func (m *model) writeToFile() {
	if err := storage.SaveOPML(m.savePath, m.root); err != nil {
		m.statusMsg = "save failed: " + err.Error()
		return
	}
	m.statusMsg = "saved to " + m.savePath
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

// addSibling inserts a new, empty-text sibling right after the selected
// node and re-lays-out the tree, returning the new node (or nil if the
// selected node is the root, which has no siblings).
func (m *model) addSibling() *node {
	n := m.currentNode()
	if n == nil {
		return nil
	}
	sib := addSiblingAfter(n)
	if sib == nil {
		return nil
	}

	m.boxes, m.edges, m.nodes = relayout(m.root)
	return sib
}

// addChild appends a new, empty-text child to the selected node and
// re-lays-out the tree, returning the new node.
func (m *model) addChild() *node {
	n := m.currentNode()
	if n == nil {
		return nil
	}
	child := addChild(n)

	m.boxes, m.edges, m.nodes = relayout(m.root)
	return child
}

// removeIfEmpty deletes the selected node from the tree if its text is
// empty, re-laying-out the tree and selecting its previous sibling, or its
// parent if it has none. It leaves the tree untouched if the node still has
// text or is the root (which has no parent to fall back to).
func (m *model) removeIfEmpty() {
	n := m.currentNode()
	if n == nil || n.box.text != "" || n.parent == nil {
		return
	}
	target := prevSibling(n)
	if target == nil {
		target = n.parent
	}
	removeNode(n)

	m.boxes, m.edges, m.nodes = relayout(m.root)
	m.setSelectedNode(target)
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
// normalMode and triggers a save, navigation keys move the cursor, and any
// key that changes the buffer's text (typing, backspace, delete, enter)
// also re-applies the edit so the layout stays current.
func (m *model) updateEditMode(msg tea.KeyPressMsg) tea.Cmd {
	switch {
	case key.Matches(msg, editModeKeys.Exit):
		m.mode = normalMode
		m.removeIfEmpty()
		return Save
	case key.Matches(msg, editModeKeys.Backspace):
		if m.editCurs > 0 {
			m.editText = append(m.editText[:m.editCurs-1], m.editText[m.editCurs:]...)
			m.editCurs--
			m.applyEdit()
		}
	case key.Matches(msg, editModeKeys.Delete):
		if m.editCurs < len(m.editText) {
			m.editText = append(m.editText[:m.editCurs], m.editText[m.editCurs+1:]...)
			m.applyEdit()
		}
	case key.Matches(msg, editModeKeys.Left):
		if m.editCurs > 0 {
			m.editCurs--
		}
	case key.Matches(msg, editModeKeys.Right):
		if m.editCurs < len(m.editText) {
			m.editCurs++
		}
	case key.Matches(msg, editModeKeys.Home):
		m.editCurs = 0
	case key.Matches(msg, editModeKeys.End):
		m.editCurs = len(m.editText)
	case key.Matches(msg, editModeKeys.Enter):
		m.insertAtCursor("\n")
		m.applyEdit()
	default:
		if msg.Text != "" {
			m.insertAtCursor(msg.Text)
			m.applyEdit()
		}
	}
	return nil
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

func newModel(spec storage.Node, savePath string) model {
	root, boxes, edges, nodes := buildScene(spec)
	return model{
		root: root, boxes: boxes, edges: edges, nodes: nodes, selected: 0,
		help:     help.New(),
		savePath: savePath,
	}
}

func main() {
	path := defaultTreePath
	if len(os.Args) > 1 {
		path = os.Args[1]
	}

	var nodeTree storage.Node
	if _, statErr := os.Stat(path); os.IsNotExist(statErr) {
		nodeTree = storage.Node{}
	} else {
		var err error
		nodeTree, err = storage.LoadOPML(path)
		if err != nil {
			fmt.Fprintln(os.Stderr, "error loading opml:", err)
			os.Exit(1)
		}
	}

	p := tea.NewProgram(newModel(nodeTree, path))
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "error running program:", err)
		os.Exit(1)
	}
}
