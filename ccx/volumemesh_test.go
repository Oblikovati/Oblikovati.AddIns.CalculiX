// SPDX-License-Identifier: GPL-2.0-only

package ccx

import (
	"os"
	"path/filepath"
	"testing"
)

// requireSolver points the engine at the in-repo vendored build output and skips the
// test when ccx/gmsh have not been built. Tests that actually run a solver call it first.
func requireSolver(t *testing.T) solverBinaries {
	t.Helper()
	repo, err := filepath.Abs("..")
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}
	bins := solverBinaries{
		ccx:  filepath.Join(repo, "vendor-src/ccx/build/ccx"),
		gmsh: filepath.Join(repo, "vendor-src/gmsh/build/gmsh"),
	}
	for tool, p := range map[string]string{"ccx": bins.ccx, "gmsh": bins.gmsh} {
		if _, err := os.Stat(p); err != nil {
			t.Skipf("%s not built: run `make build-solvers` (%s)", tool, p)
		}
	}
	return bins
}

// boxSurface returns a raw triangle soup for an sx×sy×sz box with each face's vertices
// listed independently (24 vertices, 12 triangles) — exercising the weld, which must
// collapse the 24 shared-corner vertices down to 8.
func boxSurface(sx, sy, sz float64) ([]float64, []int) {
	v := [8][3]float64{
		{0, 0, 0}, {sx, 0, 0}, {sx, sy, 0}, {0, sy, 0},
		{0, 0, sz}, {sx, 0, sz}, {sx, sy, sz}, {0, sy, sz},
	}
	quads := [6][4]int{{0, 3, 2, 1}, {4, 5, 6, 7}, {0, 1, 5, 4}, {1, 2, 6, 5}, {2, 3, 7, 6}, {3, 0, 4, 7}}
	var coords []float64
	var idx []int
	for _, q := range quads {
		base := len(coords) / 3
		for _, c := range q {
			coords = append(coords, v[c][0], v[c][1], v[c][2])
		}
		idx = append(idx, base, base+1, base+2, base, base+2, base+3)
	}
	return coords, idx
}

func TestWeldCollapsesDuplicateCorners(t *testing.T) {
	coords, idx := boxSurface(10, 10, 10)
	s, err := weldSurface(coords, idx)
	if err != nil {
		t.Fatalf("weld: %v", err)
	}
	if len(s.Verts) != 8 {
		t.Errorf("welded vertex count = %d, want 8", len(s.Verts))
	}
	if len(s.Tris) != 12 {
		t.Errorf("triangle count = %d, want 12", len(s.Tris))
	}
}

func TestGmshMeshesBoxIntoTets(t *testing.T) {
	bins := requireSolver(t)
	coords, idx := boxSurface(10, 10, 10)
	surface, err := weldSurface(coords, idx)
	if err != nil {
		t.Fatalf("weld: %v", err)
	}
	mesh, err := NewGmshMesher(bins.gmsh).Mesh(surface, MeshOptions{SizeMM: 4, Order: QuadraticTet}, t.TempDir())
	if err != nil {
		t.Fatalf("mesh: %v", err)
	}
	if len(mesh.Elements) == 0 {
		t.Fatal("no tetrahedra produced")
	}
	if mesh.ElementType() != "C3D10" {
		t.Errorf("element type = %s, want C3D10", mesh.ElementType())
	}
	for i, e := range mesh.Elements {
		if !e.IsQuadratic() {
			t.Fatalf("element %d has %d nodes, want 10 (C3D10)", i, len(e.Nodes))
		}
	}
	if len(mesh.Surface) == 0 {
		t.Error("no boundary facets captured (needed for load/BC binding)")
	}
}
