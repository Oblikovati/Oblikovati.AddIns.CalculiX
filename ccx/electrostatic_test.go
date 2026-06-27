// SPDX-License-Identifier: GPL-2.0-only

package ccx

import (
	"math"
	"strings"
	"testing"
)

func TestDeckWritesElectrostatic(t *testing.T) {
	deck := writeDeckString(t, &AnalysisModel{
		Analysis: AnalysisElectromagnetic,
		Mesh:     unitTet(),
		Material: MaterialProps{Name: "COPPER", YoungMPa: 117000, Poisson: 0.34, ElectricalSigma: 6e4},
		Temperatures: []TemperatureBC{
			{Name: "VHIGH", Nodes: []int{1}, TempK: 5},
			{Name: "VGND", Nodes: []int{2}, TempK: 0},
		},
	})
	for _, want := range []string{
		"*CONDUCTIVITY",
		"60000", // the electrical conductivity, not the elastic constants
		"*HEAT TRANSFER, STEADY STATE",
		"VHIGH, 11, 11, 5",
		"VGND, 11, 11, 0",
		"NT",
	} {
		if !strings.Contains(deck, want) {
			t.Errorf("electrostatic deck missing %q\n%s", want, deck)
		}
	}
	if strings.Contains(deck, "*ELASTIC") {
		t.Error("an electrostatic deck should not write *ELASTIC")
	}
	if strings.Contains(deck, "*DFLUX") {
		t.Error("a two-Dirichlet electrostatic deck should not write a flux load")
	}
}

func TestElectrostaticPrerequisites(t *testing.T) {
	base := func() *AnalysisModel {
		return &AnalysisModel{
			Analysis: AnalysisElectromagnetic,
			Mesh:     unitTet(),
			Material: MaterialProps{ElectricalSigma: 1},
			Temperatures: []TemperatureBC{
				{Name: "VHIGH", Nodes: []int{1}, TempK: 5},
				{Name: "VGND", Nodes: []int{2}, TempK: 0},
			},
		}
	}
	if err := checkPrerequisites(base()); err != nil {
		t.Fatalf("a valid electrostatic model should pass: %v", err)
	}

	noSigma := base()
	noSigma.Material.ElectricalSigma = 0
	if err := checkPrerequisites(noSigma); err == nil {
		t.Error("zero electrical conductivity should be rejected")
	}

	noDrop := base()
	for i := range noDrop.Temperatures {
		noDrop.Temperatures[i].TempK = 0 // grounded everywhere → trivial field
	}
	if err := checkPrerequisites(noDrop); err == nil {
		t.Error("a zero applied voltage (no potential difference) should be rejected")
	}
}

// TestElectrostaticPotentialLinear validates the electric-conduction path against the
// analytic Laplace solution: a bar held at V0 on one end face and grounded on the other has
// a potential that falls linearly along its length, so the mid-plane sits at V0/2 and the
// field spans [0, V0]. This rides the real vendored ccx solver through the heat-transfer
// analogy (potential = DOF 11).
func TestElectrostaticPotentialLinear(t *testing.T) {
	bins := requireSolver(t)
	const (
		h, L  = 10.0, 100.0 // mm, bar along z
		sigma = 5.0e4       // electrical conductivity (consistent units; irrelevant to potential)
		v0    = 5.0         // applied potential (V)
	)
	dir := t.TempDir()
	mesh := meshBox(t, bins, h, h, L, dir)
	high := selectNodes(mesh, func(n Node) bool { return n.Z < eps(L) })
	ground := selectNodes(mesh, func(n Node) bool { return n.Z > L-eps(L) })
	if len(high) == 0 || len(ground) == 0 {
		t.Fatalf("selection failed (high=%d ground=%d)", len(high), len(ground))
	}

	model := &AnalysisModel{
		Analysis: AnalysisElectromagnetic,
		Mesh:     mesh,
		Material: MaterialProps{Name: "COPPER", ElectricalSigma: sigma},
		Temperatures: []TemperatureBC{
			{Name: "VHIGH", Nodes: high, TempK: v0},
			{Name: "VGND", Nodes: ground, TempK: 0},
		},
	}
	pot := solveHeat(t, bins, model, dir) // NT field = electric potential

	lo, hi := minMaxField(pot)
	if math.Abs(lo) > 1e-3 || math.Abs(hi-v0) > 0.05*v0 {
		t.Errorf("potential range = %.4g..%.4g V, want 0..%.4g V", lo, hi, v0)
	}

	mid := selectNodes(mesh, func(n Node) bool { return math.Abs(n.Z-L/2) < eps(L) })
	if len(mid) == 0 {
		t.Fatal("no mid-plane nodes found")
	}
	got := meanTemp(pot, mid)
	want := v0 / 2
	relErr := math.Abs(got-want) / want
	t.Logf("mid-plane potential: FE=%.4f V, analytic V0/2=%.4f V, rel err=%.1f%%", got, want, relErr*100)
	if relErr > 0.02 {
		t.Errorf("mid-plane potential %.4f V differs from analytic %.4f V by %.1f%% (>2%%)", got, want, relErr*100)
	}
}
