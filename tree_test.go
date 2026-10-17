package main

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"hmm/geom"
	"hmm/layout"
	"hmm/navigator"
	"hmm/storage"
)

func TestBuildDemoSceneLayout(t *testing.T) {
	demoTree, err := storage.LoadOPML("testdata/demo_tree.opml")
	if err != nil {
		t.Fatalf("loading testdata/demo_tree.opml: %v", err)
	}
	root := buildScene(demoTree)
	nodes := slices.Collect(root.All())
	var routes [][]geom.Point
	for _, n := range nodes {
		for _, c := range n.children {
			routes = append(routes, mindmap.Route(n, c))
		}
	}

	var countNodes func(spec storage.Node) int
	countNodes = func(spec storage.Node) int {
		n := 1
		for _, c := range spec.Children {
			n += countNodes(c)
		}
		return n
	}
	wantNodes := countNodes(demoTree)

	if len(nodes) != wantNodes {
		t.Fatalf("got %d nodes, want %d", len(nodes), wantNodes)
	}
	if len(routes) != wantNodes-1 {
		t.Fatalf("got %d connectors, want %d (a tree has n-1 edges)", len(routes), wantNodes-1)
	}

	for i, a := range nodes {
		if a.size.Width <= 0 || a.size.Height <= 0 {
			t.Fatalf("box %d has non-positive size %dx%d", i, a.size.Width, a.size.Height)
		}
		for j, b := range nodes {
			if i == j {
				continue
			}
			if a.Bounds().Overlaps(b.Bounds()) {
				t.Fatalf("boxes %d and %d overlap: %+v vs %+v", i, j, a.Bounds(), b.Bounds())
			}
		}
	}

	// render draws connectors as runs of horizontal and vertical lines.
	for i, r := range routes {
		for j := 1; j < len(r); j++ {
			if r[j-1].X != r[j].X && r[j-1].Y != r[j].Y {
				t.Fatalf("connector %d has a diagonal segment: %+v", i, r)
			}
		}
	}
}

// eachDescendant calls f on n and every node below it.
func eachDescendant(n *node, f func(*node)) {
	f(n)
	for _, c := range n.children {
		eachDescendant(c, f)
	}
}

func TestLayoutIsBidirectional(t *testing.T) {
	demoTree, err := storage.LoadOPML("testdata/demo_tree.opml")
	if err != nil {
		t.Fatalf("loading testdata/demo_tree.opml: %v", err)
	}
	root := buildScene(demoTree)

	if root.x != 0 || root.y != 0 {
		t.Fatalf("root at (%d, %d), want (0, 0)", root.x, root.y)
	}
	if len(root.children) < 2 {
		t.Fatalf("demo tree needs at least 2 root children, has %d", len(root.children))
	}

	rootCenter := root.y + root.size.Height/2
	for dir, wantRight := range []bool{true, false} {
		var firstLevel []*node
		for i, c := range root.children {
			if i%2 != dir {
				continue
			}
			firstLevel = append(firstLevel, c)
			eachDescendant(c, func(d *node) {
				if wantRight && d.x < root.x+root.size.Width+layout.ColGap {
					t.Errorf("%q should be right of the root, got x=%d", d.text, d.x)
				}
				if !wantRight && d.x+d.size.Width > root.x-layout.ColGap {
					t.Errorf("%q should be left of the root, got x=%d", d.text, d.x)
				}
			})
		}

		first, last := firstLevel[0], firstLevel[len(firstLevel)-1]
		mid := (first.y + last.y + last.size.Height) / 2
		if d := mid - rootCenter; d < -1 || d > 1 {
			t.Errorf("side (right=%v) first level centered at y=%d, root at y=%d", wantRight, mid, rootCenter)
		}
	}

	// Left-side columns are right-aligned, so siblings share a trunk.
	left := root.children[1]
	for _, c := range left.children {
		if c.x+c.size.Width != left.children[0].x+left.children[0].size.Width {
			t.Errorf("left-side siblings not right-aligned: %+v", left.children)
		}
	}
}

