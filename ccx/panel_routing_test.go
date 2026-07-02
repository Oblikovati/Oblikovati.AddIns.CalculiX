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

func TestEMHyperTempMaterialEditsRouteToAggregate(t *testing.T) {
	e := NewEngine(nil)
	e.applyPanelEdit("elec_sigma", "2")
	e.applyPanelEdit("material_model", "neo-hookean (rubber)")
	e.applyPanelEdit("neo_c10", "3")
	e.applyPanelEdit("neo_d1", "0.2")
	e.applyPanelEdit("young_hot", "150")
	e.applyPanelEdit("hot_temp", "400")
	mat, _ := e.analysis.DefaultMaterial()
	if mat.ElectricalSigma != 2 || mat.MaterialModel != "neo-hookean (rubber)" ||
		mat.NeoHookeC10 != 3 || mat.NeoHookeD1 != 0.2 || mat.YoungHotGPa != 150 || mat.HotTempK != 400 {
		t.Fatalf("edits did not land in the aggregate material: %+v", mat)
	}
	s, _ := e.study()
	if s.ElectricalSigma != 2 || string(s.MaterialModel) != "neo-hookean (rubber)" ||
		s.NeoHookeC10 != 3 || s.YoungHotGPa != 150 {
		t.Fatalf("study() did not reflect the edits: %+v", s)
	}
}

func TestThermalMaterialEditRoutesToAggregate(t *testing.T) {
	e := NewEngine(nil)
	e.applyPanelEdit("alpha", "2.5e-5")
	e.applyPanelEdit("conductivity", "77")
	e.applyPanelEdit("specific_heat", "4.2e8")
	mat, _ := e.analysis.DefaultMaterial()
	if mat.ThermalAlpha != 2.5e-5 || mat.Conductivity != 77 || mat.SpecificHeat != 4.2e8 {
		t.Fatalf("thermal edits did not land in the aggregate material: %+v", mat)
	}
	// And the projection reflects them.
	s, _ := e.study()
	if s.ThermalAlpha != 2.5e-5 || s.Conductivity != 77 || s.SpecificHeat != 4.2e8 {
		t.Fatalf("study() did not reflect thermal edits: %+v", s)
	}
}

func TestStudySwitchEditsRouteToAggregate(t *testing.T) {
	e := NewEngine(nil)
	e.applyPanelEdit("contact_mode", "contact")
	e.applyPanelEdit("friction", "0.2")
	e.applyPanelEdit("body_scope", "bodies with a selected face")
	sv := e.analysis.Solver()
	if !sv.ContactMode || sv.FrictionMu != 0.2 || sv.BodyScope != "bodies with a selected face" {
		t.Fatalf("switch edits did not land in the solver: %+v", sv)
	}
	s, _ := e.study()
	if !s.ContactMode || s.FrictionMu != 0.2 || string(s.BodyScope) != "bodies with a selected face" {
		t.Fatalf("study() did not reflect switch edits: %+v", s)
	}
}
