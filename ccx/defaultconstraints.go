// SPDX-License-Identifier: GPL-2.0-only

package ccx

// defaultConstraints expresses the implicit selection convention as an explicit spec list, so
// buildModel has one resolve path whether or not constraints were added explicitly. The
// convention (unchanged): the FIRST selected face is the support; the remaining faces carry the
// global load. A modal study applies no load; a thermal-stress study applies a uniform
// temperature instead of a mechanical load.
func defaultConstraints(settings StudySettings, faces []string) []ConstraintSpec {
	support := supportSpec(settings, faces[:1])
	switch settings.Analysis {
	case AnalysisFrequency:
		return []ConstraintSpec{support} // free vibration: support only, no load
	case AnalysisThermomech:
		return []ConstraintSpec{support, ThermalLoadSpec{DeltaK: settings.DeltaK}}
	default:
		return append([]ConstraintSpec{support}, loadSpec(settings, faces[1:]))
	}
}

// supportSpec is the support on the first face: a rigid clamp, or a grounded elastic foundation
// for an elastic-support static study. The elastic option only applies to the static stress path;
// modal and thermal-stress always clamp.
func supportSpec(settings StudySettings, supportFace []string) ConstraintSpec {
	if settings.Analysis == AnalysisStatic && settings.SupportType == SupportElastic {
		return ElasticSupportSpec{Name: "FIX", Faces: supportFace, StiffnessTotal: settings.SpringStiffMM}
	}
	return FixedSpec{Name: "FIX", Faces: supportFace}
}

// loadSpec is the single mechanical load on the loaded faces, selected by the global LoadType —
// the explicit-spec form of the former applyLoad switch.
func loadSpec(settings StudySettings, loadFaces []string) ConstraintSpec {
	switch settings.LoadType {
	case LoadGravity:
		return GravitySpec{Accel: settings.GravityG * standardGravityMMs2, Dir: [3]float64{0, 0, -1}}
	case LoadCentrifugal:
		return CentrifugalSpec{Omega2: settings.RotationRadS * settings.RotationRadS,
			AxisPoint: [3]float64{0, 0, 0}, AxisDir: [3]float64{0, 0, 1}}
	case LoadPressure:
		return PressureSpec{Name: "LOAD", Faces: loadFaces, MPa: settings.PressureMPa}
	case LoadDisplacement:
		return DisplacementSpec{Name: "PRESCR", Faces: loadFaces, DOF: 3, Value: settings.DisplacementMM}
	default: // LoadForce
		return ForceSpec{Name: "LOAD", Faces: loadFaces, Dir: [3]float64{0, 0, -1}, TotalN: settings.LoadN}
	}
}
