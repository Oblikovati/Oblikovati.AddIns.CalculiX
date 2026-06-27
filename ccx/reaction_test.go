// SPDX-License-Identifier: GPL-2.0-only

package ccx

import (
	"math"
	"strings"
	"testing"
)

func TestStaticDeckRequestsReaction(t *testing.T) {
	mesh := &TetMesh{
		Nodes:    []Node{{ID: 1}, {ID: 2}, {ID: 3}, {ID: 4}},
		Elements: []TetElement{{ID: 1, Nodes: []int{1, 2, 3, 4}}},
	}
	deck := writeDeckString(t, &AnalysisModel{
		Analysis: AnalysisStatic,
		Mesh:     mesh,
		Material: MaterialProps{Name: "STEEL", YoungMPa: 210000, Poisson: 0.3},
		Fixed:    []FixedConstraint{{Name: "FIX", Nodes: []int{1, 2}, DOFLow: 1, DOFHigh: 3}},
		Forces:   []ForceLoad{{Name: "T", Nodes: []int{3, 4}, Dir: [3]float64{0, 0, -1}, TotalN: 100}},
	})
	for _, want := range []string{"*NODE PRINT, NSET=FIX, TOTALS=ONLY", "RF"} {
		if !strings.Contains(deck, want) {
			t.Errorf("static deck missing reaction request %q\n%s", want, deck)
		}
	}
}

// TestNoReactionForModal checks a modal study omits the reaction print: a free-vibration
// eigenproblem has no applied load and so no meaningful boundary reaction, even with a clamp.
func TestNoReactionForModal(t *testing.T) {
	mesh := &TetMesh{
		Nodes:    []Node{{ID: 1}, {ID: 2}, {ID: 3}, {ID: 4}},
		Elements: []TetElement{{ID: 1, Nodes: []int{1, 2, 3, 4}}},
	}
	deck := writeDeckString(t, &AnalysisModel{
		Analysis:       AnalysisFrequency,
		Mesh:           mesh,
		Material:       MaterialProps{Name: "STEEL", YoungMPa: 210000, Poisson: 0.3, DensityTonneMM3: 7.85e-9},
		Fixed:          []FixedConstraint{{Name: "FIX", Nodes: []int{1, 2}, DOFLow: 1, DOFHigh: 3}},
		EigenmodeCount: 3,
	})
	if strings.Contains(deck, "*NODE PRINT") {
		t.Errorf("a modal study must not request a boundary reaction\n%s", deck)
	}
}

func TestParseTotalReaction(t *testing.T) {
	dat := ` total force (fx,fy,fz) for set FIX and time  0.1000000E+01

   1.5000000E+01 -2.0000000E+01  -1.0000000E+03
`
	v, err := parseTotalReaction(strings.NewReader(dat))
	if err != nil {
		t.Fatal(err)
	}
	got := reactionMagnitude(v)
	want := math.Sqrt(15*15 + 20*20 + 1000*1000)
	if math.Abs(got-want) > 1e-6 {
		t.Errorf("reaction magnitude = %.6f, want %.6f", got, want)
	}
}

// TestReactionBalancesAppliedLoad is the reaction oracle: a fixed cantilever pulled by a known
// end force F must, by equilibrium, push back through its clamp with a total reaction of
// magnitude F. This validates the *NODE PRINT RF deck request + .dat reaction parser end to end
// through the real vendored ccx.
func TestReactionBalancesAppliedLoad(t *testing.T) {
	bins := requireSolver(t)
	const (
		w, L = 20.0, 80.0 // mm: a 20×20×80 bar
		f    = 1000.0     // N axial end load
	)
	dir := t.TempDir()
	mesh := meshBox(t, bins, w, w, L, dir)

	root := selectNodes(mesh, func(n Node) bool { return n.Z < eps(L) })
	tip := selectNodes(mesh, func(n Node) bool { return n.Z > L-eps(L) })
	if len(root) == 0 || len(tip) == 0 {
		t.Fatalf("selection failed (root=%d tip=%d)", len(root), len(tip))
	}
	model := &AnalysisModel{
		Analysis: AnalysisStatic,
		Mesh:     mesh,
		Material: MaterialProps{Name: "STEEL", YoungMPa: 210000, Poisson: 0.3},
		Fixed:    []FixedConstraint{{Name: "FIX", Nodes: root, DOFLow: 1, DOFHigh: 3}},
		Forces:   []ForceLoad{{Name: "TIP", Nodes: tip, Dir: [3]float64{0, 0, 1}, TotalN: f}},
	}
	stem, err := runDeck(bins, model, dir)
	if err != nil {
		t.Fatalf("solve: %v", err)
	}
	got := readReaction(stem + ".dat")
	relErr := math.Abs(got-f) / f
	t.Logf("support reaction: FE=%.4f N, applied load=%.4f N, rel err=%.2f%%", got, f, relErr*100)
	if relErr > 0.001 {
		t.Errorf("reaction %.4f N differs from applied %.4f N by %.2f%% (>0.1%%)", got, f, relErr*100)
	}
}
