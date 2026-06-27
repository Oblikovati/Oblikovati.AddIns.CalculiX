// SPDX-License-Identifier: GPL-2.0-only

package ccx

import (
	"math"
	"testing"
)

// solveEigen runs a modal/buckling deck and returns its eigenvalues (frequencies or factors).
func solveEigen(t *testing.T, bins solverBinaries, model *AnalysisModel, dir string) []float64 {
	t.Helper()
	stem, err := runDeck(bins, model, dir)
	if err != nil {
		t.Fatalf("solve: %v", err)
	}
	vals, _, _, err := readEigenvalues(stem+".dat", model.Analysis)
	if err != nil {
		t.Fatalf("read eigenvalues: %v", err)
	}
	if len(vals) == 0 {
		t.Fatal("no eigenvalues returned")
	}
	return vals
}

// TestModalCantileverFirstFrequency validates the *FREQUENCY path: the first natural
// frequency of a clamped cantilever matches the Euler-Bernoulli result
//
//	f1 = (1.875104^2 / 2π) · sqrt(E·I / (ρ·A·L^4))
func TestModalCantileverFirstFrequency(t *testing.T) {
	bins := requireSolver(t)
	const (
		L, h    = 100.0, 10.0
		young   = 210000.0
		poisson = 0.3
		rho     = 7.9e-9
	)
	dir := t.TempDir()
	mesh := meshBeam(t, bins, L, h, dir) // beam along x
	root := selectNodes(mesh, func(n Node) bool { return n.X < eps(L) })
	if len(root) == 0 {
		t.Fatal("no root nodes selected")
	}

	model := &AnalysisModel{
		Analysis:       AnalysisFrequency,
		Mesh:           mesh,
		Material:       MaterialProps{Name: "STEEL", YoungMPa: young, Poisson: poisson, DensityTonneMM3: rho},
		Fixed:          []FixedConstraint{{Name: "ROOT", Nodes: root, DOFLow: 1, DOFHigh: 3}},
		EigenmodeCount: 6,
	}
	freqs := solveEigen(t, bins, model, dir)

	inertia := h * h * h * h / 12.0
	area := h * h
	lambda := 1.875104
	want := lambda * lambda / (2 * math.Pi) * math.Sqrt(young*inertia/(rho*area*L*L*L*L))
	got := freqs[0]
	relErr := math.Abs(got-want) / want
	t.Logf("f1: FE=%.1f Hz, Euler-Bernoulli=%.1f Hz, rel err=%.1f%%", got, want, relErr*100)
	if relErr > 0.08 {
		t.Errorf("first natural frequency %.1f Hz differs from analytic %.1f Hz by %.1f%% (>8%%)", got, want, relErr*100)
	}
}

// TestBucklingEulerColumn validates the *BUCKLE path: the buckling factor of a clamped-free
// column under axial compression matches the Euler critical load divided by the applied
// load, P_cr = π²EI / (4L²) (effective length 2L for a fixed-free column).
func TestBucklingEulerColumn(t *testing.T) {
	bins := requireSolver(t)
	const (
		h, L    = 10.0, 200.0 // mm, column along z (slender, L/h = 20)
		young   = 210000.0
		poisson = 0.3
		applied = 1000.0 // N total axial compression
	)
	dir := t.TempDir()
	mesh := meshBox(t, bins, h, h, L, dir)
	base := selectNodes(mesh, func(n Node) bool { return n.Z < eps(L) })
	top := selectNodes(mesh, func(n Node) bool { return n.Z > L-eps(L) })
	if len(base) == 0 || len(top) == 0 {
		t.Fatalf("selection failed (base=%d top=%d)", len(base), len(top))
	}

	model := &AnalysisModel{
		Analysis:       AnalysisBuckling,
		Mesh:           mesh,
		Material:       MaterialProps{Name: "STEEL", YoungMPa: young, Poisson: poisson},
		Fixed:          []FixedConstraint{{Name: "BASE", Nodes: base, DOFLow: 1, DOFHigh: 3}},
		Forces:         []ForceLoad{{Name: "AXIAL", Nodes: top, Dir: [3]float64{0, 0, -1}, TotalN: applied}},
		EigenmodeCount: 4,
	}
	factors := solveEigen(t, bins, model, dir)

	inertia := h * h * h * h / 12.0
	pcr := math.Pi * math.Pi * young * inertia / (4 * L * L)
	want := pcr / applied
	got := factors[0]
	relErr := math.Abs(got-want) / want
	t.Logf("buckling factor: FE=%.3f, Euler P_cr/P=%.3f, rel err=%.1f%%", got, want, relErr*100)
	if relErr > 0.10 {
		t.Errorf("buckling factor %.3f differs from Euler %.3f by %.1f%% (>10%%)", got, want, relErr*100)
	}
}
