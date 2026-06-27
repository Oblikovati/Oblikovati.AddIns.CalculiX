// SPDX-License-Identifier: GPL-2.0-only

package ccx

import (
	"math"
	"strings"
	"testing"
)

func TestDeckWritesFilm(t *testing.T) {
	deck := writeDeckString(t, &AnalysisModel{
		Analysis:     AnalysisHeatTransfer,
		Mesh:         unitTet(),
		Material:     MaterialProps{Name: "STEEL", Conductivity: 50},
		Temperatures: []TemperatureBC{{Name: "HOT", Nodes: []int{1}, TempK: 100}},
		Films:        []FilmBC{{Name: "COOL", Faces: []ElemFace{{Elem: 1, Face: 2}}, Coeff: 0.5, SinkTempK: 20}},
	})
	for _, want := range []string{
		"*HEAT TRANSFER, STEADY STATE",
		"*CONDUCTIVITY",
		"*FILM",
		"1, F2, 20, 0.5", // element, face, sink temp, film coefficient
	} {
		if !strings.Contains(deck, want) {
			t.Errorf("film deck missing %q\n%s", want, deck)
		}
	}
	if strings.Contains(deck, "*DFLUX") {
		t.Error("a convection deck should not write a *DFLUX flux load")
	}
}

// TestConvectionSurfaceResistance is the convective-film oracle: a bar held at T_hot on one
// face and cooled by convection (coefficient h to a sink T_sink) on the far face reaches, at
// steady state, a convecting-face temperature governed by the series of the conduction and
// film resistances,
//
//	T_c = T_sink + (T_hot - T_sink) / (1 + h*L/k).
//
// Runs through the real ccx via the *FILM path; the conductivity now matters (vs a fixed-flux
// boundary, where the surface temperature does not depend on k).
func TestConvectionSurfaceResistance(t *testing.T) {
	bins := requireSolver(t)
	const (
		h, L  = 10.0, 100.0 // mm, bar along z
		k     = 50.0        // conductivity (consistent units)
		film  = 0.5         // film coefficient h
		tHot  = 100.0       // hot face (z=0)
		tSink = 0.0         // ambient
	)
	dir := t.TempDir()
	mesh := meshBox(t, bins, h, h, L, dir)
	hot := selectNodes(mesh, func(n Node) bool { return n.Z < eps(L) })
	coolFaces := elemFacesAt(mesh, func(n Node) bool { return n.Z > L-eps(L) })
	if len(hot) == 0 || len(coolFaces) == 0 {
		t.Fatalf("selection failed (hot=%d coolFaces=%d)", len(hot), len(coolFaces))
	}

	model := &AnalysisModel{
		Analysis:     AnalysisHeatTransfer,
		Mesh:         mesh,
		Material:     MaterialProps{Name: "STEEL", Conductivity: k},
		Temperatures: []TemperatureBC{{Name: "HOT", Nodes: hot, TempK: tHot}},
		Films:        []FilmBC{{Name: "COOL", Faces: coolFaces, Coeff: film, SinkTempK: tSink}},
	}
	temps := solveHeat(t, bins, model, dir)

	cool := selectNodes(mesh, func(n Node) bool { return n.Z > L-eps(L) })
	got := meanTemp(temps, cool)
	want := tSink + (tHot-tSink)/(1+film*L/k)
	relErr := math.Abs(got-want) / want
	t.Logf("convecting-face temperature: FE=%.3f, analytic T_sink+(ΔT)/(1+hL/k)=%.3f, rel err=%.1f%%", got, want, relErr*100)
	if relErr > 0.02 {
		t.Errorf("convecting-face temperature %.3f differs from analytic %.3f by %.1f%% (>2%%)", got, want, relErr*100)
	}
}
