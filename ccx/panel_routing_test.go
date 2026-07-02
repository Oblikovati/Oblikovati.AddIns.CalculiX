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

// TestStudyProjectsMultipleAggregateGroups proves projectAnalysis composes several aggregate
// groups into one StudySettings: every panel control routes to the aggregate, so a single study()
// must reflect both the seeded material group and a freshly edited EM group at once.
func TestStudyProjectsMultipleAggregateGroups(t *testing.T) {
	e := NewEngine(nil)
	// voltage is fully migrated as of Phase 2.11 E2. Verify the projection reflects both the
	// aggregate material (YoungGPa seeded) and the EM voltage edit (now in the aggregate).
	e.applyPanelEdit("voltage", "12")
	em := e.analysis.EM()
	if em.VoltageV != 12 {
		t.Fatalf("voltage edit did not land in the EM aggregate: %+v", em)
	}
	// The projection must reflect both homes: material from the aggregate, VoltageV from the EM aggregate.
	got, _ := e.study()
	if got.YoungGPa == 0 || got.VoltageV != 12 {
		t.Fatalf("study() did not reflect aggregate: young=%v voltage=%v", got.YoungGPa, got.VoltageV)
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

func TestLoadEditsRouteToAggregate(t *testing.T) {
	e := NewEngine(nil)
	e.applyPanelEdit("load_type", "pressure")
	e.applyPanelEdit("load", "250")
	e.applyPanelEdit("pressure", "3")
	e.applyPanelEdit("gravity", "2")
	e.applyPanelEdit("rotation", "60")
	e.applyPanelEdit("displacement", "0.5")
	e.applyPanelEdit("hydro_gradient", "2e-5")
	e.applyPanelEdit("hydro_surface", "8")
	ld := e.analysis.Load()
	if ld.LoadType != "pressure" || ld.LoadN != 250 || ld.PressureMPa != 3 || ld.GravityG != 2 ||
		ld.RotationRadS != 60 || ld.DisplacementMM != 0.5 || ld.HydroGradientMPaMM != 2e-5 || ld.HydroSurfaceZ != 8 {
		t.Fatalf("load edits did not land in the aggregate: %+v", ld)
	}
	s, _ := e.study()
	if string(s.LoadType) != "pressure" || s.LoadN != 250 || s.HydroSurfaceZ != 8 {
		t.Fatalf("study() did not reflect load edits: %+v", s)
	}
}

func TestSupportEditsRouteToAggregate(t *testing.T) {
	e := NewEngine(nil)
	e.applyPanelEdit("support_type", "elastic (spring)")
	e.applyPanelEdit("spring_stiffness", "250")

	sup := e.analysis.Support()
	if sup.SupportType != "elastic (spring)" {
		t.Fatalf("aggregate SupportType = %q, want \"elastic (spring)\"", sup.SupportType)
	}
	if sup.SpringStiffMM != 250 {
		t.Fatalf("aggregate SpringStiffMM = %v, want 250", sup.SpringStiffMM)
	}
	s, _ := e.study()
	if s.SupportType != SupportElastic || s.SpringStiffMM != 250 {
		t.Fatalf("study() support = {%v %v}, want {elastic 250}", s.SupportType, s.SpringStiffMM)
	}
}

func TestThermalEditsRouteToAggregate(t *testing.T) {
	e := NewEngine(nil)
	e.applyPanelEdit("delta_t", "60")
	e.applyPanelEdit("cold_temp", "10")
	e.applyPanelEdit("heat_flux", "70")
	e.applyPanelEdit("heat_drive", "convection")
	e.applyPanelEdit("film_coeff", "1.5")
	e.applyPanelEdit("sink_temp", "20")
	e.applyPanelEdit("body_heat", "9")
	e.applyPanelEdit("emissivity", "0.3")
	e.applyPanelEdit("rad_ambient", "310")
	th := e.analysis.Thermal()
	if th.HeatDriveMode != "convection" || th.DeltaK != 60 || th.ColdTempK != 10 || th.HeatFluxQ != 70 ||
		th.FilmCoeff != 1.5 || th.SinkTempK != 20 || th.BodyHeatRate != 9 || th.Emissivity != 0.3 || th.RadAmbientK != 310 {
		t.Fatalf("thermal edits did not land in the aggregate: %+v", th)
	}
	s, _ := e.study()
	if s.HeatDriveMode != HeatDriveFilm || s.DeltaK != 60 || s.RadAmbientK != 310 {
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

func TestEMEditsRouteToAggregate(t *testing.T) {
	e := NewEngine(nil)
	e.applyPanelEdit("voltage", "24")
	e.applyPanelEdit("em_drive", "current")
	e.applyPanelEdit("current_density", "3")
	em := e.analysis.EM()
	if em.EMDriveMode != "current" || em.VoltageV != 24 || em.CurrentDensity != 3 {
		t.Fatalf("EM edits did not land in the aggregate: %+v", em)
	}
	s, _ := e.study()
	if s.EMDriveMode != EMCurrent || s.VoltageV != 24 || s.CurrentDensity != 3 {
		t.Fatalf("study() did not reflect EM edits: %+v", s)
	}
}
