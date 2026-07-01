// SPDX-License-Identifier: GPL-2.0-only

package ccx

import "testing"

func TestPanelEditRoutesToAggregate(t *testing.T) {
	e := NewEngine(nil)
	e.applyPanelEdit("young", "123")
	if mat, _ := e.analysis.DefaultMaterial(); mat.YoungGPa != 123 {
		t.Fatalf("young edit did not land in the aggregate material: %+v", mat)
	}
	e.applyPanelEdit("analysis", "frequency")
	if e.analysis.Solver().AnalysisType != "frequency" {
		t.Fatalf("analysis edit did not land in the solver: %+v", e.analysis.Solver())
	}
	e.applyPanelEdit("element_order", "linear")
	if e.analysis.Mesh().Quadratic {
		t.Fatalf("element_order edit did not land in the mesh: %+v", e.analysis.Mesh())
	}
}

func TestPanelEditRoutesRemainderToExtras(t *testing.T) {
	e := NewEngine(nil)
	e.applyPanelEdit("gravity", "2.5")
	if e.extras.GravityG != 2.5 {
		t.Fatalf("gravity edit did not land in extras: %+v", e.extras.GravityG)
	}
	// And the projection reflects both homes.
	got, _ := e.study()
	if got.YoungGPa == 0 || got.GravityG != 2.5 {
		t.Fatalf("study() did not reflect aggregate+extras: young=%v gravity=%v", got.YoungGPa, got.GravityG)
	}
}
