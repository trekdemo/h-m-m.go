package main

import (
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
