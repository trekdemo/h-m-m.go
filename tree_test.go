package main

import "testing"

func TestBuildDemoSceneLayout(t *testing.T) {
	boxes, edges := buildDemoScene()

	var countNodes func(spec treeSpec) int
	countNodes = func(spec treeSpec) int {
		n := 1
		for _, c := range spec.children {
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
			t.Fatalf("edge %d does not move strictly rightward from parent to child: %+v", i, e)
		}
	}
}
