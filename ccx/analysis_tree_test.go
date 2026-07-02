// SPDX-License-Identifier: GPL-2.0-only

package ccx

import (
	"testing"

	"oblikovati.org/api/wire"
	"oblikovati.org/calculix/ccx/femmodel"
)

func TestAnalysisNodesReflectAggregate(t *testing.T) {
	a := femmodel.NewDefaultAnalysis()
	nodes := analysisNodes(a)
	if len(nodes) != 1 || nodes[0].ID != "analysis" {
		t.Fatalf("want single 'analysis' root, got %+v", nodes)
	}
	kids := childIDs(nodes[0].Children)
	for _, want := range []string{"solver", "mesh", "materials", "constraints", "results"} {
		if !contains(kids, want) {
			t.Fatalf("root missing %q child; got %v", want, kids)
		}
	}
	// One default material + one default result appear as leaves.
	mats := findChild(nodes[0].Children, "materials")
	if len(mats.Children) != 1 || mats.Children[0].ID != "mat:0" {
		t.Fatalf("want one mat:0 leaf, got %+v", mats.Children)
	}
}

func TestAnalysisNodesListConstraints(t *testing.T) {
	a := femmodel.NewDefaultAnalysis()
	// Constraints are added to the aggregate; the tree reads a.Constraints().
	a.AddConstraint("C0", femmodel.ConstraintObject{Kind: "fixed", Faces: []string{"face:k"}})
	a.AddConstraint("C1", femmodel.ConstraintObject{Kind: "fixed", Faces: []string{"face:k"}})
	nodes := analysisNodes(a)
	cn := findChild(nodes[0].Children, "constraints")
	if len(cn.Children) != 2 || cn.Children[1].ID != "con:1" {
		t.Fatalf("want two constraint leaves con:0/con:1, got %+v", cn.Children)
	}
}

// TestAnalysisNodesCarryGlyphs guards the icon pass: every node the tree emits — root, category,
// and per-object leaf (a seeded material + result appear as leaves) — must carry an inline glyph.
func TestAnalysisNodesCarryGlyphs(t *testing.T) {
	nodes := analysisNodes(femmodel.NewDefaultAnalysis())
	var count int
	forEachNode(nodes, func(n wire.BrowserNodeSpec) {
		count++
		if n.IconSVG == "" {
			t.Errorf("node %q (%q) has no glyph", n.ID, n.Label)
		}
	})
	// Guard against a vacuous pass: the default tree must carry the container nodes plus the
	// seeded material + result leaves — so the leaf glyphs above are actually exercised.
	mats := findChild(nodes[0].Children, "materials")
	res := findChild(nodes[0].Children, "results")
	if len(mats.Children) == 0 || len(res.Children) == 0 {
		t.Fatalf("expected seeded material + result leaves to walk; mats=%d results=%d", len(mats.Children), len(res.Children))
	}
	if count < 7 {
		t.Fatalf("expected the full default tree (root+5 categories+leaves) to be walked, visited only %d", count)
	}
}

func forEachNode(ns []wire.BrowserNodeSpec, fn func(wire.BrowserNodeSpec)) {
	for _, n := range ns {
		fn(n)
		forEachNode(n.Children, fn)
	}
}

// --- tiny test helpers (keep in this file) ---
func childIDs(ns []wire.BrowserNodeSpec) []string {
	out := make([]string, len(ns))
	for i, n := range ns {
		out[i] = n.ID
	}
	return out
}
func contains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}
func findChild(ns []wire.BrowserNodeSpec, id string) wire.BrowserNodeSpec {
	for _, n := range ns {
		if n.ID == id {
			return n
		}
	}
	return wire.BrowserNodeSpec{}
}
