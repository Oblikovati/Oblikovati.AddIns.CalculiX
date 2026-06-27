// SPDX-License-Identifier: GPL-2.0-only

package ccx

import (
	"math"
	"testing"
)

// meshBox tessellates an sx×sy×sz box and volume-meshes it with the vendored gmsh.
func meshBox(t *testing.T, bins solverBinaries, sx, sy, sz float64, dir string) *TetMesh {
	t.Helper()
	coords, idx := boxSurface(sx, sy, sz)
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

// elemFacesAt returns the element-faces of every boundary facet whose three corner nodes
// all satisfy pred — the coordinate-driven analog of binding a pressure to a picked face.
func elemFacesAt(mesh *TetMesh, pred func(Node) bool) []ElemFace {
	index := faceElemIndex(mesh)
	nodeIdx := mesh.nodeByID()
	var faces []ElemFace
	for _, bf := range mesh.Surface {
		if pred(nodeIdx[bf.Corners[0]]) && pred(nodeIdx[bf.Corners[1]]) && pred(nodeIdx[bf.Corners[2]]) {
			if ef, ok := index[sortedTriple(bf.Corners[0], bf.Corners[1], bf.Corners[2])]; ok {
				faces = append(faces, ef)
			}
		}
	}
	return faces
}

// TestPressureUniaxialCompression validates the *DLOAD pressure path: a slender bar fixed
// at one end and pressed on the far end compresses by ~P·L/E (uniaxial), the analytic
// result for a bar under uniform end pressure.
func TestPressureUniaxialCompression(t *testing.T) {
	bins := requireSolver(t)
	const (
		w, L  = 10.0, 100.0 // mm, bar along z (slender, L/w = 10)
		young = 210000.0    // MPa
		press = 10.0        // MPa on the top face
	)
	dir := t.TempDir()
	mesh := meshBox(t, bins, w, w, L, dir)

	base := selectNodes(mesh, func(n Node) bool { return n.Z < eps(L) })
	topFaces := elemFacesAt(mesh, func(n Node) bool { return n.Z > L-eps(L) })
	if len(base) == 0 || len(topFaces) == 0 {
		t.Fatalf("selection failed (base=%d topFaces=%d)", len(base), len(topFaces))
	}

	model := &AnalysisModel{
		Analysis:  AnalysisStatic,
		Mesh:      mesh,
		Material:  MaterialProps{Name: "STEEL", YoungMPa: young, Poisson: 0.3},
		Fixed:     []FixedConstraint{{Name: "BASE", Nodes: base, DOFLow: 1, DOFHigh: 3}},
		Pressures: []PressureLoad{{Name: "TOP", Faces: topFaces, MPa: press}},
	}
	res := solveModel(t, bins, model, dir)

	got := topCompression(res, selectNodes(mesh, func(n Node) bool { return n.Z > L-eps(L) }))
	want := press * L / young
	relErr := math.Abs(got-want) / want
	t.Logf("compression: FE=%.5g mm, analytic P*L/E=%.5g mm, rel err=%.1f%%", got, want, relErr*100)
	if relErr > 0.08 {
		t.Errorf("compression %.5g differs from analytic %.5g by %.1f%% (>8%%)", got, want, relErr*100)
	}
}

// TestGravityHangingBar validates the *DLOAD GRAV path: a bar fixed at the top and loaded
// by self-weight extends, with the free-end displacement matching ρgL²/(2E).
func TestGravityHangingBar(t *testing.T) {
	bins := requireSolver(t)
	const (
		w, L    = 20.0, 200.0 // mm
		young   = 210000.0    // MPa
		rho     = 7.9e-9      // t/mm^3 (steel)
		gravity = 9810.0      // mm/s^2
	)
	dir := t.TempDir()
	mesh := meshBox(t, bins, w, w, L, dir)

	top := selectNodes(mesh, func(n Node) bool { return n.Z > L-eps(L) })
	bottom := selectNodes(mesh, func(n Node) bool { return n.Z < eps(L) })
	if len(top) == 0 || len(bottom) == 0 {
		t.Fatalf("selection failed (top=%d bottom=%d)", len(top), len(bottom))
	}

	model := &AnalysisModel{
		Analysis: AnalysisStatic,
		Mesh:     mesh,
		Material: MaterialProps{Name: "STEEL", YoungMPa: young, Poisson: 0.3, DensityTonneMM3: rho},
		Fixed:    []FixedConstraint{{Name: "TOP", Nodes: top, DOFLow: 1, DOFHigh: 3}},
		Gravity:  &GravityLoad{Accel: gravity, Dir: [3]float64{0, 0, -1}},
	}
	res := solveModel(t, bins, model, dir)

	got := math.Abs(meanDispZ(res, bottom))
	want := rho * gravity * L * L / (2 * young)
	relErr := math.Abs(got-want) / want
	t.Logf("hanging-bar free-end drop: FE=%.5g mm, analytic ρgL²/2E=%.5g mm, rel err=%.1f%%", got, want, relErr*100)
	if relErr > 0.10 {
		t.Errorf("free-end drop %.5g differs from analytic %.5g by %.1f%% (>10%%)", got, want, relErr*100)
	}
}

// topCompression returns the mean downward (-z) displacement of the given node set.
func topCompression(res *ResultField, nodes []int) float64 {
	return math.Abs(meanDispZ(res, nodes))
}

// meanDispZ returns the mean z-displacement over a node set.
func meanDispZ(res *ResultField, nodes []int) float64 {
	sum := 0.0
	for _, id := range nodes {
		sum += res.Disp[id][2]
	}
	return sum / float64(len(nodes))
}
