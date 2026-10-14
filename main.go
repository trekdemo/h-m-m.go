// Command bubbletea-exp renders a pannable canvas containing a tree of
// text boxes, laid out bidirectionally around the root (like Mermaid's
// tidy-tree mindmaps) with a tree-specialized Sugiyama-style algorithm.
// Click and drag with the mouse to pan the canvas, like dragging a map.
package main

import (
	"cmp"
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
	selected             *node // never nil: the tree always has its root
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
			m.updateComponentSizes()
			return m, cmd
		}

		switch {
		case key.Matches(msg, normalModeKeys.Quit):
			return m, tea.Quit
		case key.Matches(msg, normalModeKeys.EnterEdit):
			m.enterEditMode()
		case key.Matches(msg, normalModeKeys.AddSibling):
			m.addSibling()
			m.enterEditMode()
		case key.Matches(msg, normalModeKeys.AddChild):
			m.addChild()
			m.enterEditMode()
		case key.Matches(msg, normalModeKeys.Left):
			m.moveSelection(m.nav.LeftOf)
		case key.Matches(msg, normalModeKeys.Right):
			m.moveSelection(m.nav.RightOf)
		case key.Matches(msg, normalModeKeys.Up):
			m.moveSelection(m.nav.Above)
		case key.Matches(msg, normalModeKeys.Down):
			m.moveSelection(m.nav.Below)
		case key.Matches(msg, normalModeKeys.Delete):
			if m.selected != m.root {
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
		case key.Matches(msg, normalModeKeys.ToggleCollapse):
			m.toggleCollapsed()
		case key.Matches(msg, normalModeKeys.ToggleHelp):
			m.help.ShowAll = !m.help.ShowAll
		}
		m.updateComponentSizes()

	case tea.WindowSizeMsg:
		m.setViewportSize(msg.Width, msg.Height)
		m.centerOnTree()

	case tea.MouseClickMsg:
		if msg.Button == tea.MouseLeft {
			if hit := m.markerAt(msg.X, msg.Y); hit != nil {
				hit.collapsed = false
				layoutTree(m.root)
				m.setSelectedNode(hit)
			} else if hit := m.nodeAt(msg.X, msg.Y); hit != nil {
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
	m.updateComponentSizes()
}

func (m *model) updateComponentSizes() {
	m.help.SetWidth(m.windowW)
	reserved := lipgloss.Height(m.help.View(m.currentKeyMap()))
	reserved++ // Reserve a line for statusMsg
	m.viewportW = m.windowW
	m.viewportH = max(0, m.windowH-reserved)
}

func (m *model) centerOnTree() {
	cx, cy := BoxesCentroid(m.root.All())
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

// moveSelection selects the node that query (one of a navigator's directional
// lookups) finds from the current one, leaving the selection alone when
// there's nothing in that direction.
func (m *model) moveSelection(query func(navigator.NavNode) navigator.NavNode) {
	if target := query(m.selected); target != nil {
		m.setSelectedNode(target.(*node))
	}
}

// setSelectedNode selects n and scrolls it into view, first revealing it
// if it's hidden, so no caller (such as adding a child to a collapsed node)
// can leave the selection on a box that isn't drawn.
func (m *model) setSelectedNode(n *node) {
	if reveal(n) {
		layoutTree(m.root)
	}
	m.selected = n
	m.scrollIntoView(n)
}

// scrollIntoView shifts the viewport by the minimum amount needed for n's
// box to be visible
func (m *model) scrollIntoView(n *node) {
	m.offsetX = min(n.x, max(m.offsetX, n.x+n.Width()-m.viewportW))
	m.offsetY = min(n.y, max(m.offsetY, n.y+n.Height()-m.viewportH))
}

// enterEditMode copies the selected node's text into the edit buffer and
// switches to editMode, so subsequent key presses edit that buffer instead
// of driving navigation.
func (m *model) enterEditMode() {
	n := m.selected
	m.mode = editMode
	m.editText = []rune(n.text)
	m.editCurs = len(m.editText)
}

// addSibling inserts a new, empty-text sibling right after the selected
// node (or a child, when the root is selected), re-lays-out the tree and
// selects the new node.
func (m *model) addSibling() {
	sib := addSiblingAfter(m.selected)
	layoutTree(m.root)
	m.setSelectedNode(sib)
}

// addChild appends a new, empty-text child to the selected node,
// re-lays-out the tree and selects the new node.
func (m *model) addChild() {
	child := addChild(m.selected)
	layoutTree(m.root)
	m.setSelectedNode(child)
}

// moveSibling re-selects the moved node after the relayout so the viewport
// follows it to its new position.
func (m *model) moveSibling(dir int) {
	if moveSibling(m.selected, dir) {
		layoutTree(m.root)
		m.setSelectedNode(m.selected)
	}
}

// toggleCollapsed collapses or expands the selected node. It doesn't
// save: collapsed state is view state, so toggling it isn't an edit.
func (m *model) toggleCollapsed() {
	n := m.selected
	if !n.canCollapse() {
		return
	}
	n.collapsed = !n.collapsed
	layoutTree(m.root)
	// The relayout can move n, so re-select it to scroll it back on screen.
	m.setSelectedNode(n)
}

// removeCurrentNode deletes the selected node and selects its previous
// sibling, or its parent if it has none. The tree is re-laid-out before
// selecting, so the viewport scrolls to where the target ends up rather
// than where it was.
func (m *model) removeCurrentNode() {
	n := m.selected
	if n.parent == nil {
		return
	}
	target := cmp.Or(prevSibling(n), n.parent)
	removeNode(n)
	layoutTree(m.root)
	m.setSelectedNode(target)
}

// removeIfEmpty cleans up the empty leaf left behind when the user adds a
// node and leaves edit mode without typing. Only leaves are removed: a
// node with children merely had its text cleared, and taking its subtree
// with it would silently delete nodes the user never touched. The root is
// never removed either (see removeCurrentNode).
func (m *model) removeIfEmpty() {
	if m.selected.text == "" && len(m.selected.children) == 0 {
		m.removeCurrentNode()
	}
}

// applyEdit commits the edit buffer back onto the selected node's box,
// re-rendering it (and updating its cached width/height) with the new
// text, then re-lays-out the whole tree, since a changed box size can
// require every other box to shift. Called after every buffer mutation so
// the layout redraws live as the user types, not just once on commit.
func (m *model) applyEdit() {
	m.selected.setText(string(m.editText))
	layoutTree(m.root)
	m.setSelectedNode(m.selected)
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

// nodeAt returns the node whose box covers viewport coordinates (x, y),
// or nil if no box does.
func (m model) nodeAt(x, y int) *node {
	x, y = x+m.offsetX, y+m.offsetY
	for n := range m.root.All() {
		if n.Bounds().Contains(x, y) {
			return n
		}
	}
	return nil
}

// markerAt returns the collapsed node whose marker covers viewport
// coordinates (x, y), or nil if no marker does.
func (m model) markerAt(x, y int) *node {
	x, y = x+m.offsetX, y+m.offsetY
	for n := range m.root.All() {
		if n.collapsed && n.markerBounds().Contains(x, y) {
			return n
		}
	}
	return nil
}

// onScreen reports whether any part of r, in canvas coordinates, falls
// inside the viewport, so drawing can skip building layers for things
// that are entirely off screen.
func (m model) onScreen(r navigator.Rect) bool {
	sx, sy := r.X-m.offsetX, r.Y-m.offsetY
	return sx+r.Width > 0 && sx < m.viewportW && sy+r.Height > 0 && sy < m.viewportH
}

// buildCanvas composites the connector edges, boxes and collapsed nodes'
// markers currently visible in the viewport onto a lipgloss.Canvas sized to the viewport. Edges are
// drawn first, cell by cell, so box borders always render cleanly on top.
func (m model) buildCanvas() *lipgloss.Canvas {
	canvas := lipgloss.NewCanvas(m.viewportW, m.viewportH)

	// Accumulate every edge's directions per canvas cell first: when
	// sibling edges share a trunk column, a cell can have lines coming
	// from more than one direction (e.g. west, north, and south), which
	// needs a junction character like ┤ rather than whichever single
	// edge's rune happened to be drawn last.
	dirs := map[point]direction{}
	for n := range m.root.All() {
		for _, child := range n.visibleChildren() {
			for _, c := range edgeCells(connector(n, child)) {
				dirs[point{c.x, c.y}] |= c.dir
			}
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
	for n := range m.root.All() {
		// The box and its marker are culled separately: a marker sits
		// outside its box, so it can be on screen while the box isn't.
		if m.onScreen(n.Bounds()) {
			rendered := n.Render()
			if n == m.selected {
				rendered = n.RenderAsSelected()
				if m.mode == editMode {
					// The cursor lives in the edit buffer, not the box text.
					rendered = n.selStyle.Render(m.editingDisplayText())
				}
			}
			layers = append(layers, lipgloss.NewLayer(rendered).X(n.x-m.offsetX).Y(n.y-m.offsetY))
		}
		if mb := n.markerBounds(); n.collapsed && m.onScreen(mb) {
			marker := lipgloss.NewStyle().Foreground(n.color).Render(n.marker())
			layers = append(layers, lipgloss.NewLayer(marker).X(mb.X-m.offsetX).Y(mb.Y-m.offsetY))
		}
	}
	canvas.Compose(lipgloss.NewCompositor(layers...))

	return canvas
}

func newModel(spec storage.Node, savePath string) model {
	root := buildScene(spec)
	return model{
		root:     root,
		selected: root,
		nav:      navigator.SpatialNavigator{},
		help:     help.New(),
		savePath: savePath,
	}
}

// version is set at build time by the release pipeline via -ldflags.
var version = "dev"

func main() {
	path := defaultTreePath
	if len(os.Args) > 1 {
		if os.Args[1] == "--version" || os.Args[1] == "-v" {
			fmt.Println("hmm", version)
			return
		}
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
