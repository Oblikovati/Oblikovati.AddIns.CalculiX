// SPDX-License-Identifier: GPL-2.0-only

package ccx

import (
	"math"
	"strings"
	"testing"
)

// TestEallSetUnitesBodies: a multi-body mesh (per-body Eb0/Eb1 element sets) defines an Eall
// set uniting them, so a body load can address the whole model; a single Eall section needs no
// extra *ELSET.
func TestEallSetUnitesBodies(t *testing.T) {
	mesh := &TetMesh{
		Nodes: []Node{{ID: 1}, {ID: 2}, {ID: 3}, {ID: 4}, {ID: 5}, {ID: 6}, {ID: 7}, {ID: 8}},
		Elements: []TetElement{
			{ID: 1, Nodes: []int{1, 2, 3, 4}, Body: 0},
			{ID: 2, Nodes: []int{5, 6, 7, 8}, Body: 1},
		},
	}
	steel := MaterialProps{Name: "S", YoungMPa: 210000, Poisson: 0.3, DensityTonneMM3: 7.85e-9}
	multi := writeDeckString(t, &AnalysisModel{
		Analysis: AnalysisStatic,
		Mesh:     mesh,
		Material: steel,
		Sections: []MaterialSection{
			{ElsetName: "Eb0", Material: steel, ElementIDs: []int{1}},
			{ElsetName: "Eb1", Material: steel, ElementIDs: []int{2}},
		},
		Gravity: &GravityLoad{Accel: 9810, Dir: [3]float64{0, 0, -1}},
	})
	if !strings.Contains(multi, "*ELSET, ELSET=Eall") || !strings.Contains(multi, "\nEb0\n") || !strings.Contains(multi, "\nEb1\n") {
		t.Errorf("multi-body deck should unite Eb0/Eb1 into Eall:\n%s", multi)
	}
	if !strings.Contains(multi, "Eall, GRAV") {
		t.Errorf("gravity should address Eall:\n%s", multi)
	}

	// A single-section (Eall) study already defines Eall via *ELEMENT — no extra *ELSET.
	single := writeDeckString(t, &AnalysisModel{Analysis: AnalysisStatic, Mesh: unitTet(), Material: steel,
		Gravity: &GravityLoad{Accel: 9810, Dir: [3]float64{0, 0, -1}}})
	if strings.Contains(single, "*ELSET, ELSET=Eall") {
		t.Errorf("single-section deck should not add a redundant *ELSET Eall:\n%s", single)
	}
}

func TestDeckWritesCentrifugal(t *testing.T) {
	deck := writeDeckString(t, &AnalysisModel{
		Analysis:    AnalysisStatic,
		Mesh:        unitTet(),
		Material:    MaterialProps{Name: "STEEL", YoungMPa: 210000, Poisson: 0.3, DensityTonneMM3: 7.85e-9},
		Centrifugal: &CentrifugalLoad{Omega2: 1e6, AxisPoint: [3]float64{0, 0, 0}, AxisDir: [3]float64{0, 0, 1}},
	})
	for _, want := range []string{
		"*DENSITY",
		"*DLOAD",
		"Eall, CENTRIF, 1000000, 0, 0, 0, 0, 0, 1",
	} {
		if !strings.Contains(deck, want) {
			t.Errorf("centrifugal deck missing %q\n%s", want, deck)
		}
	}
}

// TestCentrifugalRotatingBar is the centrifugal-load oracle: a bar of length L rotating about
// an axis through its fixed root carries, at distance x from the axis, an axial tension equal
// to the centrifugal pull of the material beyond it,
//
//	sigma_xx(x) = rho * omega^2 * (L^2 - x^2) / 2.
//
// Sampled at the mid-section (x = L/2, away from the fixed-face constraint concentration) it
// gives 3/8·rho·omega²·L². Runs through the real ccx via the *DLOAD CENTRIF path.
func TestCentrifugalRotatingBar(t *testing.T) {
	bins := requireSolver(t)
	const (
		L, h  = 200.0, 10.0 // mm: slender bar along x (rotation axis = z through origin)
		young = 210000.0    // MPa
		rho   = 7.85e-9     // t/mm^3
		omega = 1000.0      // rad/s
	)
	dir := t.TempDir()
	mesh := meshBeam(t, bins, L, h, dir)
	root := selectNodes(mesh, func(n Node) bool { return n.X < eps(L) })
	mid := selectNodes(mesh, func(n Node) bool { return math.Abs(n.X-L/2) < 0.05*L })
	if len(root) == 0 || len(mid) == 0 {
		t.Fatalf("selection failed (root=%d mid=%d)", len(root), len(mid))
	}

	model := &AnalysisModel{
		Analysis:    AnalysisStatic,
		Mesh:        mesh,
		Material:    MaterialProps{Name: "STEEL", YoungMPa: young, Poisson: 0.3, DensityTonneMM3: rho},
		Fixed:       []FixedConstraint{{Name: "ROOT", Nodes: root, DOFLow: 1, DOFHigh: 3}},
		Centrifugal: &CentrifugalLoad{Omega2: omega * omega, AxisPoint: [3]float64{0, 0, 0}, AxisDir: [3]float64{0, 0, 1}},
	}
	res := solveModel(t, bins, model, dir)

	got := meanSxx(res, mid)
	want := rho * omega * omega * (L*L - (L/2)*(L/2)) / 2
	relErr := math.Abs(got-want) / want
	t.Logf("centrifugal mid-section axial stress: FE=%.3f MPa, analytic ρω²(L²-x²)/2=%.3f MPa, rel err=%.1f%%", got, want, relErr*100)
	if relErr > 0.05 {
		t.Errorf("centrifugal mid-section stress %.3f MPa differs from analytic %.3f MPa by %.1f%% (>5%%)", got, want, relErr*100)
	}
}

// meanSxx returns the mean axial (xx) stress over a node set.
func meanSxx(res *ResultField, nodes []int) float64 {
	sum := 0.0
	for _, id := range nodes {
		sum += res.Stress[id][0]
	}
	return sum / float64(len(nodes))
}
