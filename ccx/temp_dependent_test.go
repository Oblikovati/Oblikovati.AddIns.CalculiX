// SPDX-License-Identifier: GPL-2.0-only

package ccx

import (
	"math"
	"strings"
	"testing"
)

func TestDeckWritesTemperatureDependentElastic(t *testing.T) {
	mesh := &TetMesh{
		Nodes:    []Node{{ID: 1}, {ID: 2}, {ID: 3}, {ID: 4}},
		Elements: []TetElement{{ID: 1, Nodes: []int{1, 2, 3, 4}}},
	}
	deck := writeDeckString(t, &AnalysisModel{
		Analysis: AnalysisThermomech,
		Mesh:     mesh,
		Material: MaterialProps{Name: "STEEL", Poisson: 0.3, ExpansionPerK: 1.2e-5, ElasticTable: []ElasticTempPoint{
			{YoungMPa: 210000, Poisson: 0.3, TempK: 0},
			{YoungMPa: 105000, Poisson: 0.3, TempK: 100},
		}},
		Fixed:   []FixedConstraint{{Name: "FIX", Nodes: []int{1, 2}, DOFLow: 1, DOFHigh: 3}},
		Thermal: &ThermalLoad{DeltaK: 100},
	})
	for _, want := range []string{
		"*ELASTIC",
		"210000, 0.3, 0",
		"105000, 0.3, 100",
	} {
		if !strings.Contains(deck, want) {
			t.Errorf("temperature-dependent deck missing %q\n%s", want, deck)
		}
	}
}

// TestTemperatureDependentBlockedStress is the E(T) oracle: a bar heated by ΔT but blocked from
// expanding along its axis (both end faces held in z, the lateral faces on symmetry rollers so
// they expand freely) develops a uniform axial thermal stress
//
//	σ_zz = −E(T)·α·ΔT
//
// using the Young's modulus AT THE BAR'S TEMPERATURE — so a softer hot modulus gives a
// proportionally smaller stress. This validates the temperature-dependent *ELASTIC table being
// interpolated by the element temperature, end to end through the real vendored ccx.
func TestTemperatureDependentBlockedStress(t *testing.T) {
	bins := requireSolver(t)
	const (
		w, L    = 10.0, 50.0 // mm bar
		eCold   = 210000.0   // MPa at 0 K
		eHot    = 105000.0   // MPa at hotK (half — a strong, easy-to-detect drop)
		hotK    = 100.0      // table upper temperature
		alpha   = 1.2e-5     // 1/K
		deltaTK = 100.0      // ΔT applied (= hotK, so E is evaluated at eHot)
	)
	dir := t.TempDir()
	mesh := meshBox(t, bins, w, w, L, dir)

	x0 := selectNodes(mesh, func(n Node) bool { return n.X < eps(w) })
	y0 := selectNodes(mesh, func(n Node) bool { return n.Y < eps(w) })
	z0 := selectNodes(mesh, func(n Node) bool { return n.Z < eps(L) })
	zL := selectNodes(mesh, func(n Node) bool { return n.Z > L-eps(L) })
	if len(x0) == 0 || len(y0) == 0 || len(z0) == 0 || len(zL) == 0 {
		t.Fatalf("face selection failed (x0=%d y0=%d z0=%d zL=%d)", len(x0), len(y0), len(z0), len(zL))
	}
	model := &AnalysisModel{
		Analysis: AnalysisThermomech,
		Mesh:     mesh,
		Material: MaterialProps{Name: "STEEL", Poisson: 0.3, ExpansionPerK: alpha, ElasticTable: []ElasticTempPoint{
			{YoungMPa: eCold, Poisson: 0.3, TempK: 0},
			{YoungMPa: eHot, Poisson: 0.3, TempK: hotK},
		}},
		Fixed: []FixedConstraint{
			{Name: "SYMX", Nodes: x0, DOFLow: 1, DOFHigh: 1},
			{Name: "SYMY", Nodes: y0, DOFLow: 2, DOFHigh: 2},
			{Name: "BOTZ", Nodes: z0, DOFLow: 3, DOFHigh: 3},
			{Name: "TOPZ", Nodes: zL, DOFLow: 3, DOFHigh: 3},
		},
		Thermal: &ThermalLoad{DeltaK: deltaTK},
	}
	res := solveModel(t, bins, model, dir)

	mid := selectNodes(mesh, func(n Node) bool { return n.Z > L/2-eps(L) && n.Z < L/2+eps(L) })
	got := meanSzz(res, mid)
	want := -eHot * alpha * deltaTK // E evaluated at the hot temperature
	relErr := math.Abs(got-want) / math.Abs(want)
	t.Logf("blocked thermal stress: FE σ_zz=%.4f MPa, analytic −E(T)·α·ΔT=%.4f MPa, rel err=%.1f%%", got, want, relErr*100)
	if relErr > 0.02 {
		t.Errorf("axial stress %.4f MPa differs from −E(T)·α·ΔT %.4f MPa by %.1f%% (>2%%)", got, want, relErr*100)
	}
}
