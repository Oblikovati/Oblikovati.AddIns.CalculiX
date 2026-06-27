// SPDX-License-Identifier: GPL-2.0-only

package ccx

import (
	"strings"
	"testing"
)

// unitTet is a single-element mesh for fast, solver-free deck-text checks.
func unitTet() *TetMesh {
	return &TetMesh{
		Nodes:    []Node{{ID: 1}, {ID: 2, X: 1}, {ID: 3, Y: 1}, {ID: 4, Z: 1}},
		Elements: []TetElement{{ID: 1, Nodes: []int{1, 2, 3, 4}}},
	}
}

func writeDeckString(t *testing.T, m *AnalysisModel) string {
	t.Helper()
	var b strings.Builder
	if err := WriteDeck(&b, m); err != nil {
		t.Fatalf("WriteDeck: %v", err)
	}
	return b.String()
}

func TestDeckWritesPressureCard(t *testing.T) {
	deck := writeDeckString(t, &AnalysisModel{
		Analysis:  AnalysisStatic,
		Mesh:      unitTet(),
		Material:  MaterialProps{Name: "STEEL", YoungMPa: 210000, Poisson: 0.3},
		Fixed:     []FixedConstraint{{Name: "FIX", Nodes: []int{1}, DOFLow: 1, DOFHigh: 3}},
		Pressures: []PressureLoad{{Name: "P", Faces: []ElemFace{{Elem: 1, Face: 2}}, MPa: 5}},
	})
	for _, want := range []string{"*DLOAD", "1, P2, 5"} {
		if !strings.Contains(deck, want) {
			t.Errorf("deck missing %q\n%s", want, deck)
		}
	}
	if strings.Contains(deck, "*DENSITY") {
		t.Error("pressure-only study should not write *DENSITY")
	}
}

func TestDeckWritesGravityAndDensity(t *testing.T) {
	deck := writeDeckString(t, &AnalysisModel{
		Analysis: AnalysisStatic,
		Mesh:     unitTet(),
		Material: MaterialProps{Name: "STEEL", YoungMPa: 210000, Poisson: 0.3, DensityTonneMM3: 7.9e-9},
		Fixed:    []FixedConstraint{{Name: "FIX", Nodes: []int{1}, DOFLow: 1, DOFHigh: 3}},
		Gravity:  &GravityLoad{Accel: 9810, Dir: [3]float64{0, 0, -1}},
	})
	for _, want := range []string{"*DENSITY", "*DLOAD", "Eall, GRAV, 9810"} {
		if !strings.Contains(deck, want) {
			t.Errorf("deck missing %q\n%s", want, deck)
		}
	}
}

func TestFaceElemIndexResolvesEachFace(t *testing.T) {
	// A single tet exposes its four faces; each boundary-triangle corner triple must
	// resolve to that element with the right CalculiX face number.
	mesh := unitTet()
	index := faceElemIndex(mesh)
	if len(index) != 4 {
		t.Fatalf("face index has %d entries, want 4", len(index))
	}
	got := resolveElemFaces([][3]int{{1, 2, 3}}, index) // P1 = nodes 1-2-3
	if len(got) != 1 || got[0] != (ElemFace{Elem: 1, Face: 1}) {
		t.Errorf("resolveElemFaces = %v, want [{1 1}]", got)
	}
}
