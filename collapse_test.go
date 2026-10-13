package main

import (
	"math/rand"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"hmm/navigator"
	"hmm/storage"
)

// collapseSpec is a root with one branch that has a twig and a bud below it,
// plus a second, childless branch.
var collapseSpec = storage.Node{Text: "root", Children: []storage.Node{
	{Text: "branch", Children: []storage.Node{
		{Text: "twig", Children: []storage.Node{{Text: "bud"}}},
	}},
	{Text: "leaf"},
}}

func newCollapseModel(t *testing.T) model {
	t.Helper()
	m := newModel(collapseSpec, filepath.Join(t.TempDir(), "tree.opml"))
	mm, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	return mm.(model)
}

func pressSpace(m model) model {
	mm, _ := m.Update(tea.KeyPressMsg{Code: tea.KeySpace, Text: " "})
	return mm.(model)
}

func texts(nodes []*node) []string {
	var out []string
	for _, n := range nodes {
		out = append(out, n.text)
	}
	return out
}

func TestSpaceTogglesCollapsed(t *testing.T) {
	m := newCollapseModel(t)
	branch := m.root.children[0]
	m.setSelectedNode(branch)

	m = pressSpace(m)
	if !branch.collapsed {
		t.Fatal("Space on an expanded node should collapse it")
	}
	if got, want := texts(slices.Collect(m.root.All())), []string{"root", "branch", "leaf"}; !equalStrings(got, want) {
		t.Fatalf("visible nodes = %v, want %v", got, want)
	}

	m = pressSpace(m)
	if branch.collapsed {
		t.Fatal("Space on a collapsed node should expand it")
	}
	if got := len(slices.Collect(m.root.All())); got != 5 {
		t.Fatalf("visible nodes after expanding = %d, want 5", got)
	}
}

func TestSpaceIgnoresLeavesAndRoot(t *testing.T) {
	m := newCollapseModel(t)
	m = pressSpace(m) // root is selected
	if m.root.collapsed {
		t.Fatal("the root should never collapse")
	}
	leaf := m.root.children[1]
	m.setSelectedNode(leaf)
	m = pressSpace(m)
	if leaf.collapsed {
		t.Fatal("a leaf should never collapse")
	}
}

func TestNestedCollapseSurvivesAncestorToggle(t *testing.T) {
	m := newCollapseModel(t)
	branch := m.root.children[0]
	child := branch.children[0]

	m.setSelectedNode(child)
	m = pressSpace(m)
	m.setSelectedNode(branch)
	m = pressSpace(m)
	m = pressSpace(m)

	if !child.collapsed {
		t.Fatal("expanding an ancestor should leave a collapsed descendant collapsed")
	}
	if got, want := texts(slices.Collect(m.root.All())), []string{"root", "branch", "twig", "leaf"}; !equalStrings(got, want) {
		t.Fatalf("visible nodes = %v, want %v", got, want)
	}
}

func TestCollapsedNodeNavigatesLikeALeaf(t *testing.T) {
	m := newCollapseModel(t)
	branch := m.root.children[0]
	branch.collapsed = true
	layoutTree(m.root)

	if got := branch.Children(); got != nil {
		t.Fatalf("Children() of a collapsed node = %v, want none", got)
	}
	for _, query := range []func(navigator.NavNode) navigator.NavNode{
		m.nav.LeftOf, m.nav.RightOf, m.nav.Above, m.nav.Below,
	} {
		if got := query(branch); got != nil && got.(*node).parent == branch {
			t.Fatalf("navigation reached hidden node %q", got.(*node).text)
		}
	}
}

func TestCollapsedSubtreeIsStillSaved(t *testing.T) {
	m := newCollapseModel(t)
	m.setSelectedNode(m.root.children[0])
	m = pressSpace(m)
	m.writeToFile()

	data, err := os.ReadFile(m.savePath)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"twig", "bud"} {
		if !strings.Contains(string(data), `text="`+want+`"`) {
			t.Fatalf("saved OPML lost hidden node %q:\n%s", want, data)
		}
	}
	if strings.Contains(string(data), "collapsed") {
		t.Fatalf("collapsed state is view state and shouldn't be saved:\n%s", data)
	}
}

func TestCollapsedSubtreeIsNotLaidOutOrDrawn(t *testing.T) {
	m := newCollapseModel(t)
	branch := m.root.children[0]
	m.setSelectedNode(branch)
	m = pressSpace(m)

	if out := m.View().Content; strings.Contains(out, "twig") {
		t.Fatalf("hidden nodes were drawn:\n%s", out)
	}
}

func TestAddingAChildRevealsIt(t *testing.T) {
	m := newCollapseModel(t)
	branch := m.root.children[0]
	twig := branch.children[0]
	twig.collapsed = true
	m.setSelectedNode(branch)
	m = pressSpace(m)

	mm, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	m = mm.(model)

	if branch.collapsed {
		t.Fatal("adding a child to a collapsed node should expand it")
	}
	if !twig.collapsed {
		t.Fatal("revealing the new child should leave its collapsed siblings alone")
	}
	if !slices.Contains(slices.Collect(m.root.All()), m.selected) {
		t.Fatal("the new child is selected but not visible")
	}
}

