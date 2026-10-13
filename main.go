// Command bubbletea-exp renders a pannable canvas containing a tree of
// text boxes, laid out bidirectionally around the root (like Mermaid's
// tidy-tree mindmaps) with a tree-specialized Sugiyama-style algorithm.
// Click and drag with the mouse to pan the canvas, like dragging a map.
package main

import (
	"fmt"
	"os"
	"slices"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"
	uv "github.com/charmbracelet/ultraviolet"

	"hmm/navigator"
	"hmm/storage"
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
	boxes                []textBox
	edges                []edge
	nodes                []*node
	selected             int // index into boxes/nodes of the selected node
	nav                  navigator.Navigator

	savePath  string // OPML file that writeToFile writes the tree back to
	statusMsg string // last save result, shown below the canvas until overwritten

	mode     editorMode
	editText []rune // text buffer being edited, replaces the selected box's text on commit
	editCurs int    // cursor position within editText, in runes

	help help.Model

	dragging     bool
	lastX, lastY int
}

func (m model) Init() tea.Cmd { return nil }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		if m.mode == editMode {
			cmd := m.updateEditMode(msg)
			m.updateViewportSize()
			return m, cmd
		}

		switch {
		case key.Matches(msg, normalModeKeys.Quit):
			return m, tea.Quit
		case key.Matches(msg, normalModeKeys.EnterEdit):
			m.enterEditMode()
		case key.Matches(msg, normalModeKeys.AddSibling):
			if n := m.addSibling(); n != nil {
				m.enterEditMode()
			}
		case key.Matches(msg, normalModeKeys.AddChild):
			if n := m.addChild(); n != nil {
				m.enterEditMode()
			}
		case key.Matches(msg, normalModeKeys.Left):
			m.move(m.nav.LeftOf)
		case key.Matches(msg, normalModeKeys.Right):
			m.move(m.nav.RightOf)
		case key.Matches(msg, normalModeKeys.Up):
			m.move(m.nav.Above)
		case key.Matches(msg, normalModeKeys.Down):
			m.move(m.nav.Below)
		case key.Matches(msg, normalModeKeys.Delete):
			if n := m.currentNode(); n != nil && n != m.root {
				m.removeCurrentNode()
				m.writeToFile()
				return m, nil
			}
		case key.Matches(msg, normalModeKeys.MoveSibUp):
			m.moveSibling(-1)
			m.writeToFile()
			return m, nil
		case key.Matches(msg, normalModeKeys.MoveSibDown):
			m.moveSibling(1)
			m.writeToFile()
			return m, nil
		case key.Matches(msg, normalModeKeys.ToggleHelp):
			m.help.ShowAll = !m.help.ShowAll
		}
		m.updateViewportSize()

	case tea.WindowSizeMsg:
		m.setViewportSize(msg.Width, msg.Height)
		m.centerOnTree()

	case tea.MouseClickMsg:
		if msg.Button == tea.MouseLeft {
			if hit := m.nodeAt(msg.X, msg.Y); hit != nil {
				m.setSelectedNode(hit)
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

	v.Content = lipgloss.JoinVertical(
		lipgloss.Top,
		m.buildCanvas().Render(),
		m.statusMsg,
		m.help.View(m.currentKeyMap()),
	)
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

func (m *model) setViewportSize(width, height int) {
	m.windowW = width
	m.windowH = height
	m.updateViewportSize()
}

func (m *model) updateViewportSize() {
	m.help.SetWidth(m.windowW)
	reserved := lipgloss.Height(m.help.View(m.currentKeyMap()))
	reserved++ // Reserve a line for statusMsg
	m.viewportW = m.windowW
	m.viewportH = max(0, m.windowH-reserved)
}

func (m *model) centerOnTree() {
	cx, cy := BoxesCentroid(m.boxes)
	m.offsetX = cx - m.viewportW/2
	m.offsetY = cy - m.viewportH/2
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

// move selects the node that query (one of a navigator's directional
// lookups) finds from the current one, leaving the selection alone when
// there's nothing in that direction.
func (m *model) move(query func(navigator.NavNode) navigator.NavNode) {
	if n := m.currentNode(); n != nil {
		if target := query(n); target != nil {
			m.setSelectedNode(target.(*node))
		}
	}
}

func (m *model) setSelectedNode(n *node) {
	if n == nil {
		return
	}

	m.selected = n.idx

	// Shift the viewport by the minimum amount needed so the selected box is
	// fully visible, so keyboard navigation never selects a node the user can't
	// see.
	m.offsetX = scrollToShow(m.offsetX, m.viewportW, n.x, n.Width())
	m.offsetY = scrollToShow(m.offsetY, m.viewportH, n.y, n.Height())
}

// scrollToShow returns the viewport offset along one axis moved by the
// minimum amount needed for the span [pos, pos+size) to be visible in a
// viewport of length view starting at offset.
func scrollToShow(offset, view, pos, size int) int {
	switch {
	case pos < offset:
		return pos
	case pos+size > offset+view:
		return pos + size - view
	}
	return offset
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
	m.editText = []rune(n.text)
	m.editCurs = len(m.editText)
}

// addSibling inserts a new, empty-text sibling right after the selected
// node (or a child, when the root is selected) and re-lays-out the tree,
// returning the new node.
func (m *model) addSibling() *node {
	n := m.currentNode()
	if n == nil {
		return nil
	}
	sib := addSiblingAfter(n)

	m.boxes, m.edges, m.nodes = relayout(m.root)
	m.setSelectedNode(sib)
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
	m.setSelectedNode(child)

	return child
}

func (m *model) moveSibling(dir int) {
	if n := m.currentNode(); n != nil && moveSibling(n, dir) {
		m.boxes, m.edges, m.nodes = relayout(m.root)
		m.setSelectedNode(n)
	}
}

func (m *model) removeCurrentNode() {
	n := m.currentNode()
	if n == nil || n.parent == nil {
		return
	}

	target := prevSibling(n)
	if target == nil {
		target = n.parent
	}
	m.setSelectedNode(target)

	removeNode(n)

	m.boxes, m.edges, m.nodes = relayout(m.root)
}

// removeIfEmpty deletes the selected node from the tree if its text is
// empty, re-laying-out the tree and selecting its previous sibling, or its
// parent if it has none. It leaves the tree untouched if the node still has
// text or is the root (which has no parent to fall back to).
func (m *model) removeIfEmpty() {
	n := m.currentNode()
	if n == nil {
		return
	}
	if n.text == "" {
		m.removeCurrentNode()
	}
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
	n.setText(string(m.editText))
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
		m.writeToFile()
		return nil
	case key.Matches(msg, editModeKeys.Backspace):
		if m.editCurs > 0 {
			m.editText = slices.Delete(m.editText, m.editCurs-1, m.editCurs)
			m.editCurs--
			m.applyEdit()
		}
	case key.Matches(msg, editModeKeys.Delete):
		if m.editCurs < len(m.editText) {
			m.editText = slices.Delete(m.editText, m.editCurs, m.editCurs+1)
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
	m.editText = slices.Insert(m.editText, m.editCurs, runes...)
	m.editCurs += len(runes)
}

// editingDisplayText renders the edit buffer with a cursor glyph spliced
// in at the current cursor position, for display while editMode is active.
func (m model) editingDisplayText() string {
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
func (m model) nodeAt(x, y int) *node {
	x, y = x+m.offsetX, y+m.offsetY
	for i, b := range m.boxes {
		if x >= b.x && x < b.x+b.Width() && y >= b.y && y < b.y+b.Height() {
			return m.nodes[i]
		}
	}
	return nil
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
		if sx+b.Width() <= 0 || sx >= m.viewportW || sy+b.Height() <= 0 || sy >= m.viewportH {
			continue // fully off screen
		}
		rendered := b.Render()
		if i == m.selected {
			rendered = b.RenderAsSelected()
			if m.mode == editMode {
				// The cursor lives in the edit buffer, not the box text.
				rendered = b.selStyle.Render(m.editingDisplayText())
			}
		}
		layers = append(layers, lipgloss.NewLayer(rendered).X(sx).Y(sy))
	}
	canvas.Compose(lipgloss.NewCompositor(layers...))

	return canvas
}

func newModel(spec storage.Node, savePath string) model {
	root, boxes, edges, nodes := buildScene(spec)
	return model{
		root: root, boxes: boxes, edges: edges, nodes: nodes, selected: 0,
		nav:      navigator.SpatialNavigator{},
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
