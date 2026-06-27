// SPDX-License-Identifier: GPL-2.0-only

package ccx

import (
	"math"
	"strings"
	"testing"
)

// TestHydrostaticPressuresDepthLinear checks the per-face pressure grows linearly with depth and
// is clamped to zero above the free surface.
func TestHydrostaticPressuresDepthLinear(t *testing.T) {
	// Two tets stacked in z; their P1 faces (corners 0-1-2) sit at z=0 and z=10.
	mesh := &TetMesh{
		Nodes: []Node{
			{ID: 1, Z: 0}, {ID: 2, Z: 0}, {ID: 3, Z: 0}, {ID: 4, Z: 5},
			{ID: 5, Z: 10}, {ID: 6, Z: 10}, {ID: 7, Z: 10}, {ID: 8, Z: 15},
		},
		Elements: []TetElement{
			{ID: 1, Nodes: []int{1, 2, 3, 4}},
			{ID: 2, Nodes: []int{5, 6, 7, 8}},
		},
	}
	const gradient, surface = 0.01, 8.0 // surface at z=8, below the upper face
	p := hydrostaticPressures(mesh, []ElemFace{{Elem: 1, Face: 1}, {Elem: 2, Face: 1}}, gradient, surface)
	if math.Abs(p[0]-gradient*(surface-0)) > 1e-9 {
		t.Errorf("bottom face pressure = %.5f, want %.5f", p[0], gradient*surface)
	}
	if p[1] != 0 {
		t.Errorf("face above the free surface must have zero pressure, got %.5f", p[1])
	}
}

func TestHydrostaticDeckPerFacePressure(t *testing.T) {
	mesh := &TetMesh{
		Nodes:    []Node{{ID: 1}, {ID: 2}, {ID: 3}, {ID: 4}},
		Elements: []TetElement{{ID: 1, Nodes: []int{1, 2, 3, 4}}},
	}
	deck := writeDeckString(t, &AnalysisModel{
		Analysis: AnalysisStatic,
		Mesh:     mesh,
		Material: MaterialProps{Name: "STEEL", YoungMPa: 210000, Poisson: 0.3},
		Fixed:    []FixedConstraint{{Name: "FIX", Nodes: []int{1}, DOFLow: 1, DOFHigh: 3}},
		Pressures: []PressureLoad{{Name: "LOAD",
			Faces:      []ElemFace{{Elem: 7, Face: 1}, {Elem: 9, Face: 2}},
			PerFaceMPa: []float64{0.5, 0.25}}},
	})
	for _, want := range []string{"*DLOAD", "7, P1, 0.5", "9, P2, 0.25"} {
		if !strings.Contains(deck, want) {
			t.Errorf("hydrostatic deck missing %q\n%s", want, deck)
		}
	}
}

// TestHydrostaticWallReaction is the hydrostatic oracle: a wall submerged below a free surface
// feels a fluid pressure that grows linearly with depth, so the total horizontal force it must
// react is the integral of that triangular pressure profile,
//
//	F = γ · w · H² / 2
//
// (γ the pressure gradient ρg, w the wall width, H its submerged height). The fixed base reacts
// exactly that force. This validates the per-face hydrostatic pressure end to end through the
// real vendored ccx (and the reaction read-back).
func TestHydrostaticWallReaction(t *testing.T) {
	bins := requireSolver(t)
	const (
		w, depth = 10.0, 20.0 // mm: a 10-wide wall, 20 tall (z = 0..20)
		gradient = 0.01       // MPa/mm (ρg)
	)
	dir := t.TempDir()
	mesh := meshBox(t, bins, w, w, depth, dir)

	// Fix the −x back face and load the +x front face: the two are disjoint, so no loaded node is
	// also constrained — keeping the support-reaction total exactly the applied pressure force.
	back := selectNodes(mesh, func(n Node) bool { return n.X < eps(w) })
	wall := elemFacesAt(mesh, func(n Node) bool { return n.X > w-eps(w) }) // the +x face, spanning z=0..depth
	if len(back) == 0 || len(wall) == 0 {
		t.Fatalf("selection failed (back=%d wall=%d)", len(back), len(wall))
	}
	model := &AnalysisModel{
		Analysis: AnalysisStatic,
		Mesh:     mesh,
		Material: MaterialProps{Name: "STEEL", YoungMPa: 210000, Poisson: 0.3},
		Fixed:    []FixedConstraint{{Name: "FIX", Nodes: back, DOFLow: 1, DOFHigh: 3}},
		Pressures: []PressureLoad{{Name: "WALL", Faces: wall,
			PerFaceMPa: hydrostaticPressures(mesh, wall, gradient, depth)}}, // surface at the top (z=depth)
	}
	stem, err := runDeck(bins, model, dir)
	if err != nil {
		t.Fatalf("solve: %v", err)
	}
	got := readReaction(stem + ".dat")
	want := gradient * w * depth * depth / 2 // ∫₀^H γ(H−z)·w dz = γ·w·H²/2
	relErr := math.Abs(got-want) / want
	t.Logf("hydrostatic wall reaction: FE=%.4f N, analytic γ·w·H²/2=%.4f N, rel err=%.2f%%", got, want, relErr*100)
	if relErr > 0.01 {
		t.Errorf("reaction %.4f N differs from γ·w·H²/2 %.4f N by %.2f%% (>1%%)", got, want, relErr*100)
	}
}
