package main

import (
	"testing"

	"github.com/trekdemo/bubbletea-exp/storage"
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
			t.Fatalf("edge %d does not move strictly rightward from parent to child: %+v", i, e)
		}
	}
}
