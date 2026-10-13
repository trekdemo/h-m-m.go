package main

import (
	"path/filepath"
	"slices"
	"testing"

	tea "charm.land/bubbletea/v2"

	"hmm/storage"
)

func TestModelSmoke(t *testing.T) {
	spec, err := storage.LoadOPML("testdata/demo_tree.opml")
	if err != nil {
		t.Fatalf("loading testdata/demo_tree.opml: %v", err)
	}
	m := newModel(spec, "testdata/demo_tree.opml")

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

	nodes := slices.Collect(m.root.All())
	for i, a := range nodes {
		for j, b := range nodes {
			if i == j {
				continue
			}
			if rectsOverlap(a.x, a.y, a.Width(), a.Height(), b.x, b.y, b.Width(), b.Height()) {
				t.Fatalf("boxes %d and %d overlap", i, j)
			}
		}
	}
}

func TestDeleteSelectsPreviousSiblingOrParent(t *testing.T) {
	m := newModel(storage.Node{Text: "root", Children: []storage.Node{
		{Text: "a"}, {Text: "b"},
	}}, filepath.Join(t.TempDir(), "tree.opml"))
	mm, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = mm.(model)
	a, b := m.root.children[0], m.root.children[1]

	m.setSelectedNode(b)
	m.removeCurrentNode()
	if m.selected != a {
		t.Fatalf("after deleting b, selected %q, want its previous sibling a", m.selected.text)
	}
	m.removeCurrentNode()
	if m.selected != m.root {
		t.Fatalf("after deleting a first child, selected %q, want the parent", m.selected.text)
	}
	m.removeCurrentNode()
	if m.selected != m.root || len(m.root.children) != 0 {
		t.Fatal("deleting the root should be refused")
	}
}

func TestLeavingEditModeOnlyRemovesEmptyLeaves(t *testing.T) {
	m := newModel(storage.Node{Text: "root", Children: []storage.Node{
		{Text: "parent", Children: []storage.Node{{Text: "child"}}},
	}}, filepath.Join(t.TempDir(), "tree.opml"))
	mm, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = mm.(model)
	parent := m.root.children[0]

	m.setSelectedNode(parent)
	m.enterEditMode()
	m.editText = nil
	m.applyEdit()
	mm, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	m = mm.(model)
	if len(m.root.children) != 1 || len(parent.children) != 1 {
		t.Fatal("clearing a node with children removed it and its subtree")
	}

	m.addChild()
	m.enterEditMode()
	mm, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	m = mm.(model)
	if len(parent.children) != 1 {
		t.Fatal("leaving an empty new leaf should remove it")
	}
}
