package main

import (
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"hmm/navigator"
	"hmm/storage"
)

func TestBuildDemoSceneLayout(t *testing.T) {
	demoTree, err := storage.LoadOPML("testdata/demo_tree.opml")
	if err != nil {
		t.Fatalf("loading testdata/demo_tree.opml: %v", err)
	}
	_, boxes, edges, _ := buildScene(demoTree)

	var countNodes func(spec storage.Node) int
	countNodes = func(spec storage.Node) int {
		n := 1
		for _, c := range spec.Children {
			n += countNodes(c)
		}
		return n
	}
	wantNodes := countNodes(demoTree)

	if len(boxes) != wantNodes {
		t.Fatalf("got %d boxes, want %d", len(boxes), wantNodes)
	}
	if len(edges) != wantNodes-1 {
		t.Fatalf("got %d edges, want %d (a tree has n-1 edges)", len(edges), wantNodes-1)
	}

	for i, a := range boxes {
		if a.width <= 0 || a.height <= 0 {
			t.Fatalf("box %d has non-positive size %dx%d", i, a.width, a.height)
		}
		for j, b := range boxes {
			if i == j {
				continue
			}
			if rectsOverlap(a.x, a.y, a.width, a.height, b.x, b.y, b.width, b.height) {
				t.Fatalf("boxes %d and %d overlap: %+v vs %+v", i, j, a, b)
			}
		}
	}

	for i, e := range edges {
		if e.x2 <= e.x1 {
			t.Fatalf("edge %d does not run strictly left to right: %+v", i, e)
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
	root, _, _, _ := buildScene(demoTree)

	if root.box.x != 0 || root.box.y != 0 {
		t.Fatalf("root at (%d, %d), want (0, 0)", root.box.x, root.box.y)
	}
	if len(root.children) < 2 {
		t.Fatalf("demo tree needs at least 2 root children, has %d", len(root.children))
	}

	rootCenter := root.box.y + root.box.height/2
	for dir, wantRight := range []bool{true, false} {
		var firstLevel []*node
		for i, c := range root.children {
			if i%2 != dir {
				continue
			}
			firstLevel = append(firstLevel, c)
			eachDescendant(c, func(d *node) {
				if wantRight && d.box.x < root.box.x+root.box.width+colGap {
					t.Errorf("%q should be right of the root, got x=%d", d.box.text, d.box.x)
				}
				if !wantRight && d.box.x+d.box.width > root.box.x-colGap {
					t.Errorf("%q should be left of the root, got x=%d", d.box.text, d.box.x)
				}
			})
		}

		first, last := firstLevel[0], firstLevel[len(firstLevel)-1]
		mid := (first.box.y + last.box.y + last.box.height) / 2
		if d := mid - rootCenter; d < -1 || d > 1 {
			t.Errorf("side (right=%v) first level centered at y=%d, root at y=%d", wantRight, mid, rootCenter)
		}
	}

	// Left-side columns are right-aligned, so siblings share a trunk.
	left := root.children[1]
	for _, c := range left.children {
		if c.box.x+c.box.width != left.children[0].box.x+left.children[0].box.width {
			t.Errorf("left-side siblings not right-aligned: %+v", left.children)
		}
	}
}

func TestConnectorMirrorsAcrossSides(t *testing.T) {
	root := buildTree(storage.Node{Text: "root", Children: []storage.Node{
		{Text: "right"},
		{Text: "left"},
	}})
	layoutTree(root)
	right, left := root.children[0], root.children[1]

	// Cell x on the right mirrors to cell mirror-x on the left, where
	// mirror is the root's first and last cell summed.
	mirror := root.box.x + root.box.x + root.box.width - 1
	r, l := connector(root, right), connector(root, left)
	if r.x1+l.x2 != mirror || r.x2+l.x1 != mirror {
		t.Fatalf("connectors not mirrored around the root: right %+v, left %+v", r, l)
	}
	if rTrunk, lTrunk := (r.x1+r.x2)/2, (l.x1+l.x2)/2; rTrunk+lTrunk != mirror {
		t.Fatalf("trunks not mirrored: right x=%d, left x=%d", rTrunk, lTrunk)
	}
}

func TestMoveSiblingKeepsRootChildrenOnTheirSide(t *testing.T) {
	root := buildTree(storage.Node{Text: "root", Children: []storage.Node{
		{Text: "a"}, {Text: "b"}, {Text: "c"}, {Text: "d"},
	}})
	layoutTree(root) // assigns sides: a, c right; b, d left
	a := root.children[0]

	if !moveSibling(a, 1) {
		t.Fatal("moveSibling(a, 1) = false, want true")
	}
	var got []string
	for _, c := range root.children {
		got = append(got, c.box.text)
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
	if !moveSibling(child.children[0], 1) || child.children[0].box.text != "y" {
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
	var boxes []box
	var edges []edge
	var nodes []*node
	layoutTree(root)
	flattenTree(root, &boxes, &edges, &nodes)
	nav := navigator.TreeNavigator{Nodes: navNodes(nodes)}
	right, left := root.children[0], root.children[1]

	cases := []struct {
		name string
		got  navigator.NavNode
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
		if c.got != navigator.NavNode(c.want) {
			t.Errorf("%s = %v, want %q", c.name, c.got, c.want.box.text)
		}
	}
	if got := nav.LeftOf(left.children[0]); got != nil {
		t.Errorf("left of a left-side leaf = %v, want nil", got)
	}
}

// sizedNode builds a node with an explicit box size, bypassing text
// rendering, so layout tests can use sizes newNode never produces.
func sizedNode(w, h int, children ...*node) *node {
	n := &node{box: box{width: w, height: h}, children: children}
	for _, c := range children {
		c.parent = n
	}
	return n
}

func TestTidyLayoutRandomTrees(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	var grow func(depth int) *node
	grow = func(depth int) *node {
		var children []*node
		if depth < 5 {
			for range rng.Intn(5) {
				children = append(children, grow(depth+1))
			}
		}
		return sizedNode(3+rng.Intn(30), 1+rng.Intn(6), children...)
	}

	for iter := range 200 {
		root := grow(0)
		layoutTree(root)

		var all []*node
		eachDescendant(root, func(n *node) { all = append(all, n) })

		for i, a := range all {
			for _, b := range all[i+1:] {
				// Boxes sharing any column must also keep rowGap between them.
				if a.box.x < b.box.x+b.box.width && b.box.x < a.box.x+a.box.width &&
					a.box.y < b.box.y+b.box.height+rowGap && b.box.y < a.box.y+a.box.height+rowGap {
					t.Fatalf("iter %d: boxes too close: %+v vs %+v", iter, a.box, b.box)
				}
			}
		}

		eachDescendant(root, func(n *node) {
			if n == root || len(n.children) == 0 {
				return
			}
			for _, c := range n.children {
				right := c.box.x == n.box.x+n.box.width+colGap
				left := c.box.x+c.box.width+colGap == n.box.x
				if !right && !left {
					t.Fatalf("iter %d: child %+v not colGap past its parent %+v", iter, c.box, n.box)
				}
			}
			first, last := n.children[0], n.children[len(n.children)-1]
			span := first.box.y + last.box.y + last.box.height
			if d := 2*n.box.y + n.box.height - span; d < -2 || d > 2 {
				t.Fatalf("iter %d: parent %+v not centered on children %+v..%+v", iter, n.box, first.box, last.box)
			}
		})
	}
}

func TestTidyLayoutIsNonLayered(t *testing.T) {
	wide := sizedNode(30, 1, sizedNode(5, 1))
	narrow := sizedNode(5, 1, sizedNode(5, 1))
	root := sizedNode(5, 1, sizedNode(5, 1, wide, narrow))
	layoutTree(root)

	gw, gn := wide.children[0], narrow.children[0]
	if gw.box.x == gn.box.x {
		t.Fatalf("grandchildren share a column (x=%d); each should sit just past its own parent", gw.box.x)
	}
	if gn.box.x != narrow.box.x+narrow.box.width+colGap {
		t.Fatalf("narrow parent's child at x=%d, want %d", gn.box.x, narrow.box.x+narrow.box.width+colGap)
	}
}

func TestTidyLayoutSpacesSmallSiblingsEvenly(t *testing.T) {
	tall := func() *node {
		var kids []*node
		for range 8 {
			kids = append(kids, sizedNode(5, 3))
		}
		return sizedNode(5, 3, kids...)
	}
	mid := []*node{sizedNode(5, 3), sizedNode(5, 3), sizedNode(5, 3)}
	parent := sizedNode(5, 3, append(append([]*node{tall()}, mid...), tall())...)
	root := sizedNode(5, 3, parent)
	layoutTree(root)

	var gaps []int
	for i := 1; i < len(parent.children); i++ {
		prev, cur := parent.children[i-1], parent.children[i]
		gaps = append(gaps, cur.box.y-(prev.box.y+prev.box.height))
	}
	for _, g := range gaps {
		if g-gaps[0] < -1 || g-gaps[0] > 1 {
			t.Fatalf("gaps between siblings = %v, want them (nearly) equal", gaps)
		}
	}
	if gaps[0] <= rowGap {
		t.Fatalf("gaps = %v: the small siblings were packed, not spread", gaps)
	}
}

// sidesOf returns the side of each of root's children, by text.
func sidesOf(root *node) map[string]string {
	out := map[string]string{}
	for _, c := range root.children {
		out[c.box.text] = map[side]string{sideRight: "right", sideLeft: "left"}[c.side]
	}
	return out
}

func TestSidesStayPutWhenSiblingsChange(t *testing.T) {
	root := buildTree(storage.Node{Text: "root", Children: []storage.Node{
		{Text: "a"}, {Text: "b"}, {Text: "c"}, {Text: "d"},
	}})
	layoutTree(root)
	want := map[string]string{"a": "right", "b": "left", "c": "right", "d": "left"}
	if got := sidesOf(root); !equalSides(got, want) {
		t.Fatalf("initial sides = %v, want %v", got, want)
	}

	// A sibling added after a right-side branch joins it on the right,
	// and nobody else moves.
	sib := addSiblingAfter(root.children[0])
	sib.box.text = "a2"
	layoutTree(root)
	want["a2"] = "right"
	if got := sidesOf(root); !equalSides(got, want) {
		t.Fatalf("after adding a sibling, sides = %v, want %v", got, want)
	}

	// Removing a branch doesn't move the ones after it either.
	removeNode(root.children[2]) // b
	layoutTree(root)
	delete(want, "b")
	if got := sidesOf(root); !equalSides(got, want) {
		t.Fatalf("after removing a branch, sides = %v, want %v", got, want)
	}

	// A new child of the root balances the sides: right has a, a2, c
	// and left only d, so it goes left.
	child := addChild(root)
	child.box.text = "e"
	layoutTree(root)
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
	layoutTree(root)

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