func TestSelectingAHiddenNodeRevealsOnlyItsAncestors(t *testing.T) {
	m := newCollapseModel(t)
	branch := m.root.children[0]
	twig := branch.children[0]
	bud := twig.children[0]
	twig.collapsed = true
	branch.collapsed = true
	layoutTree(m.root)

	m.setSelectedNode(bud)
	if branch.collapsed || twig.collapsed {
		t.Fatal("selecting a hidden node should expand every collapsed ancestor")
	}
	if !slices.Contains(slices.Collect(m.root.All()), bud) {
		t.Fatal("the selected node is still hidden")
	}
}

func TestMarkerCountsAllHiddenDescendants(t *testing.T) {
	m := newCollapseModel(t)
	branch := m.root.children[0]
	branch.children[0].collapsed = true
	m.setSelectedNode(branch)
	m = pressSpace(m)

	if got, want := branch.marker(), "─ +2"; got != want {
		t.Fatalf("marker = %q, want %q", got, want)
	}
	if out := m.View().Content; !strings.Contains(out, "+2") {
		t.Fatalf("marker isn't drawn:\n%s", out)
	}
	if got := m.root.children[1].marker(); got != "" {
		t.Fatalf("expanded node has marker %q, want none", got)
	}
}

func TestMarkerPointsWhereChildrenGrow(t *testing.T) {
	root := buildTree(storage.Node{Text: "root", Children: []storage.Node{
		{Text: "right", Children: []storage.Node{{Text: "r"}}},
		{Text: "left", Children: []storage.Node{{Text: "l"}}},
	}})
	right, left := root.children[0], root.children[1]
	right.collapsed, left.collapsed = true, true
	layoutTree(root)

	if got := left.marker(); got != "+1 ─" {
		t.Fatalf("left-side marker = %q, want %q", got, "+1 ─")
	}
	if mb := right.markerBounds(); mb.X != right.x+right.Width() {
		t.Fatalf("right-side marker starts at x=%d, want just past the box at %d", mb.X, right.x+right.Width())
	}
	if mb := left.markerBounds(); mb.X+mb.Width != left.x {
		t.Fatalf("left-side marker ends at x=%d, want just before the box at %d", mb.X+mb.Width, left.x)
	}
}

// TestMarkersNeverOverlapBoxes collapses a scattering of nodes in randomly
// shaped trees and checks the layout left room for every marker: the tidy
// layout only keeps boxes apart, so a marker it didn't know about could
// have a cousin's subtree packed right on top of it.
func TestMarkersNeverOverlapBoxes(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	for trial := range 200 {
		root := buildTree(randomSpec(rng, 0))
		i := 0
		eachDescendant(root, func(n *node) {
			if n.canCollapse() && i%3 == trial%3 {
				n.collapsed = true
			}
			i++
		})
		layoutTree(root)

		nodes := slices.Collect(root.All())
		for _, a := range nodes {
			if !a.collapsed {
				continue
			}
			mb := a.markerBounds()
			for _, b := range nodes {
				if rectsOverlap(mb.X, mb.Y, mb.Width, mb.Height, b.x, b.y, b.Width(), b.Height()) {
					t.Fatalf("trial %d: marker of %q overlaps box %q", trial, a.text, b.text)
				}
			}
		}
	}
}

// randomSpec builds a small tree with varied text widths, so narrow
// collapsed nodes sit next to wide cousins with deep subtrees.
func randomSpec(rng *rand.Rand, depth int) storage.Node {
	n := storage.Node{Text: strings.Repeat("w", 1+rng.Intn(12))}
	if depth < 4 {
		for range rng.Intn(4) {
			n.Children = append(n.Children, randomSpec(rng, depth+1))
		}
	}
	return n
}

// TestWideMarkerIsLaidOutOfTheWay covers markers wider than colGap, which
// the gap every box already leaves past its edge can't absorb. The
// collapsed node's sibling has enough children that, centred on it, they
// reach up level with the marker.
func TestWideMarkerIsLaidOutOfTheWay(t *testing.T) {
	many := make([]storage.Node, 1000)
	fan := make([]storage.Node, 8)
	for i := range fan {
		fan[i] = storage.Node{Text: "fan"}
	}
	root := buildTree(storage.Node{Text: "root", Children: []storage.Node{
		{Text: "p", Children: []storage.Node{
			{Text: "a", Children: many},
			{Text: "b", Children: fan},
		}},
	}})
	a := root.children[0].children[0]
	a.collapsed = true
	layoutTree(root)

	mb := a.markerBounds()
	if mb.Width <= colGap {
		t.Fatalf("marker %q isn't wider than colGap, so this test proves nothing", a.marker())
	}
	for b := range root.All() {
		if rectsOverlap(mb.X, mb.Y, mb.Width, mb.Height, b.x, b.y, b.Width(), b.Height()) {
			t.Fatalf("marker %q overlaps box %q at %+v", a.marker(), b.text, b.Bounds())
		}
	}
}

func TestClickingMarkerExpandsAndSelects(t *testing.T) {
	m := newCollapseModel(t)
	branch := m.root.children[0]
	m.setSelectedNode(branch)
	m = pressSpace(m)
	m.setSelectedNode(m.root)

	mb := branch.markerBounds()
	x, y := mb.X+mb.Width-1-m.offsetX, mb.Y-m.offsetY
	mm, _ := m.Update(tea.MouseClickMsg{X: x, Y: y, Button: tea.MouseLeft})
	m = mm.(model)

	if branch.collapsed {
		t.Fatal("clicking the marker should expand the node")
	}
	if m.selected != branch {
		t.Fatalf("selected %q, want the expanded node", m.selected.text)
	}
}
