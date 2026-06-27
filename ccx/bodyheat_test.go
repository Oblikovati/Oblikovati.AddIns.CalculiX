// SPDX-License-Identifier: GPL-2.0-only

package ccx

import (
	"math"
	"strings"
	"testing"
)

func TestDeckWritesBodyHeat(t *testing.T) {
	deck := writeDeckString(t, &AnalysisModel{
		Analysis:     AnalysisHeatTransfer,
		Mesh:         unitTet(),
		Material:     MaterialProps{Name: "STEEL", Conductivity: 50},
		Temperatures: []TemperatureBC{{Name: "T0", Nodes: []int{1}, TempK: 0}},
		BodyHeat:     &BodyHeat{Rate: 2.5},
	})
	for _, want := range []string{"*HEAT TRANSFER, STEADY STATE", "*CONDUCTIVITY", "*DFLUX", "Eall, BF, 2.5"} {
		if !strings.Contains(deck, want) {
			t.Errorf("body-heat deck missing %q\n%s", want, deck)
		}
	}
}

// TestBodyHeatGeneratingSlab is the body-heat-source oracle: a slab of thickness L with both
// faces held at T0 and a uniform internal generation q”' develops, at steady state, a
// parabolic temperature peaking at the centre,
//
//	T_max = T0 + q'''*L^2 / (8*k).
//
// Runs through the real ccx via the *DLOAD BF (volumetric) path.
func TestBodyHeatGeneratingSlab(t *testing.T) {
	bins := requireSolver(t)
	const (
		h, L = 10.0, 100.0 // mm, slab along z
		k    = 50.0        // conductivity (consistent units)
		q    = 1.0         // volumetric generation
		t0   = 0.0         // both faces held here
	)
	dir := t.TempDir()
	mesh := meshBox(t, bins, h, h, L, dir)
	ends := selectNodes(mesh, func(n Node) bool { return n.Z < eps(L) || n.Z > L-eps(L) })
	mid := selectNodes(mesh, func(n Node) bool { return math.Abs(n.Z-L/2) < 0.05*L })
	if len(ends) == 0 || len(mid) == 0 {
		t.Fatalf("selection failed (ends=%d mid=%d)", len(ends), len(mid))
	}

	model := &AnalysisModel{
		Analysis:     AnalysisHeatTransfer,
		Mesh:         mesh,
		Material:     MaterialProps{Name: "STEEL", Conductivity: k},
		Temperatures: []TemperatureBC{{Name: "ENDS", Nodes: ends, TempK: t0}},
		BodyHeat:     &BodyHeat{Rate: q},
	}
	temps := solveHeat(t, bins, model, dir)

	got := meanTemp(temps, mid)
	want := t0 + q*L*L/(8*k)
	relErr := math.Abs(got-want) / want
	t.Logf("generating-slab centre temperature: FE=%.3f, analytic q'''L²/8k=%.3f, rel err=%.1f%%", got, want, relErr*100)
	if relErr > 0.02 {
		t.Errorf("centre temperature %.3f differs from analytic %.3f by %.1f%% (>2%%)", got, want, relErr*100)
	}
}
