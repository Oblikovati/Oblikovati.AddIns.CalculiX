// SPDX-License-Identifier: GPL-2.0-only

package ccx

import (
	"math"
	"strings"
	"testing"
)

func TestCoincidentFaces(t *testing.T) {
	const tol = 1.0
	top := tieGroup{centroid: [3]float64{5, 5, 50}, normal: [3]float64{0, 0, 1}}
	bottom := tieGroup{centroid: [3]float64{5, 5, 50}, normal: [3]float64{0, 0, -1}}
	if !coincidentFaces(top, bottom, tol) {
		t.Error("coincident anti-parallel faces should bond")
	}
	// Same place but same-direction normals (not a shared interface).
	if coincidentFaces(top, tieGroup{centroid: top.centroid, normal: top.normal}, tol) {
		t.Error("parallel-normal faces must not bond")
	}
	// Anti-parallel but a body-length apart (the assembly's two outer ends).
	far := tieGroup{centroid: [3]float64{5, 5, 0}, normal: [3]float64{0, 0, -1}}
	if coincidentFaces(top, far, tol) {
		t.Error("distant anti-parallel faces must not bond")
	}
}

func TestWriteTiesDeck(t *testing.T) {
	mesh := &TetMesh{
		Nodes:    []Node{{ID: 1}, {ID: 2}, {ID: 3}, {ID: 4}},
		Elements: []TetElement{{ID: 1, Nodes: []int{1, 2, 3, 4}}},
	}
	deck := writeDeckString(t, &AnalysisModel{
		Analysis: AnalysisStatic,
		Mesh:     mesh,
		Material: MaterialProps{Name: "STEEL", YoungMPa: 210000, Poisson: 0.3},
		Ties: []TieConstraint{{
			Name:   "TIE0",
			Slave:  []ElemFace{{Elem: 7, Face: 1}},
			Master: []ElemFace{{Elem: 9, Face: 4}},
		}},
	})
	for _, want := range []string{
		"*SURFACE, NAME=TIE0_S, TYPE=ELEMENT",
		"7, S1",
		"*SURFACE, NAME=TIE0_M, TYPE=ELEMENT",
		"9, S4",
		"*TIE, NAME=TIE0",
		"TIE0_S, TIE0_M",
	} {
		if !strings.Contains(deck, want) {
			t.Errorf("tie deck missing %q\n%s", want, deck)
		}
	}
	// The *TIE must precede the *STEP (it is a model-level constraint).
	if strings.Index(deck, "*TIE") > strings.Index(deck, "*STEP") {
		t.Error("*TIE must be written before *STEP")
	}
}

// translateMeshZ shifts every node of a mesh by dz, so two boxes meshed at the origin can be
// stacked into a two-body assembly.
func translateMeshZ(mesh *TetMesh, dz float64) *TetMesh {
	out := &TetMesh{Elements: mesh.Elements, Surface: mesh.Surface}
	out.Nodes = make([]Node, len(mesh.Nodes))
	for i, n := range mesh.Nodes {
		out.Nodes[i] = Node{ID: n.ID, X: n.X, Y: n.Y, Z: n.Z + dz}
	}
	return out
}

// TestTiedBarMonolithic is the bonded-contact oracle: two boxes meshed SEPARATELY (so their
// shared interface is non-conformal) are stacked, merged, and bonded with a *TIE. Under an
// axial end load the tied assembly must extend like the monolithic bar it represents,
//
//	delta = P*L / (A*E)
//
// which only holds if the tie actually transmits load across the interface (without it the
// upper body is unconstrained and the solve is singular). This validates detectTies end to
// end through the real vendored ccx.
func TestTiedBarMonolithic(t *testing.T) {
	bins := requireSolver(t)
	const (
		h, half = 10.0, 50.0 // mm: two 10×10×50 boxes → a 10×10×100 bar
		L       = 2 * half
		young   = 210000.0 // MPa
		p       = 1000.0   // N axial load (+z) on the top face
	)
	dir := t.TempDir()
	lower := meshBox(t, bins, h, h, half, t.TempDir())
	upper := translateMeshZ(meshBox(t, bins, h, h, half, t.TempDir()), half)
	mesh := mergeTetMeshes([]*TetMesh{lower, upper})

	ties := detectTies(mesh)
	if len(ties) != 1 {
		t.Fatalf("detectTies found %d ties, want 1 (the stacked interface)", len(ties))
	}
	if len(ties[0].Slave) == 0 || len(ties[0].Master) == 0 {
		t.Fatalf("tie has empty surfaces: slave=%d master=%d", len(ties[0].Slave), len(ties[0].Master))
	}

	root := selectNodes(mesh, func(n Node) bool { return n.Z < eps(L) })
	tip := selectNodes(mesh, func(n Node) bool { return n.Z > L-eps(L) })
	if len(root) == 0 || len(tip) == 0 {
		t.Fatalf("selection failed (root=%d tip=%d)", len(root), len(tip))
	}
	model := &AnalysisModel{
		Analysis: AnalysisStatic,
		Mesh:     mesh,
		Material: MaterialProps{Name: "STEEL", YoungMPa: young, Poisson: 0},
		Fixed:    []FixedConstraint{{Name: "ROOT", Nodes: root, DOFLow: 1, DOFHigh: 3}},
		Forces:   []ForceLoad{{Name: "TIP", Nodes: tip, Dir: [3]float64{0, 0, 1}, TotalN: p}},
		Ties:     ties,
	}
	res := solveModel(t, bins, model, dir)

	got := meanUZ(res, tip)
	want := p * L / (h * h * young)
	relErr := math.Abs(got-want) / want
	t.Logf("tied-bar extension: FE=%.5f mm, monolithic P·L/AE=%.5f mm, rel err=%.1f%%", got, want, relErr*100)
	if relErr > 0.05 {
		t.Errorf("tied extension %.5f mm differs from monolithic %.5f mm by %.1f%% (>5%%)", got, want, relErr*100)
	}
}
