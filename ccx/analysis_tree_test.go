// SPDX-License-Identifier: GPL-2.0-only

package ccx

import (
	"testing"

	"oblikovati.org/api/wire"
	"oblikovati.org/calculix/ccx/femmodel"
)

func TestAnalysisNodesReflectAggregate(t *testing.T) {
	a := femmodel.NewDefaultAnalysis()
	nodes := analysisNodes(a, nil)
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
	cons := []ConstraintSpec{fixedSpecForTest(), fixedSpecForTest()}
	nodes := analysisNodes(a, cons)
	cn := findChild(nodes[0].Children, "constraints")
	if len(cn.Children) != 2 || cn.Children[1].ID != "con:1" {
		t.Fatalf("want two constraint leaves con:0/con:1, got %+v", cn.Children)
	}
}

// fixedSpecForTest returns a minimal FixedSpec for tree-builder tests; it only needs to satisfy
// the ConstraintSpec interface — the faces are never resolved in unit tests.
func fixedSpecForTest() ConstraintSpec {
	return FixedSpec{Name: "C0", Faces: []string{"face:k"}}
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
