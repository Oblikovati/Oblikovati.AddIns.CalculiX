// SPDX-License-Identifier: GPL-2.0-only

package ccx

import (
	"math"
	"strings"
	"testing"
)

func TestDominantAxis(t *testing.T) {
	cases := []struct {
		v   [3]float64
		dof int
	}{
		{[3]float64{1, 0, 0}, 1}, {[3]float64{0, -1, 0}, 2}, {[3]float64{0, 0, 1}, 3},
		{[3]float64{0.1, 0.9, 0.2}, 2}, {[3]float64{-0.95, 0.3, 0}, 1},
	}
	for _, c := range cases {
		if got := dominantAxis(c.v); got != c.dof {
			t.Errorf("dominantAxis(%v) = %d, want %d", c.v, got, c.dof)
		}
	}
}

// TestRollerResolvesToNormalDOF checks a roller fixes only the single global DOF closest to the
// face normal, and emits a *BOUNDARY on that one DOF.
func TestRollerResolvesToNormalDOF(t *testing.T) {
	mesh := &TetMesh{
		Nodes:    []Node{{ID: 1}, {ID: 2}, {ID: 3}, {ID: 4}},
		Elements: []TetElement{{ID: 1, Nodes: []int{1, 2, 3, 4}}},
	}
	groups := &FaceGroups{
		Nodes:   map[string][]int{"top": {1, 2}},
		Normals: map[string][3]float64{"top": {0, 0, 1}},
	}
	m := &AnalysisModel{Analysis: AnalysisStatic, Mesh: mesh,
		Material: MaterialProps{Name: "M", YoungMPa: 210000, Poisson: 0.3}}
	RollerSpec{Name: "ROLL", Faces: []string{"top"}}.Resolve(&ResolveContext{Model: m, Mesh: mesh, Groups: groups})
	if len(m.Fixed) != 1 || m.Fixed[0].DOFLow != 3 || m.Fixed[0].DOFHigh != 3 {
		t.Fatalf("roller on a +z face should fix DOF 3 only, got %+v", m.Fixed)
	}
	deck := writeDeckString(t, m)
	if !strings.Contains(deck, "ROLL, 3, 3") {
		t.Errorf("roller deck should emit *BOUNDARY on DOF 3 only\n%s", deck)
	}
}

// TestSymmetryRollerUniaxial is the roller/symmetry oracle: a box held by three symmetry rollers
// — x=0 (normal −x → fixes DOF1), y=0 (→DOF2), z=0 (→DOF3) — is one octant of free uniaxial
// tension. Stretched along z by a prescribed displacement, with the lateral faces free, it must
// develop the uniaxial stress σ_zz = E·ε. Crucially the model is built ENTIRELY through
// RollerSpec.Resolve (the DOF derived from each face's normal), so this validates that the
// roller constraint reproduces the hand-built symmetry support, end to end through the real ccx.
func TestSymmetryRollerUniaxial(t *testing.T) {
	bins := requireSolver(t)
	const (
		a, L  = 10.0, 20.0 // mm: a 10×10×20 column
		young = 210000.0   // MPa
		eps0  = 0.001      // axial strain
	)
	dir := t.TempDir()
	mesh := meshBox(t, bins, a, a, L, dir)

	groups := &FaceGroups{
		Nodes: map[string][]int{
			"x0":  selectNodes(mesh, func(n Node) bool { return n.X < eps(a) }),
			"y0":  selectNodes(mesh, func(n Node) bool { return n.Y < eps(a) }),
			"z0":  selectNodes(mesh, func(n Node) bool { return n.Z < eps(L) }),
			"top": selectNodes(mesh, func(n Node) bool { return n.Z > L-eps(L) }),
		},
		Normals: map[string][3]float64{
			"x0": {-1, 0, 0}, "y0": {0, -1, 0}, "z0": {0, 0, -1},
		},
	}
	for k, v := range groups.Nodes {
		if len(v) == 0 {
			t.Fatalf("face %s selected no nodes", k)
		}
	}
	model := &AnalysisModel{Analysis: AnalysisStatic, Mesh: mesh,
		Material: MaterialProps{Name: "STEEL", YoungMPa: young, Poisson: 0.3}}
	specs := []ConstraintSpec{
		RollerSpec{Name: "SYMX", Faces: []string{"x0"}},
		RollerSpec{Name: "SYMY", Faces: []string{"y0"}},
		RollerSpec{Name: "SYMZ", Faces: []string{"z0"}},
		DisplacementSpec{Name: "PULL", Faces: []string{"top"}, DOF: 3, Value: eps0 * L},
	}
	resolveSpecs(specs, &ResolveContext{Model: model, Mesh: mesh, Groups: groups})

	res := solveModel(t, bins, model, dir)
	mid := selectNodes(mesh, func(n Node) bool { return n.Z > L/2-eps(L) && n.Z < L/2+eps(L) })
	got := meanSzz(res, mid)
	want := young * eps0
	relErr := math.Abs(got-want) / want
	t.Logf("symmetry-roller uniaxial: FE σ_zz=%.4f MPa, analytic E·ε=%.4f MPa, rel err=%.2f%%", got, want, relErr*100)
	if relErr > 0.01 {
		t.Errorf("axial stress %.4f MPa differs from E·ε %.4f MPa by %.2f%% (>1%%)", got, want, relErr*100)
	}
}
