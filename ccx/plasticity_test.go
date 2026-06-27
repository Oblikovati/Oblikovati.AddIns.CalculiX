// SPDX-License-Identifier: GPL-2.0-only

package ccx

import (
	"math"
	"strings"
	"testing"
)

func TestDeckWritesPlastic(t *testing.T) {
	deck := writeDeckString(t, &AnalysisModel{
		Analysis: AnalysisStatic,
		Mesh:     unitTet(),
		Material: MaterialProps{Name: "STEEL", YoungMPa: 210000, Poisson: 0.3, YieldMPa: 250},
		Fixed:    []FixedConstraint{{Name: "FIX", Nodes: []int{1}, DOFLow: 1, DOFHigh: 3}},
		Forces:   []ForceLoad{{Name: "L", Nodes: []int{2}, Dir: [3]float64{0, 0, -1}, TotalN: 1}},
	})
	for _, want := range []string{
		"*ELASTIC",
		"*PLASTIC",
		"250, 0.",           // yield stress, zero plastic strain (perfect plasticity)
		"*STATIC\n0.1, 1.0", // incremented step for the nonlinear solve
	} {
		if !strings.Contains(deck, want) {
			t.Errorf("plastic deck missing %q\n%s", want, deck)
		}
	}
	// A material with no yield must not write *PLASTIC.
	elastic := writeDeckString(t, &AnalysisModel{
		Analysis: AnalysisStatic, Mesh: unitTet(),
		Material: MaterialProps{Name: "STEEL", YoungMPa: 210000, Poisson: 0.3},
		Fixed:    []FixedConstraint{{Name: "FIX", Nodes: []int{1}, DOFLow: 1, DOFHigh: 3}},
		Forces:   []ForceLoad{{Name: "L", Nodes: []int{2}, Dir: [3]float64{0, 0, -1}, TotalN: 1}},
	})
	if strings.Contains(elastic, "*PLASTIC") {
		t.Error("a zero-yield material should not write *PLASTIC")
	}
}

// TestPerfectPlasticityCaps is the plasticity oracle: a bar fixed on one face and stretched
// well past yield on the far face cannot carry more than its yield stress — with ideal
// (perfect) plasticity the axial stress saturates at
//
//	sigma = yield
//
// however far it is pulled (the extra strain becomes plastic). Runs through the real ccx via
// the *PLASTIC path with an incremented *STATIC step.
func TestPerfectPlasticityCaps(t *testing.T) {
	bins := requireSolver(t)
	const (
		h, L  = 10.0, 100.0 // mm, bar along z
		young = 210000.0    // MPa
		yield = 250.0       // MPa
		delta = 0.3         // mm: elastic stress E·δ/L = 630 MPa ≈ 2.5× yield
	)
	dir := t.TempDir()
	mesh := meshBox(t, bins, h, h, L, dir)
	root := selectNodes(mesh, func(n Node) bool { return n.Z < eps(L) })
	far := selectNodes(mesh, func(n Node) bool { return n.Z > L-eps(L) })
	mid := selectNodes(mesh, func(n Node) bool { return math.Abs(n.Z-L/2) < 0.05*L })
	if len(root) == 0 || len(far) == 0 || len(mid) == 0 {
		t.Fatalf("selection failed (root=%d far=%d mid=%d)", len(root), len(far), len(mid))
	}

	model := &AnalysisModel{
		Analysis:      AnalysisStatic,
		Mesh:          mesh,
		Material:      MaterialProps{Name: "STEEL", YoungMPa: young, Poisson: 0.3, YieldMPa: yield},
		Fixed:         []FixedConstraint{{Name: "ROOT", Nodes: root, DOFLow: 1, DOFHigh: 3}},
		Displacements: []DisplacementBC{{Name: "PULL", Nodes: far, DOF: 3, Value: delta}},
	}
	res := solveModel(t, bins, model, dir)

	got := meanSzz(res, mid)
	relErr := math.Abs(got-yield) / yield
	t.Logf("perfect-plasticity axial stress: FE=%.3f MPa, yield=%.3f MPa, rel err=%.1f%%", got, yield, relErr*100)
	if relErr > 0.03 {
		t.Errorf("axial stress %.3f MPa is not capped at yield %.3f MPa (%.1f%% off, >3%%)", got, yield, relErr*100)
	}
}
