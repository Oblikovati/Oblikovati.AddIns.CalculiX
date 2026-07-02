// SPDX-License-Identifier: GPL-2.0-only

package ccx

import "oblikovati.org/calculix/ccx/femmodel"

// objectForKind builds a pure ConstraintObject for a builder kind, extracting the per-kind params
// from the (projected) settings — the exact inverse of newConstraintSpec's read of StudySettings.
func objectForKind(kind ConstraintKind, faces []string, s StudySettings) femmodel.ConstraintObject {
	o := femmodel.ConstraintObject{Kind: string(builderKindOrDefault(kind)), Faces: faces}
	switch builderKindOrDefault(kind) {
	case KindElasticSupport:
		o.StiffnessTotal = s.SpringStiffMM
	case KindForce:
		o.TotalN, o.Dir = s.LoadN, [3]float64{0, 0, -1}
	case KindPressure:
		o.MPa = s.PressureMPa
	case KindHydrostatic:
		o.GradientMPa, o.SurfaceZ = s.HydroGradientMPaMM, s.HydroSurfaceZ
	case KindDisplacement:
		o.DOF, o.Value = 3, s.DisplacementMM
	}
	return o
}

// constraintSpecFor maps a pure ConstraintObject back to the ccx ConstraintSpec that binds faces
// and writes the AnalysisModel — the one place neutral Kind → typed spec happens (ADR-1).
func constraintSpecFor(o femmodel.ConstraintObject) ConstraintSpec {
	name, faces := o.Name(), o.Faces
	switch ConstraintKind(o.Kind) {
	case KindRoller:
		return RollerSpec{Name: name, Faces: faces}
	case KindSymmetry:
		return SymmetrySpec{Name: name, Faces: faces}
	case KindElasticSupport:
		return ElasticSupportSpec{Name: name, Faces: faces, StiffnessTotal: o.StiffnessTotal}
	case KindForce:
		return ForceSpec{Name: name, Faces: faces, Dir: o.Dir, TotalN: o.TotalN}
	case KindPressure:
		return PressureSpec{Name: name, Faces: faces, MPa: o.MPa}
	case KindHydrostatic:
		return HydrostaticSpec{Name: name, Faces: faces, GradientMPa: o.GradientMPa, SurfaceZ: o.SurfaceZ}
	case KindDisplacement:
		return DisplacementSpec{Name: name, Faces: faces, DOF: o.DOF, Value: o.Value}
	default:
		return FixedSpec{Name: name, Faces: faces}
	}
}

// mapConstraints projects the aggregate's constraint objects to solver-pipeline specs, in order.
func mapConstraints(objs []femmodel.ConstraintObject) []ConstraintSpec {
	if len(objs) == 0 {
		return nil
	}
	specs := make([]ConstraintSpec, len(objs))
	for i, o := range objs {
		specs[i] = constraintSpecFor(o)
	}
	return specs
}
