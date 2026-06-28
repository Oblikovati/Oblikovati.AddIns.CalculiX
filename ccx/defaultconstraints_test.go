// SPDX-License-Identifier: GPL-2.0-only

package ccx

import "testing"

// fixtureForBuild returns a tiny two-face mesh + bindings the buildModel path can resolve: face
// "fA" is the support, "fB" the loaded face.
func fixtureForBuild() (*TetMesh, *FaceGroups, []MaterialProps) {
	mesh := &TetMesh{
		Nodes:    []Node{{ID: 1}, {ID: 2}, {ID: 3}, {ID: 4}},
		Elements: []TetElement{{ID: 1, Nodes: []int{1, 2, 3, 4}}},
	}
	groups := &FaceGroups{
		Nodes:     map[string][]int{"fA": {1, 2}, "fB": {3, 4}},
		ElemFaces: map[string][]ElemFace{"fA": {{Elem: 1, Face: 1}}, "fB": {{Elem: 1, Face: 2}}},
	}
	return mesh, groups, []MaterialProps{{Name: "M", YoungMPa: 210000, Poisson: 0.3}}
}

func buildWith(t *testing.T, s StudySettings) *AnalysisModel {
	t.Helper()
	mesh, groups, mats := fixtureForBuild()
	return buildModel(s, mesh, groups, []string{"fA", "fB"}, mats)
}

// TestDefaultConstraintsReproduceEveryLoadType checks the synthesized default model matches the
// former implicit convention for each support/load/analysis branch — the Phase-1 faithfulness
// gate (no solver needed; this is a structural assertion over the resolved model).
func TestDefaultConstraintsReproduceEveryLoadType(t *testing.T) {
	base := defaultSettings()

	force := buildWith(t, withLoad(base, LoadForce, func(s *StudySettings) { s.LoadN = 250 }))
	if len(force.Fixed) != 1 || force.Fixed[0].DOFLow != 1 || force.Fixed[0].DOFHigh != 3 {
		t.Fatalf("force: support should be a full clamp, got %+v", force.Fixed)
	}
	if len(force.Forces) != 1 || force.Forces[0].TotalN != 250 || force.Forces[0].Dir != [3]float64{0, 0, -1} {
		t.Fatalf("force: load mismatch, got %+v", force.Forces)
	}

	pres := buildWith(t, withLoad(base, LoadPressure, func(s *StudySettings) { s.PressureMPa = 4 }))
	if len(pres.Pressures) != 1 || pres.Pressures[0].MPa != 4 || len(pres.Pressures[0].Faces) == 0 {
		t.Fatalf("pressure: load mismatch, got %+v", pres.Pressures)
	}

	grav := buildWith(t, withLoad(base, LoadGravity, func(s *StudySettings) { s.GravityG = 2 }))
	if grav.Gravity == nil || grav.Gravity.Accel != 2*standardGravityMMs2 {
		t.Fatalf("gravity: load mismatch, got %+v", grav.Gravity)
	}

	cent := buildWith(t, withLoad(base, LoadCentrifugal, func(s *StudySettings) { s.RotationRadS = 10 }))
	if cent.Centrifugal == nil || cent.Centrifugal.Omega2 != 100 {
		t.Fatalf("centrifugal: load mismatch, got %+v", cent.Centrifugal)
	}

	disp := buildWith(t, withLoad(base, LoadDisplacement, func(s *StudySettings) { s.DisplacementMM = 0.2 }))
	if len(disp.Displacements) != 1 || disp.Displacements[0].DOF != 3 || disp.Displacements[0].Value != 0.2 {
		t.Fatalf("displacement: load mismatch, got %+v", disp.Displacements)
	}

	hyd := buildWith(t, withLoad(base, LoadHydrostatic, func(s *StudySettings) {
		s.HydroGradientMPaMM = 0.01
		s.HydroSurfaceZ = 5
	}))
	if len(hyd.Pressures) != 1 || len(hyd.Pressures[0].PerFaceMPa) != len(hyd.Pressures[0].Faces) || hyd.Pressures[0].MPa != 0 {
		t.Fatalf("hydrostatic: expected per-face pressures, got %+v", hyd.Pressures)
	}
}

// TestDefaultConstraintsSupportAndAnalysisBranches checks the elastic-support swap and the
// modal / thermal-stress analysis branches.
func TestDefaultConstraintsSupportAndAnalysisBranches(t *testing.T) {
	base := defaultSettings()

	elastic := buildWith(t, withLoad(base, LoadForce, func(s *StudySettings) {
		s.SupportType = SupportElastic
		s.SpringStiffMM = 1000
	}))
	if len(elastic.Fixed) != 0 || len(elastic.Springs) != 1 || elastic.Springs[0].StiffnessTotal != 1000 {
		t.Fatalf("elastic support: expected a spring foundation and no clamp, got Fixed=%+v Springs=%+v", elastic.Fixed, elastic.Springs)
	}

	modal := base
	modal.Analysis = AnalysisFrequency
	mm := buildWith(t, modal)
	if len(mm.Fixed) != 1 || mm.Forces != nil || mm.Thermal != nil {
		t.Fatalf("modal: expected support only, got Fixed=%+v Forces=%+v Thermal=%+v", mm.Fixed, mm.Forces, mm.Thermal)
	}

	thermo := base
	thermo.Analysis = AnalysisThermomech
	thermo.DeltaK = 80
	tm := buildWith(t, thermo)
	if len(tm.Fixed) != 1 || tm.Thermal == nil || tm.Thermal.DeltaK != 80 {
		t.Fatalf("thermomech: expected clamp + ΔT, got Fixed=%+v Thermal=%+v", tm.Fixed, tm.Thermal)
	}
}

// TestExplicitConstraintsOverrideDefault checks an explicit constraint list is resolved instead
// of the synthesized default.
func TestExplicitConstraintsOverrideDefault(t *testing.T) {
	s := defaultSettings()
	s.Constraints = []ConstraintSpec{
		FixedSpec{Name: "A", Faces: []string{"fA"}},
		FixedSpec{Name: "B", Faces: []string{"fB"}},
	}
	m := buildWith(t, s)
	if len(m.Fixed) != 2 || m.Forces != nil {
		t.Fatalf("explicit list should yield two clamps and no synthesized load, got Fixed=%+v Forces=%+v", m.Fixed, m.Forces)
	}
}

// withLoad clones the settings to static with a given load type and an extra tweak.
func withLoad(base StudySettings, lt LoadType, tweak func(*StudySettings)) StudySettings {
	s := base
	s.Analysis = AnalysisStatic
	s.LoadType = lt
	tweak(&s)
	return s
}
