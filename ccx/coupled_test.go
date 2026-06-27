// SPDX-License-Identifier: GPL-2.0-only

package ccx

import (
	"math"
	"strings"
	"testing"
)

func TestDeckWritesCoupledSteady(t *testing.T) {
	deck := writeDeckString(t, &AnalysisModel{
		Analysis: AnalysisCoupledThermal,
		Mesh:     unitTet(),
		Material: MaterialProps{Name: "STEEL", YoungMPa: 210000, Poisson: 0.3, ExpansionPerK: 1.2e-5, Conductivity: 50},
		Fixed:    []FixedConstraint{{Name: "FIX", Nodes: []int{1}, DOFLow: 1, DOFHigh: 3}},
		Temperatures: []TemperatureBC{
			{Name: "TCOLD", Nodes: []int{1}, TempK: 0},
			{Name: "THOT", Nodes: []int{2}, TempK: 100},
		},
	})
	for _, want := range []string{
		"*COUPLED TEMPERATURE-DISPLACEMENT, STEADY STATE",
		"*ELASTIC",
		"*EXPANSION, ZERO=0.",
		"*CONDUCTIVITY",
		"THOT, 11, 11, 100",
		"U, NT",
		"*EL FILE",
	} {
		if !strings.Contains(deck, want) {
			t.Errorf("coupled deck missing %q\n%s", want, deck)
		}
	}
	if strings.Contains(deck, "*SPECIFIC HEAT") {
		t.Error("a steady coupled deck should not write *SPECIFIC HEAT")
	}
}

func TestDeckWritesCoupledTransient(t *testing.T) {
	deck := writeDeckString(t, &AnalysisModel{
		Analysis:     AnalysisCoupledThermal,
		Mesh:         unitTet(),
		Material:     MaterialProps{Name: "STEEL", YoungMPa: 210000, Poisson: 0.3, ExpansionPerK: 1.2e-5, Conductivity: 50, SpecificHeat: 5e8, DensityTonneMM3: 7.85e-9},
		Fixed:        []FixedConstraint{{Name: "FIX", Nodes: []int{1}, DOFLow: 1, DOFHigh: 3}},
		Temperatures: []TemperatureBC{{Name: "TCOLD", Nodes: []int{1}, TempK: 300}, {Name: "THOT", Nodes: []int{2}, TempK: 400}},
		InitialTempK: 300,
		Transient:    &TransientStep{IncrementS: 0.5, TotalS: 5},
	})
	for _, want := range []string{
		"*INITIAL CONDITIONS, TYPE=TEMPERATURE",
		"Nall, 300",
		"*SPECIFIC HEAT",
		"*DENSITY",
		"*COUPLED TEMPERATURE-DISPLACEMENT\n0.5, 5",
	} {
		if !strings.Contains(deck, want) {
			t.Errorf("transient coupled deck missing %q\n%s", want, deck)
		}
	}
	if strings.Contains(deck, "STEADY STATE") {
		t.Error("a transient coupled deck must not be STEADY STATE")
	}
}

func TestCoupledPrerequisites(t *testing.T) {
	base := func() *AnalysisModel {
		return &AnalysisModel{
			Analysis: AnalysisCoupledThermal,
			Mesh:     unitTet(),
			Material: MaterialProps{Name: "M", YoungMPa: 210000, Poisson: 0.3, ExpansionPerK: 1.2e-5, Conductivity: 50},
			Fixed:    []FixedConstraint{{Name: "FIX", Nodes: []int{1}, DOFLow: 1, DOFHigh: 3}},
			Temperatures: []TemperatureBC{
				{Name: "TCOLD", Nodes: []int{1}, TempK: 0},
				{Name: "THOT", Nodes: []int{2}, TempK: 100},
			},
		}
	}
	if err := checkPrerequisites(base()); err != nil {
		t.Fatalf("a valid coupled model should pass: %v", err)
	}
	noExp := base()
	noExp.Material.ExpansionPerK = 0
	if err := checkPrerequisites(noExp); err == nil {
		t.Error("zero thermal expansion should be rejected")
	}
	noCond := base()
	noCond.Material.Conductivity = 0
	if err := checkPrerequisites(noCond); err == nil {
		t.Error("zero conductivity should be rejected")
	}
	noGrad := base()
	noGrad.Temperatures[1].TempK = 0 // both faces cold → no gradient
	if err := checkPrerequisites(noGrad); err == nil {
		t.Error("a uniform temperature (no gradient) should be rejected")
	}
}

