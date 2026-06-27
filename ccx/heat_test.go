// SPDX-License-Identifier: GPL-2.0-only

package ccx

import (
	"fmt"
	"math"
	"os"
	"strings"
	"testing"
)

func TestDeckWritesHeatTransfer(t *testing.T) {
	deck := writeDeckString(t, &AnalysisModel{
		Analysis:     AnalysisHeatTransfer,
		Mesh:         unitTet(),
		Material:     MaterialProps{Name: "STEEL", YoungMPa: 210000, Poisson: 0.3, Conductivity: 50},
		Temperatures: []TemperatureBC{{Name: "COLD", Nodes: []int{1}, TempK: 0}},
		HeatFluxes:   []HeatFlux{{Name: "HOT", Faces: []ElemFace{{Elem: 1, Face: 2}}, Flux: 50}},
	})
	for _, want := range []string{
		"*CONDUCTIVITY",
		"*HEAT TRANSFER, STEADY STATE",
		"COLD, 11, 11, 0",
		"*DFLUX",
		"1, S2, 50",
		"NT",
	} {
		if !strings.Contains(deck, want) {
			t.Errorf("deck missing %q\n%s", want, deck)
		}
	}
	if strings.Contains(deck, "*ELASTIC") {
		t.Error("a heat-transfer deck should not write *ELASTIC")
	}
}

// frdTempLine formats a fixed-width .frd NDTEMP record (" -1" + I10 id + E12.5 value).
func frdTempLine(id int, temp float64) string {
	return fmt.Sprintf(" -1%10d%12.5E", id, temp)
}

func TestParseNodalTemperatures(t *testing.T) {
	frd := strings.Join([]string{
		" -4  NDTEMP      1    1",
		" -5  T           1    1    0    0",
		frdTempLine(1, 0.0),
		frdTempLine(2, 100.0),
		" -3",
	}, "\n")
	temps, err := parseNodalTemperatures(strings.NewReader(frd))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if temps[2] != 100.0 || temps[1] != 0.0 {
		t.Errorf("temps = %v, want {1:0, 2:100}", temps)
	}
}

// solveHeat runs a heat-transfer deck and returns the nodal temperature field.
func solveHeat(t *testing.T, bins solverBinaries, model *AnalysisModel, dir string) map[int]float64 {
	t.Helper()
	stem, err := runDeck(bins, model, dir)
	if err != nil {
		t.Fatalf("solve: %v", err)
	}
	f, err := os.Open(stem + ".frd")
	if err != nil {
		t.Fatalf("open frd: %v", err)
	}
	defer f.Close()
	temps, err := parseNodalTemperatures(f)
	if err != nil {
		t.Fatalf("parse temperatures: %v", err)
	}
	return temps
}

// TestHeatTransferFourier validates the *HEAT TRANSFER path: a bar held at 0 K on one face
// and heated by a surface flux q on the far face reaches, at steady state, a hot-face
// temperature of q·L/k (Fourier's law).
func TestHeatTransferFourier(t *testing.T) {
	bins := requireSolver(t)
	const (
		h, L = 10.0, 100.0 // mm, bar along z
		k    = 50.0        // conductivity (consistent units)
		q    = 50.0        // surface heat flux
	)
	dir := t.TempDir()
	mesh := meshBox(t, bins, h, h, L, dir)
	cold := selectNodes(mesh, func(n Node) bool { return n.Z < eps(L) })
	hotFaces := elemFacesAt(mesh, func(n Node) bool { return n.Z > L-eps(L) })
	if len(cold) == 0 || len(hotFaces) == 0 {
		t.Fatalf("selection failed (cold=%d hotFaces=%d)", len(cold), len(hotFaces))
	}

	model := &AnalysisModel{
		Analysis:     AnalysisHeatTransfer,
		Mesh:         mesh,
		Material:     MaterialProps{Name: "STEEL", Conductivity: k},
		Temperatures: []TemperatureBC{{Name: "COLD", Nodes: cold, TempK: 0}},
		HeatFluxes:   []HeatFlux{{Name: "HOT", Faces: hotFaces, Flux: q}},
	}
	temps := solveHeat(t, bins, model, dir)

	hot := selectNodes(mesh, func(n Node) bool { return n.Z > L-eps(L) })
	got := meanTemp(temps, hot)
	want := q * L / k
	relErr := math.Abs(got-want) / want
	t.Logf("hot-face temperature: FE=%.3f, Fourier q·L/k=%.3f, rel err=%.1f%%", got, want, relErr*100)
	if relErr > 0.02 {
		t.Errorf("hot-face temperature %.3f differs from analytic %.3f by %.1f%% (>2%%)", got, want, relErr*100)
	}
}

// meanTemp returns the mean temperature over a node set.
func meanTemp(temps map[int]float64, nodes []int) float64 {
	sum := 0.0
	for _, id := range nodes {
		sum += temps[id]
	}
	return sum / float64(len(nodes))
}
