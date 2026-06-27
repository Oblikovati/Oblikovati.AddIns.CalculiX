// SPDX-License-Identifier: GPL-2.0-only

package ccx

import (
	"math"
	"os"
	"path/filepath"
	"testing"
)

// TestCantileverMatchesEulerBernoulli is the M1 end-to-end oracle: mesh a slender beam
// with the vendored gmsh, write a CalculiX deck, solve with the vendored ccx, parse the
// .frd, and assert the tip deflection matches the analytic Euler-Bernoulli result
//
//	delta = F*L^3 / (3*E*I),  I = b*h^3/12
//
// within a tolerance that allows for shear deformation, the clamped-face stiffening, and
// mesh discretization. This proves the whole pipeline produces physically correct results,
// not merely that it runs.
func TestCantileverMatchesEulerBernoulli(t *testing.T) {
	bins := requireSolver(t)
	const (
		L, h    = 200.0, 10.0 // mm: slender beam (L/h = 20) so Euler-Bernoulli is accurate
		young   = 210000.0    // MPa
		poisson = 0.3
		force   = 100.0 // N, applied downward (-z) on the tip face
	)
	dir := t.TempDir()

	mesh := meshBeam(t, bins, L, h, dir)
	root := selectNodes(mesh, func(n Node) bool { return n.X < eps(L) })
	tip := selectNodes(mesh, func(n Node) bool { return n.X > L-eps(L) })
	if len(root) == 0 || len(tip) == 0 {
		t.Fatalf("node selection failed (root=%d tip=%d)", len(root), len(tip))
	}

	model := &AnalysisModel{
		Analysis: AnalysisStatic,
		Mesh:     mesh,
		Material: MaterialProps{Name: "STEEL", YoungMPa: young, Poisson: poisson},
		Fixed:    []FixedConstraint{{Name: "ROOT", Nodes: root, DOFLow: 1, DOFHigh: 3}},
		Forces:   []ForceLoad{{Name: "TIP", Nodes: tip, Dir: [3]float64{0, 0, -1}, TotalN: force}},
	}
	res := solveModel(t, bins, model, dir)

	got := tipDeflection(res, tip)
	inertia := h * h * h * h / 12.0
	want := force * L * L * L / (3.0 * young * inertia)
	relErr := math.Abs(got-want) / want
	t.Logf("tip deflection: FE=%.4f mm, Euler-Bernoulli=%.4f mm, rel err=%.1f%%", got, want, relErr*100)
	if relErr > 0.12 {
		t.Errorf("tip deflection %.4f mm differs from analytic %.4f mm by %.1f%% (>12%%)", got, want, relErr*100)
	}
	if vm := peak(vonMisesField(res)); !(vm > 0) || math.IsInf(vm, 0) || math.IsNaN(vm) {
		t.Errorf("von Mises peak = %v, want a finite positive stress", vm)
	}
}

// meshBeam tessellates a length×side×side box and volume-meshes it with the vendored gmsh.
func meshBeam(t *testing.T, bins solverBinaries, length, side float64, dir string) *TetMesh {
	t.Helper()
	coords, idx := boxSurface(length, side, side)
	surface, err := weldSurface(coords, idx)
	if err != nil {
		t.Fatalf("weld: %v", err)
	}
	mesh, err := NewGmshMesher(bins.gmsh).Mesh(surface, MeshOptions{SizeMM: 3, Order: QuadraticTet}, dir)
	if err != nil {
		t.Fatalf("mesh: %v", err)
	}
	return mesh
}

// solveModel writes the deck, runs ccx, and parses the result field.
func solveModel(t *testing.T, bins solverBinaries, model *AnalysisModel, dir string) *ResultField {
	t.Helper()
	stem := filepath.Join(dir, "beam")
	if err := writeFile(stem+".inp", func(f *os.File) error { return WriteDeck(f, model) }); err != nil {
		t.Fatalf("write deck: %v", err)
	}
	if err := runCcx(bins.ccx, stem); err != nil {
		t.Fatalf("solve: %v", err)
	}
	f, err := os.Open(stem + ".frd")
	if err != nil {
		t.Fatalf("open frd: %v", err)
	}
	defer f.Close()
	res, err := parseFRD(f)
	if err != nil {
		t.Fatalf("parse frd: %v", err)
	}
	return res
}

// tipDeflection returns the mean downward (-z) displacement of the tip node set.
func tipDeflection(res *ResultField, tip []int) float64 {
	sum := 0.0
	for _, id := range tip {
		sum += -res.Disp[id][2]
	}
	return sum / float64(len(tip))
}

// selectNodes returns the ids of mesh nodes satisfying pred.
func selectNodes(mesh *TetMesh, pred func(Node) bool) []int {
	var ids []int
	for _, n := range mesh.Nodes {
		if pred(n) {
			ids = append(ids, n.ID)
		}
	}
	return ids
}

// eps returns a coordinate tolerance for face selection, relative to a length scale.
func eps(scale float64) float64 { return scale * 1e-4 }
