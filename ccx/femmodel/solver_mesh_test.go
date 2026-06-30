// SPDX-License-Identifier: GPL-2.0-only

package femmodel

import "testing"

func TestSolverObject(t *testing.T) {
	s := newSolverObject("solver", "static", 6, 0)
	if s.ObjectID() != "solver" || s.Category() != CategorySolver || s.Name() != "Solver" {
		t.Fatalf("solver identity wrong: %+v", s)
	}
	if s.AnalysisType != "static" || s.Eigenmodes != 6 || s.TransientTimeS != 0 {
		t.Fatalf("solver fields wrong: %+v", s)
	}
}

func TestMeshObject(t *testing.T) {
	m := newMeshObject("mesh", 0, true)
	if m.ObjectID() != "mesh" || m.Category() != CategoryMesh || m.Name() != "Mesh" {
		t.Fatalf("mesh identity wrong: %+v", m)
	}
	if m.MaxSizeMM != 0 || !m.Quadratic {
		t.Fatalf("mesh fields wrong: %+v", m)
	}
}
