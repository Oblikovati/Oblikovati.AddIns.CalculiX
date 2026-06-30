// SPDX-License-Identifier: GPL-2.0-only

package femmodel

import "testing"

func TestNewDefaultAnalysisCoreValues(t *testing.T) {
	a := NewDefaultAnalysis()
	if a.Solver().AnalysisType != "static" || a.Solver().Eigenmodes != 6 || a.Solver().TransientTimeS != 0 {
		t.Fatalf("default solver wrong: %+v", a.Solver())
	}
	if a.Mesh().MaxSizeMM != 0 || !a.Mesh().Quadratic {
		t.Fatalf("default mesh wrong: %+v", a.Mesh())
	}
	mat, ok := a.DefaultMaterial()
	if !ok || mat.YoungGPa != 210 || mat.Poisson != 0.3 || mat.DensityGCm3 != 7.85 || mat.YieldMPa != 0 {
		t.Fatalf("default material wrong: %+v ok=%v", mat, ok)
	}
	r, ok := a.PrimaryResult()
	if !ok || r.Field != "von Mises stress" || r.DeformScale != 0 {
		t.Fatalf("default result wrong: %+v ok=%v", r, ok)
	}
}

func TestAddMaterialAssignsUniqueIDs(t *testing.T) {
	a := NewDefaultAnalysis()
	first := len(a.Materials())
	m2 := a.AddMaterial("Aluminium", 69, 0.33, 2.70, 0, false)
	if m2.ObjectID() == a.Materials()[0].ObjectID() {
		t.Fatalf("AddMaterial reused id %q", m2.ObjectID())
	}
	if len(a.Materials()) != first+1 {
		t.Fatalf("AddMaterial did not append: %d", len(a.Materials()))
	}
	if _, ok := a.DefaultMaterial(); !ok {
		t.Fatalf("DefaultMaterial lost after adding a scoped material")
	}
}

func TestSetSolverPreservesID(t *testing.T) {
	a := NewDefaultAnalysis()
	origID := a.Solver().ObjectID()
	a.SetSolver(newSolverObject("other", "frequency", 12, 0))
	if a.Solver().ObjectID() != origID {
		t.Fatalf("SetSolver changed id: got %q want %q", a.Solver().ObjectID(), origID)
	}
	if a.Solver().AnalysisType != "frequency" || a.Solver().Eigenmodes != 12 {
		t.Fatalf("SetSolver did not update fields: %+v", a.Solver())
	}
}

func TestSetMeshPreservesID(t *testing.T) {
	a := NewDefaultAnalysis()
	origID := a.Mesh().ObjectID()
	a.SetMesh(newMeshObject("other", 2.5, false))
	if a.Mesh().ObjectID() != origID {
		t.Fatalf("SetMesh changed id: got %q want %q", a.Mesh().ObjectID(), origID)
	}
	if a.Mesh().MaxSizeMM != 2.5 || a.Mesh().Quadratic {
		t.Fatalf("SetMesh did not update fields: %+v", a.Mesh())
	}
}
