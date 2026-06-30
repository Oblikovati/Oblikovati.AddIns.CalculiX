// SPDX-License-Identifier: GPL-2.0-only

package ccx

import "oblikovati.org/calculix/ccx/femmodel"

// projectAnalysis flattens the femmodel.Analysis tree onto the engine's StudySettings + explicit
// constraint list — the seam that keeps the mature mesh/deck/solve/render pipeline unchanged while
// the edit model becomes a tree. It starts from the v1 defaults and overrides only the fields the
// Phase-1 tree owns (Solver/Mesh/Material/Result); fields not yet migrated keep their defaults, so
// projecting the default Analysis reproduces defaultSettings() exactly. Constraints are carried on
// StudySettings and returned alongside for callers that want them directly.
func projectAnalysis(a *femmodel.Analysis) (StudySettings, []ConstraintSpec) {
	s := defaultSettings()

	sv := a.Solver()
	s.Analysis = AnalysisType(sv.AnalysisType)
	s.Eigenmodes = sv.Eigenmodes
	s.TransientTimeS = sv.TransientTimeS

	m := a.Mesh()
	s.MeshSizeMM = m.MaxSizeMM
	s.ElementOrder = elementOrder(m.Quadratic)

	if mat, ok := a.DefaultMaterial(); ok {
		s.YoungGPa = mat.YoungGPa
		s.Poisson = mat.Poisson
		s.DensityGCm3 = mat.DensityGCm3
		s.YieldMPa = mat.YieldMPa
	}
	if r, ok := a.PrimaryResult(); ok {
		s.ResultField = ResultFieldKind(r.Field)
		s.DeformScale = r.DeformScale
	}
	return s, s.Constraints
}

// elementOrder maps the mesh object's Quadratic flag to the deck element order.
func elementOrder(quadratic bool) ElementOrder {
	if quadratic {
		return QuadraticTet
	}
	return LinearTet
}