func TestMoveSiblingKeepsRootChildrenOnTheirSide(t *testing.T) {
	root := buildTree(storage.Node{Text: "root", Children: []storage.Node{
		{Text: "a"}, {Text: "b"}, {Text: "c"}, {Text: "d"},
	}})
	arrange(root) // assigns sides: a, c right; b, d left
	a := root.children[0]

	if !moveSibling(a, 1) {
		t.Fatal("moveSibling(a, 1) = false, want true")
	}
	var got []string
	for _, c := range root.children {
		got = append(got, c.text)
	}
	if want := []string{"c", "b", "a", "d"}; !equalStrings(got, want) {
		t.Fatalf("children = %v, want %v", got, want)
	}
	if moveSibling(a, 1) {
		t.Fatal("moveSibling past the last same-side sibling should be a no-op")
	}

	// Deeper nodes still move one step at a time.
	child := buildTree(storage.Node{Text: "p", Children: []storage.Node{{Text: "x"}, {Text: "y"}}})
	child.parent = root
	if !moveSibling(child.children[0], 1) || child.children[0].text != "y" {
		t.Fatal("non-root siblings should swap with their immediate neighbor")
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestTreeNavigatorFollowsSides(t *testing.T) {
	root := buildTree(storage.Node{Text: "root", Children: []storage.Node{
		{Text: "right", Children: []storage.Node{{Text: "right child"}}},
		{Text: "left", Children: []storage.Node{{Text: "left child"}}},
	}})
	arrange(root)
	nav := navigator.Tree[*node]{}
	right, left := root.children[0], root.children[1]

	cases := []struct {
		name string
		got  *node
		want *node
	}{
		{"right of root", nav.RightOf(root), right},
		{"left of root", nav.LeftOf(root), left},
		{"right of right", nav.RightOf(right), right.children[0]},
		{"left of right", nav.LeftOf(right), root},
		{"left of left", nav.LeftOf(left), left.children[0]},
		{"right of left", nav.RightOf(left), root},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("%s = %v, want %q", c.name, c.got, c.want.text)
		}
	}
	if got := nav.LeftOf(left.children[0]); got != nil {
		t.Errorf("left of a left-side leaf = %v, want nil", got)
	}
}

// sidesOf returns the side of each of root's children, by text.
func sidesOf(root *node) map[string]string {
	out := map[string]string{}
	for _, c := range root.children {
		out[c.text] = map[layout.Side]string{layout.Right: "right", layout.Left: "left"}[c.side]
	}
	return out
}

func TestSidesStayPutWhenSiblingsChange(t *testing.T) {
	root := buildTree(storage.Node{Text: "root", Children: []storage.Node{
		{Text: "a"}, {Text: "b"}, {Text: "c"}, {Text: "d"},
	}})
	arrange(root)
	want := map[string]string{"a": "right", "b": "left", "c": "right", "d": "left"}
	if got := sidesOf(root); !equalSides(got, want) {
		t.Fatalf("initial sides = %v, want %v", got, want)
	}

	// A sibling added after a right-side branch joins it on the right,
	// and nobody else moves.
	sib := addSiblingAfter(root.children[0])
	sib.setText("a2")
	arrange(root)
	want["a2"] = "right"
	if got := sidesOf(root); !equalSides(got, want) {
		t.Fatalf("after adding a sibling, sides = %v, want %v", got, want)
	}

	// Removing a branch doesn't move the ones after it either.
	removeNode(root.children[2]) // b
	arrange(root)
	delete(want, "b")
	if got := sidesOf(root); !equalSides(got, want) {
		t.Fatalf("after removing a branch, sides = %v, want %v", got, want)
	}

	// A new child of the root balances the sides: right has a, a2, c
	// and left only d, so it goes left.
	child := addChild(root)
	child.setText("e")
	arrange(root)
	want["e"] = "left"
	if got := sidesOf(root); !equalSides(got, want) {
		t.Fatalf("after adding a root child, sides = %v, want %v", got, want)
	}
}

func equalSides(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

func TestSidesAreNotSaved(t *testing.T) {
	root := buildTree(storage.Node{Text: "root", Children: []storage.Node{{Text: "a"}, {Text: "b"}}})
	arrange(root)

	path := filepath.Join(t.TempDir(), "tree.opml")
	if err := storage.SaveOPML(path, root); err != nil {
		t.Fatalf("SaveOPML: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "side=") {
		t.Fatalf("saved OPML carries layout sides:\n%s", data)
	}
}
