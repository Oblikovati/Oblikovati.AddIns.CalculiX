// SPDX-License-Identifier: GPL-2.0-only

package ccx

import (
	"math"
	"strings"
	"testing"
)

func TestDeckWritesDisplacement(t *testing.T) {
	deck := writeDeckString(t, &AnalysisModel{
		Analysis:      AnalysisStatic,
		Mesh:          unitTet(),
		Material:      MaterialProps{Name: "STEEL", YoungMPa: 210000, Poisson: 0.3},
		Fixed:         []FixedConstraint{{Name: "FIX", Nodes: []int{1}, DOFLow: 1, DOFHigh: 3}},
		Displacements: []DisplacementBC{{Name: "PRESCR", Nodes: []int{2}, DOF: 3, Value: 0.5}},
	})
	for _, want := range []string{
		"*NSET, NSET=PRESCR",
		"PRESCR, 3, 3, 0.5", // enforced displacement (non-zero) on DOF 3
	} {
		if !strings.Contains(deck, want) {
			t.Errorf("displacement deck missing %q\n%s", want, deck)
		}
	}
}

// TestPrescribedDisplacementStretch is the enforced-displacement oracle: a bar fixed on one
// face and pulled a distance δ on the far face (its other DOFs free) carries a uniform axial
// stress
//
//	sigma = E * delta / L
//
// (Hooke's law for the imposed strain δ/L). Runs through the real ccx via the non-zero
// *BOUNDARY path.
func TestPrescribedDisplacementStretch(t *testing.T) {
	bins := requireSolver(t)
	const (
		h, L  = 10.0, 100.0 // mm, bar along z
		young = 210000.0    // MPa
		delta = 0.1         // mm enforced stretch on the far face
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
		Material:      MaterialProps{Name: "STEEL", YoungMPa: young, Poisson: 0.3},
		Fixed:         []FixedConstraint{{Name: "ROOT", Nodes: root, DOFLow: 1, DOFHigh: 3}},
		Displacements: []DisplacementBC{{Name: "PULL", Nodes: far, DOF: 3, Value: delta}},
	}
	res := solveModel(t, bins, model, dir)

	got := meanSzz(res, mid)
	want := young * delta / L
	relErr := math.Abs(got-want) / want
	t.Logf("enforced-displacement axial stress: FE=%.3f MPa, analytic E·δ/L=%.3f MPa, rel err=%.1f%%", got, want, relErr*100)
	if relErr > 0.05 {
		t.Errorf("axial stress %.3f MPa differs from analytic %.3f MPa by %.1f%% (>5%%)", got, want, relErr*100)
	}
}

// meanSzz returns the mean axial (zz) stress over a node set.
func meanSzz(res *ResultField, nodes []int) float64 {
	sum := 0.0
	for _, id := range nodes {
		sum += res.Stress[id][2]
	}
	return sum / float64(len(nodes))
}
