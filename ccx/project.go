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
