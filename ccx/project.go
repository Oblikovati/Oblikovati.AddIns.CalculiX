// SPDX-License-Identifier: GPL-2.0-only

package ccx

import "oblikovati.org/calculix/ccx/femmodel"

// projectAnalysis flattens the Analysis tree ONTO the flat extras remainder: it starts from extras
// (which supplies every not-yet-modeled field) and overlays the fields the tree owns — "analysis
// wins". This single seam keeps the mesh/deck/solve/render pipeline reading a plain StudySettings
// while the edit model is a tree. projectAnalysis(NewDefaultAnalysis(), defaultSettings()) reproduces
// defaultSettings() exactly (the equivalence guard). Constraints are carried on StudySettings and
// returned alongside for callers that want them directly.
func projectAnalysis(a *femmodel.Analysis, extras StudySettings) (StudySettings, []ConstraintSpec) {
	s := extras

	sv := a.Solver()
	s.Analysis = AnalysisType(sv.AnalysisType)
	s.Eigenmodes = sv.Eigenmodes
	s.TransientTimeS = sv.TransientTimeS
	s.BodyScope = BodyScope(sv.BodyScope)
	s.ContactMode = sv.ContactMode
	s.FrictionMu = sv.FrictionMu

	m := a.Mesh()
	s.MeshSizeMM = m.MaxSizeMM
	s.ElementOrder = elementOrder(m.Quadratic)

	s = overlayMaterial(a, s)
	s = overlayLoad(a, s)
	s = overlaySupport(a, s)
	s = overlayThermal(a, s)

	if r, ok := a.PrimaryResult(); ok {
		s.ResultField = ResultFieldKind(r.Field)
		s.DeformScale = r.DeformScale
	}
	s.Constraints = mapConstraints(a.Constraints())
	return s, s.Constraints
}

// overlayMaterial copies all default-material fields from the Analysis aggregate onto s.
// Covers mechanical, thermal, electrical, hyperelastic, and temperature-dependent properties.
func overlayMaterial(a *femmodel.Analysis, s StudySettings) StudySettings {
	mat, ok := a.DefaultMaterial()
	if !ok {
		return s
	}
	s.YoungGPa = mat.YoungGPa
	s.Poisson = mat.Poisson
	s.DensityGCm3 = mat.DensityGCm3
	s.YieldMPa = mat.YieldMPa
	s.ThermalAlpha = mat.ThermalAlpha
	s.Conductivity = mat.Conductivity
	s.SpecificHeat = mat.SpecificHeat
	s.ElectricalSigma = mat.ElectricalSigma
	s.MaterialModel = MaterialModel(mat.MaterialModel)
	s.NeoHookeC10 = mat.NeoHookeC10
	s.NeoHookeD1 = mat.NeoHookeD1
	s.YoungHotGPa = mat.YoungHotGPa
	s.HotTempK = mat.HotTempK
	return s
}

// overlayLoad copies the 8 default-load fields from the Analysis aggregate onto s.
// Covers load type, force, pressure, gravity, rotation, displacement, and hydrostatic params.
func overlayLoad(a *femmodel.Analysis, s StudySettings) StudySettings {
	ld := a.Load()
	s.LoadType = LoadType(ld.LoadType)
	s.LoadN, s.PressureMPa, s.GravityG = ld.LoadN, ld.PressureMPa, ld.GravityG
	s.RotationRadS, s.DisplacementMM = ld.RotationRadS, ld.DisplacementMM
	s.HydroGradientMPaMM, s.HydroSurfaceZ = ld.HydroGradientMPaMM, ld.HydroSurfaceZ
	return s
}

// overlaySupport copies the 2 default-support fields from the Analysis aggregate onto s.
func overlaySupport(a *femmodel.Analysis, s StudySettings) StudySettings {
	sup := a.Support()
	s.SupportType = SupportType(sup.SupportType)
	s.SpringStiffMM = sup.SpringStiffMM
	return s
}

// overlayThermal copies the 9 thermal boundary-condition fields from the aggregate onto s.
func overlayThermal(a *femmodel.Analysis, s StudySettings) StudySettings {
	th := a.Thermal()
	s.HeatDriveMode = HeatDrive(th.HeatDriveMode)
	s.DeltaK, s.ColdTempK, s.HeatFluxQ = th.DeltaK, th.ColdTempK, th.HeatFluxQ
	s.FilmCoeff, s.SinkTempK, s.BodyHeatRate = th.FilmCoeff, th.SinkTempK, th.BodyHeatRate
	s.Emissivity, s.RadAmbientK = th.Emissivity, th.RadAmbientK
	return s
}

// elementOrder maps the mesh object's Quadratic flag to the deck element order.
func elementOrder(quadratic bool) ElementOrder {
	if quadratic {
		return QuadraticTet
	}
	return LinearTet
}