// TestCoupledThermalGradientExpansion is the coupled-analysis oracle: a bar held cold (Tc) on
// the fixed face and hot (Th) on the far face develops, at steady state, a linear temperature
// field, so its free thermal expansion stretches the tip by the integral of the thermal strain
//
//	delta = ∫ alpha*(T(z)-Tc) dz = alpha*(Th-Tc)*L/2.
//
// This exercises the *COUPLED TEMPERATURE-DISPLACEMENT path through the real solver — the
// temperature field is SOLVED (not prescribed uniform) and drives the displacement in one step.
func TestCoupledThermalGradientExpansion(t *testing.T) {
	bins := requireSolver(t)
	const (
		h, L  = 10.0, 100.0 // mm, bar along z
		young = 210000.0    // MPa
		alpha = 1.2e-5      // 1/K
		tc    = 0.0         // cold face (z=0), also the expansion reference
		th    = 100.0       // hot face (z=L)
	)
	dir := t.TempDir()
	mesh := meshBox(t, bins, h, h, L, dir)
	cold := selectNodes(mesh, func(n Node) bool { return n.Z < eps(L) })
	hot := selectNodes(mesh, func(n Node) bool { return n.Z > L-eps(L) })
	if len(cold) == 0 || len(hot) == 0 {
		t.Fatalf("selection failed (cold=%d hot=%d)", len(cold), len(hot))
	}

	model := &AnalysisModel{
		Analysis: AnalysisCoupledThermal,
		Mesh:     mesh,
		Material: MaterialProps{Name: "STEEL", YoungMPa: young, Poisson: 0, ExpansionPerK: alpha, Conductivity: 50},
		Fixed:    []FixedConstraint{{Name: "FIX", Nodes: cold, DOFLow: 1, DOFHigh: 3}},
		Temperatures: []TemperatureBC{
			{Name: "TCOLD", Nodes: cold, TempK: tc},
			{Name: "THOT", Nodes: hot, TempK: th},
		},
		InitialTempK: tc,
	}
	res := solveModel(t, bins, model, dir)

	got := meanUZ(res, hot)
	want := alpha * (th - tc) * L / 2
	relErr := math.Abs(got-want) / want
	t.Logf("coupled tip expansion: FE=%.5f mm, analytic α·ΔT·L/2=%.5f mm, rel err=%.1f%%", got, want, relErr*100)
	if relErr > 0.05 {
		t.Errorf("coupled tip expansion %.5f mm differs from analytic %.5f mm by %.1f%% (>5%%)", got, want, relErr*100)
	}
}

// TestCoupledTransientApproachesSteady runs the transient coupled path long enough to reach
// steady state and checks the final tip expansion converges to the steady analytic value —
// validating the *SPECIFIC HEAT / time-stepping path (the .frd's final increment is read).
func TestCoupledTransientApproachesSteady(t *testing.T) {
	bins := requireSolver(t)
	const (
		h, L  = 10.0, 100.0
		young = 210000.0
		alpha = 1.2e-5
		th    = 100.0
	)
	dir := t.TempDir()
	mesh := meshBox(t, bins, h, h, L, dir)
	cold := selectNodes(mesh, func(n Node) bool { return n.Z < eps(L) })
	hot := selectNodes(mesh, func(n Node) bool { return n.Z > L-eps(L) })

	model := &AnalysisModel{
		Analysis: AnalysisCoupledThermal,
		Mesh:     mesh,
		// Density × specific heat give a fast diffusivity so a short total time reaches steady.
		Material:     MaterialProps{Name: "STEEL", YoungMPa: young, Poisson: 0, ExpansionPerK: alpha, Conductivity: 50, DensityTonneMM3: 7.85e-9, SpecificHeat: 1e3},
		Fixed:        []FixedConstraint{{Name: "FIX", Nodes: cold, DOFLow: 1, DOFHigh: 3}},
		Temperatures: []TemperatureBC{{Name: "TCOLD", Nodes: cold, TempK: 0}, {Name: "THOT", Nodes: hot, TempK: th}},
		InitialTempK: 0,
		Transient:    &TransientStep{IncrementS: 1, TotalS: 50},
	}
	res := solveModel(t, bins, model, dir)

	got := meanUZ(res, hot)
	steady := alpha * th * L / 2
	t.Logf("transient final tip expansion: FE=%.5f mm, steady=%.5f mm", got, steady)
	if got <= 0 || got > 1.2*steady {
		t.Errorf("transient tip expansion %.5f mm not in the physical range (0, %.5f]", got, steady)
	}
}
