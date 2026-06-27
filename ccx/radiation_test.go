// SPDX-License-Identifier: GPL-2.0-only

package ccx

import (
	"math"
	"strings"
	"testing"
)

func TestDeckWritesRadiation(t *testing.T) {
	deck := writeDeckString(t, &AnalysisModel{
		Analysis:     AnalysisHeatTransfer,
		Mesh:         unitTet(),
		Material:     MaterialProps{Name: "STEEL", Conductivity: 50},
		Temperatures: []TemperatureBC{{Name: "HOT", Nodes: []int{1}, TempK: 800}},
		Radiations:   []RadiationBC{{Name: "RAD", Faces: []ElemFace{{Elem: 1, Face: 3}}, Emissivity: 0.8, AmbientK: 300}},
	})
	for _, want := range []string{
		"*PHYSICAL CONSTANTS, ABSOLUTE ZERO=0, STEFAN BOLTZMANN=5.67e-11",
		"*HEAT TRANSFER, STEADY STATE",
		"*RADIATE",
		"1, R3, 300, 0.8", // element, face, ambient temp, emissivity
	} {
		if !strings.Contains(deck, want) {
			t.Errorf("radiation deck missing %q\n%s", want, deck)
		}
	}
	// Physical constants must precede the step.
	if strings.Index(deck, "*PHYSICAL CONSTANTS") > strings.Index(deck, "*STEP") {
		t.Error("*PHYSICAL CONSTANTS must be written before *STEP")
	}
}

// TestRadiationConductionBalance is the radiation oracle: a bar held at T_hot on one face and
// radiating to an ambient T_amb on the far face reaches, at steady state, a far-face
// temperature T_L where conduction in equals radiation out,
//
//	k*(T_hot - T_L)/L = ε*σ*(T_L^4 - T_amb^4).
//
// The analytic T_L (the root of that transcendental balance, found by Newton here) is compared
// against the FE result. Runs through the real ccx via the *RADIATE / *PHYSICAL CONSTANTS path
// (a nonlinear heat solve, seeded by the initial temperature).
func TestRadiationConductionBalance(t *testing.T) {
	bins := requireSolver(t)
	const (
		w, L = 10.0, 100.0 // mm, slender bar along z
		k    = 5.0         // low conductivity, so radiation noticeably cools the far face
		emis = 1.0
		tHot = 800.0
		tAmb = 300.0
	)
	dir := t.TempDir()
	mesh := meshBox(t, bins, w, w, L, dir)
	hot := selectNodes(mesh, func(n Node) bool { return n.Z < eps(L) })
	radFaces := elemFacesAt(mesh, func(n Node) bool { return n.Z > L-eps(L) })
	if len(hot) == 0 || len(radFaces) == 0 {
		t.Fatalf("selection failed (hot=%d radFaces=%d)", len(hot), len(radFaces))
	}

	model := &AnalysisModel{
		Analysis:     AnalysisHeatTransfer,
		Mesh:         mesh,
		Material:     MaterialProps{Name: "STEEL", Conductivity: k},
		Temperatures: []TemperatureBC{{Name: "HOT", Nodes: hot, TempK: tHot}},
		Radiations:   []RadiationBC{{Name: "RAD", Faces: radFaces, Emissivity: emis, AmbientK: tAmb}},
		InitialTempK: tHot, // seed the nonlinear T^4 solve
	}
	temps := solveHeat(t, bins, model, dir)

	cool := selectNodes(mesh, func(n Node) bool { return n.Z > L-eps(L) })
	got := meanTemp(temps, cool)
	want := radiationFarTemp(k, L, emis, stefanBoltzmannConsistent, tHot, tAmb)
	relErr := math.Abs(got-want) / want
	t.Logf("radiating-face temperature: FE=%.2f K, balance root=%.2f K, rel err=%.1f%%", got, want, relErr*100)
	if relErr > 0.05 {
		t.Errorf("radiating-face temperature %.2f K differs from analytic %.2f K by %.1f%% (>5%%)", got, want, relErr*100)
	}
}

// radiationFarTemp solves k*(tHot-T)/L = emis*sigma*(T^4 - tAmb^4) for T by Newton iteration.
func radiationFarTemp(k, l, emis, sigma, tHot, tAmb float64) float64 {
	tAmb4 := tAmb * tAmb * tAmb * tAmb
	f := func(t float64) float64 { return k*(tHot-t)/l - emis*sigma*(t*t*t*t-tAmb4) }
	df := func(t float64) float64 { return -k/l - 4*emis*sigma*t*t*t }
	tg := (tHot + tAmb) / 2
	for i := 0; i < 100; i++ {
		tg -= f(tg) / df(tg)
	}
	return tg
}
