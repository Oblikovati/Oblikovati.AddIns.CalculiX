// SPDX-License-Identifier: GPL-2.0-only

package ccx

import (
	"math"
	"strings"
	"testing"
)

func TestWriteSpringSupportDeck(t *testing.T) {
	mesh := &TetMesh{
		Nodes:    []Node{{ID: 1}, {ID: 2}, {ID: 3}, {ID: 4}},
		Elements: []TetElement{{ID: 1, Nodes: []int{1, 2, 3, 4}}},
	}
	deck := writeDeckString(t, &AnalysisModel{
		Analysis: AnalysisStatic,
		Mesh:     mesh,
		Material: MaterialProps{Name: "STEEL", YoungMPa: 210000, Poisson: 0.3},
		Springs: []SpringSupport{{
			Name: "FIX", Nodes: []int{1, 2}, StiffnessTotal: 1000, FirstElem: 2,
		}},
		Forces: []ForceLoad{{Name: "T", Nodes: []int{3, 4}, Dir: [3]float64{0, 0, -1}, TotalN: 100}},
	})
	for _, want := range []string{
		"*ELEMENT, TYPE=SPRING1, ELSET=FIX_SPRING1",
		"2, 1", // first spring element (id 2) on node 1, DOF-1 set
		"*SPRING, ELSET=FIX_SPRING1",
		"*ELEMENT, TYPE=SPRING1, ELSET=FIX_SPRING3",
		"*SPRING, ELSET=FIX_SPRING3",
		"500", // per-node stiffness: total 1000 over 2 nodes
	} {
		if !strings.Contains(deck, want) {
			t.Errorf("spring deck missing %q\n%s", want, deck)
		}
	}
	// Spring cards are model-level — written before the *STEP.
	if strings.Index(deck, "*SPRING") > strings.Index(deck, "*STEP") {
		t.Error("*SPRING must be written before *STEP")
	}
	// Two nodes × three directions = six SPRING1 elements, none colliding with the solid element id 1.
	if got := strings.Count(deck, "TYPE=SPRING1"); got != 3 {
		t.Errorf("expected 3 SPRING1 element sets (one per direction), got %d", got)
	}
}

// TestElasticSupportSettlesOnSpringFoundation is the spring oracle: a box rests on a grounded
// elastic foundation (its bottom face sprung to ground in every direction with total stiffness K)
// and is pushed down by a force F on the top face. The (stiff) block barely compresses, so it
// settles on the springs by
//
//	delta = F / K
//
// which only holds if the *SPRING foundation actually carries the load (without it the body is
// unconstrained and the solve is singular). This validates the elastic-support path end to end
// through the real vendored ccx.
func TestElasticSupportSettlesOnSpringFoundation(t *testing.T) {
	bins := requireSolver(t)
	const (
		w, L  = 10.0, 50.0 // mm: a 10×10×50 column
		young = 210000.0   // MPa
		k     = 1000.0     // N/mm total foundation stiffness (≪ block axial stiffness EA/L = 4.2e5)
		f     = 500.0      // N compressive load (−z) on the top face
	)
	dir := t.TempDir()
	mesh := meshBox(t, bins, w, w, L, dir)

	base := selectNodes(mesh, func(n Node) bool { return n.Z < eps(L) })
	top := selectNodes(mesh, func(n Node) bool { return n.Z > L-eps(L) })
	if len(base) == 0 || len(top) == 0 {
		t.Fatalf("selection failed (base=%d top=%d)", len(base), len(top))
	}
	model := &AnalysisModel{
		Analysis: AnalysisStatic,
		Mesh:     mesh,
		Material: MaterialProps{Name: "STEEL", YoungMPa: young, Poisson: 0.3},
		Springs:  []SpringSupport{{Name: "FIX", Nodes: base, StiffnessTotal: k, FirstElem: maxElementID(mesh) + 1}},
		Forces:   []ForceLoad{{Name: "TOP", Nodes: top, Dir: [3]float64{0, 0, -1}, TotalN: f}},
	}
	res := solveModel(t, bins, model, dir)

	got := math.Abs(meanUZ(res, top))
	want := f / k
	relErr := math.Abs(got-want) / want
	t.Logf("elastic-support settlement: FE=%.5f mm, analytic F/K=%.5f mm, rel err=%.1f%%", got, want, relErr*100)
	if relErr > 0.02 {
		t.Errorf("settlement %.5f mm differs from F/K %.5f mm by %.1f%% (>2%%)", got, want, relErr*100)
	}
}
