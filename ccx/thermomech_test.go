// SPDX-License-Identifier: GPL-2.0-only

package ccx

import (
	"math"
	"strings"
	"testing"
)

func TestDeckWritesThermalCards(t *testing.T) {
	deck := writeDeckString(t, &AnalysisModel{
		Analysis: AnalysisThermomech,
		Mesh:     unitTet(),
		Material: MaterialProps{Name: "STEEL", YoungMPa: 210000, Poisson: 0.3, ExpansionPerK: 1.2e-5},
		Fixed:    []FixedConstraint{{Name: "FIX", Nodes: []int{1}, DOFLow: 1, DOFHigh: 3}},
		Thermal:  &ThermalLoad{DeltaK: 100},
	})
	for _, want := range []string{
		"*EXPANSION, ZERO=0.",
		"*INITIAL CONDITIONS, TYPE=TEMPERATURE",
		"Nall, 0.",
		"*TEMPERATURE",
		"Nall, 100",
	} {
		if !strings.Contains(deck, want) {
			t.Errorf("deck missing %q\n%s", want, deck)
		}
	}
}

// TestThermalFreeExpansion validates the thermomech path: a bar fixed at one end and heated
// uniformly by ΔT expands, with the free-end axial displacement matching α·ΔT·L.
func TestThermalFreeExpansion(t *testing.T) {
	bins := requireSolver(t)
	const (
		h, L  = 10.0, 100.0 // mm, bar along z
		young = 210000.0
		alpha = 1.2e-5 // 1/K
		dT    = 100.0  // K
	)
	dir := t.TempDir()
	mesh := meshBox(t, bins, h, h, L, dir)
	base := selectNodes(mesh, func(n Node) bool { return n.Z < eps(L) })
	top := selectNodes(mesh, func(n Node) bool { return n.Z > L-eps(L) })
	if len(base) == 0 || len(top) == 0 {
		t.Fatalf("selection failed (base=%d top=%d)", len(base), len(top))
	}

	model := &AnalysisModel{
		Analysis: AnalysisThermomech,
		Mesh:     mesh,
		Material: MaterialProps{Name: "STEEL", YoungMPa: young, Poisson: 0.3, ExpansionPerK: alpha},
		Fixed:    []FixedConstraint{{Name: "BASE", Nodes: base, DOFLow: 1, DOFHigh: 3}},
		Thermal:  &ThermalLoad{DeltaK: dT},
	}
	res := solveModel(t, bins, model, dir)

	got := math.Abs(meanDispZ(res, top))
	want := alpha * dT * L
	relErr := math.Abs(got-want) / want
	t.Logf("free expansion: FE=%.5g mm, analytic α·ΔT·L=%.5g mm, rel err=%.1f%%", got, want, relErr*100)
	if relErr > 0.02 {
		t.Errorf("free-end expansion %.5g differs from analytic %.5g by %.1f%% (>2%%)", got, want, relErr*100)
	}
}
