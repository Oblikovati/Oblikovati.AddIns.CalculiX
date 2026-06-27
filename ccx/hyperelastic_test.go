// SPDX-License-Identifier: GPL-2.0-only

package ccx

import (
	"math"
	"strings"
	"testing"
)

func TestDeckWritesHyperelastic(t *testing.T) {
	mesh := &TetMesh{
		Nodes:    []Node{{ID: 1}, {ID: 2}, {ID: 3}, {ID: 4}},
		Elements: []TetElement{{ID: 1, Nodes: []int{1, 2, 3, 4}}},
	}
	deck := writeDeckString(t, &AnalysisModel{
		Analysis: AnalysisStatic,
		Mesh:     mesh,
		Material: MaterialProps{Name: "RUBBER", Hyper: &NeoHooke{C10: 1.0, D1: 0.1}},
		Fixed:    []FixedConstraint{{Name: "FIX", Nodes: []int{1, 2}, DOFLow: 3, DOFHigh: 3}},
		Displacements: []DisplacementBC{
			{Name: "PULL", Nodes: []int{3, 4}, DOF: 3, Value: 2},
		},
	})
	for _, want := range []string{
		"*HYPERELASTIC, NEO HOOKE",
		"1, 0.1", // C10, D1
		"*STEP, NLGEOM",
		"*STATIC\n0.1, 1.0",
	} {
		if !strings.Contains(deck, want) {
			t.Errorf("hyperelastic deck missing %q\n%s", want, deck)
		}
	}
	// A hyperelastic material must NOT also write *ELASTIC.
	if strings.Contains(deck, "*ELASTIC") {
		t.Errorf("a Neo-Hookean material must not emit *ELASTIC\n%s", deck)
	}
}

// neoHookeUniaxialCauchy returns the exact axial Cauchy (true) stress of a compressible
// Neo-Hookean material (CalculiX U = C10(Ī1−3) + (1/D1)(J−1)²) at axial stretch lambda under
// uniaxial tension. The lateral stretch is found by driving the lateral Cauchy stress to zero
// with Newton's method, so the result is exact for any compressibility (no incompressible
// approximation) — the oracle the FE result is checked against.
func neoHookeUniaxialCauchy(lambda, c10, d1 float64) float64 {
	lt := 1 / math.Sqrt(lambda) // incompressible guess
	for i := 0; i < 100; i++ {
		f := neoCauchy(lambda, lt, c10, d1, false)
		const h = 1e-8
		df := (neoCauchy(lambda, lt+h, c10, d1, false) - f) / h
		if df == 0 {
			break
		}
		step := f / df
		lt -= step
		if math.Abs(step) < 1e-13 {
			break
		}
	}
	return neoCauchy(lambda, lt, c10, d1, true)
}

// neoCauchy returns the axial (axial=true) or lateral (axial=false) principal Cauchy stress for
// axial stretch lambda and lateral stretch lt.
func neoCauchy(lambda, lt, c10, d1 float64, axial bool) float64 {
	j := lambda * lt * lt
	j23 := math.Pow(j, -2.0/3.0)
	bAxial := j23 * lambda * lambda
	bLat := j23 * lt * lt
	i1bar := bAxial + 2*bLat
	b := bLat
	if axial {
		b = bAxial
	}
	return (2*c10/j)*(b-i1bar/3) + (2/d1)*(j-1)
}

// TestNeoHookeUniaxialIncompressibleLimit checks the oracle reduces to the textbook
// incompressible Neo-Hookean uniaxial stress σ = 2·C10·(λ² − 1/λ) as D1 → 0.
func TestNeoHookeUniaxialIncompressibleLimit(t *testing.T) {
	const c10, lambda = 1.0, 1.3
	got := neoHookeUniaxialCauchy(lambda, c10, 1e-6)
	want := 2 * c10 * (lambda*lambda - 1/lambda)
	if rel := math.Abs(got-want) / want; rel > 1e-3 {
		t.Errorf("incompressible limit: got %.5f, want %.5f (rel %.2g)", got, want, rel)
	}
}

// TestNeoHookeUniaxialStretch is the hyperelastic oracle: a Neo-Hookean (rubber) cube stretched
// uniaxially by a prescribed displacement develops the axial true stress predicted by the
// compressible Neo-Hookean uniaxial solution. Symmetry roller faces (x=0→u_x=0, y=0→u_y=0,
// z=0→u_z=0) make a single tet-meshed cube model one octant of free uniaxial tension. This
// validates the *HYPERELASTIC NEO HOOKE material + NLGEOM large-deformation step end to end
// through the real vendored ccx.
func TestNeoHookeUniaxialStretch(t *testing.T) {
	bins := requireSolver(t)
	const (
		a      = 10.0 // mm cube edge
		c10    = 1.0  // MPa
		d1     = 0.1  // 1/MPa (bulk K = 20 MPa)
		lambda = 1.2  // axial stretch
	)
	dir := t.TempDir()
	mesh := meshBox(t, bins, a, a, a, dir)

	x0 := selectNodes(mesh, func(n Node) bool { return n.X < eps(a) })
	y0 := selectNodes(mesh, func(n Node) bool { return n.Y < eps(a) })
	z0 := selectNodes(mesh, func(n Node) bool { return n.Z < eps(a) })
	top := selectNodes(mesh, func(n Node) bool { return n.Z > a-eps(a) })
	if len(x0) == 0 || len(y0) == 0 || len(z0) == 0 || len(top) == 0 {
		t.Fatalf("symmetry-face selection failed (x0=%d y0=%d z0=%d top=%d)", len(x0), len(y0), len(z0), len(top))
	}
	model := &AnalysisModel{
		Analysis: AnalysisStatic,
		Mesh:     mesh,
		Material: MaterialProps{Name: "RUBBER", Hyper: &NeoHooke{C10: c10, D1: d1}},
		Fixed: []FixedConstraint{
			{Name: "SYMX", Nodes: x0, DOFLow: 1, DOFHigh: 1},
			{Name: "SYMY", Nodes: y0, DOFLow: 2, DOFHigh: 2},
			{Name: "SYMZ", Nodes: z0, DOFLow: 3, DOFHigh: 3},
		},
		Displacements: []DisplacementBC{{Name: "PULL", Nodes: top, DOF: 3, Value: (lambda - 1) * a}},
	}
	res := solveModel(t, bins, model, dir)

	mid := selectNodes(mesh, func(n Node) bool { return n.Z > a/2-eps(a) && n.Z < a/2+eps(a) })
	got := meanSzz(res, mid)
	want := neoHookeUniaxialCauchy(lambda, c10, d1)
	relErr := math.Abs(got-want) / want
	t.Logf("Neo-Hookean uniaxial: FE σ_zz=%.5f MPa, analytic=%.5f MPa, rel err=%.1f%%", got, want, relErr*100)
	if relErr > 0.03 {
		t.Errorf("axial stress %.5f MPa differs from Neo-Hookean %.5f MPa by %.1f%% (>3%%)", got, want, relErr*100)
	}
}
