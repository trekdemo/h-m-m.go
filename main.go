// Command bubbletea-exp renders a pannable canvas of randomly placed,
// non-overlapping text boxes. Click and drag with the mouse to pan the
// canvas, similar to dragging a map around.
package main

import (
	"fmt"
	"math/rand"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	boxTextWidth = 20 // max characters per line inside a box
	canvasWidth  = 150
	canvasHeight = 60
	boxMargin    = 1 // minimum empty gap kept between boxes
)

var samplePhrases = []string{
	"Hello",
	"Bubble Tea",
	"Lip Gloss makes styling terminal UIs a breeze",
	"Canvas",
	"Drag me around like a map",
	"The quick brown fox jumps over the lazy dog",
	"Terminal UI",
	"Randomly placed",
	"Non overlapping boxes",
	"Rounded borders look nice",
	"Go is fun",
	"Panning works like Google Maps when you click and drag",
	"Short",
	"A slightly longer piece of sample text for variety",
	"Charm",
	"Word wrap at twenty characters maximum per line",
	"Box",
	"Another box with a medium amount of text inside it",
	"Tiny",
	"Yet another sample sentence to fill up space nicely",
}

var borderColors = []lipgloss.Color{
	lipgloss.Color("#FF6AC1"),
	lipgloss.Color("#4EA8DE"),
	lipgloss.Color("#FFB454"),
	lipgloss.Color("#7EE787"),
	lipgloss.Color("#B399FF"),
	lipgloss.Color("#FF6B6B"),
	lipgloss.Color("#4EEAFF"),
	lipgloss.Color("#F5E663"),
}

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

	dragging     bool
	lastX, lastY int
}

func newModel() model {
	return model{
		boxes: generateBoxes(),
	}
}

// generateBoxes lays out a set of boxes at random positions on the virtual
// canvas such that none of them overlap.
func generateBoxes() []box {
	var boxes []box

	// No color is applied at render time: the returned string must stay
	// free of ANSI escapes so it can be safely sliced rune-by-rune when
	// composited onto the canvas. Color is applied per box afterwards.
	plainStyle := lipgloss.NewStyle().
		Width(boxTextWidth).
		Padding(0, 1).
		Border(lipgloss.RoundedBorder())

	for _, phrase := range samplePhrases {
		lines := strings.Split(plainStyle.Render(phrase), "\n")
		h := len(lines)
		w := 0
		for _, l := range lines {
			if lipgloss.Width(l) > w {
				w = lipgloss.Width(l)
			}
		}

		if w >= canvasWidth || h >= canvasHeight {
			continue
		}

		const maxAttempts = 500
		for attempt := 0; attempt < maxAttempts; attempt++ {
			x := rand.Intn(canvasWidth - w)
			y := rand.Intn(canvasHeight - h)

			if !overlapsAny(boxes, x, y, w, h) {
				color := borderColors[len(boxes)%len(borderColors)]
				boxes = append(boxes, box{x: x, y: y, width: w, height: h, lines: lines, color: color})
				break
			}
		}
	}

	return boxes
}

func overlapsAny(boxes []box, x, y, w, h int) bool {
	for _, b := range boxes {
		if rectsOverlap(x-boxMargin, y-boxMargin, w+2*boxMargin, h+2*boxMargin, b.x, b.y, b.width, b.height) {
			return true
		}
	}
	return false
}

func rectsOverlap(ax, ay, aw, ah, bx, by, bw, bh int) bool {
	return ax < bx+bw && ax+aw > bx && ay < by+bh && ay+ah > by
}

// boxesCentroid returns the center point of the bounding box that encloses
// all boxes, so the initial viewport can be aimed at where the content
// actually is rather than the middle of the (mostly empty) canvas.
func boxesCentroid(boxes []box) (int, int) {
	if len(boxes) == 0 {
		return canvasWidth / 2, canvasHeight / 2
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
		// Start the view centered on the boxes themselves, not the empty
		// canvas: with boxes scattered sparsely, centering on the canvas
		// midpoint could easily land on a patch with nothing visible.
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

	grid, owner := m.buildGrid()

	lines := make([]string, m.viewportH)
	for i := range grid {
		lines[i] = renderRow(grid[i], owner[i], m.boxes)
	}
	return strings.Join(lines, "\n")
}

// buildGrid composites the boxes currently visible in the viewport into a
// plain (ANSI-free) rune grid, alongside a parallel grid recording which
// box (by index into m.boxes, or -1) owns each cell.
func (m model) buildGrid() ([][]rune, [][]int) {
	grid := make([][]rune, m.viewportH)
	owner := make([][]int, m.viewportH)
	for i := range grid {
		row := make([]rune, m.viewportW)
		own := make([]int, m.viewportW)
		for j := range row {
			row[j] = ' '
			own[j] = -1
		}
		grid[i] = row
		owner[i] = own
	}

	for bi, b := range m.boxes {
		sx := b.x - m.offsetX
		sy := b.y - m.offsetY
		if sx+b.width <= 0 || sx >= m.viewportW || sy+b.height <= 0 || sy >= m.viewportH {
			continue // fully off screen
		}

		for li, line := range b.lines {
			row := sy + li
			if row < 0 || row >= m.viewportH {
				continue
			}
			col := sx
			for _, r := range line {
				if col >= 0 && col < m.viewportW {
					grid[row][col] = r
					owner[row][col] = bi
				}
				col++
			}
		}
	}

	return grid, owner
}

// renderRow turns a row of runes and their owning box indices into a
// string, colorizing contiguous runs that belong to the same box so that
// ANSI escapes are never split across cells.
func renderRow(runes []rune, owner []int, boxes []box) string {
	var sb strings.Builder
	n := len(runes)
	for i := 0; i < n; {
		start := i
		o := owner[i]
		for i < n && owner[i] == o {
			i++
		}
		run := string(runes[start:i])
		if o == -1 {
			sb.WriteString(run)
		} else {
			sb.WriteString(lipgloss.NewStyle().Foreground(boxes[o].color).Render(run))
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
