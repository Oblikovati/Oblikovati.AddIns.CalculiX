// SPDX-License-Identifier: GPL-2.0-only

package ccx

import "fmt"

// builderKinds lists the face-based constraint types the panel builder can add. Whole-body loads
// (gravity, centrifugal) have no face selection and stay on the global load-type path.
func builderKinds() []ConstraintKind {
	return []ConstraintKind{
		KindFixed, KindRoller, KindSymmetry, KindElasticSupport,
		KindForce, KindPressure, KindHydrostatic, KindDisplacement,
	}
}

// builderKindOrDefault treats the unset builder kind as a fixed support.
func builderKindOrDefault(k ConstraintKind) ConstraintKind {
	if k == "" {
		return KindFixed
	}
	return k
}

// newConstraintSpec builds a constraint spec of the given kind over the selected faces, taking its
// parameters from the matching flat builder fields. This is the one place panel state becomes a
// spec; adding a builder kind is a case here plus the spec type.
func newConstraintSpec(kind ConstraintKind, name string, faces []string, s StudySettings) ConstraintSpec {
	switch builderKindOrDefault(kind) {
	case KindRoller:
		return RollerSpec{Name: name, Faces: faces}
	case KindSymmetry:
		return SymmetrySpec{Name: name, Faces: faces}
	case KindElasticSupport:
		return ElasticSupportSpec{Name: name, Faces: faces, StiffnessTotal: s.SpringStiffMM}
	case KindForce:
		return ForceSpec{Name: name, Faces: faces, Dir: [3]float64{0, 0, -1}, TotalN: s.LoadN}
	case KindPressure:
		return PressureSpec{Name: name, Faces: faces, MPa: s.PressureMPa}
	case KindHydrostatic:
		return HydrostaticSpec{Name: name, Faces: faces, GradientMPa: s.HydroGradientMPaMM, SurfaceZ: s.HydroSurfaceZ}
	case KindDisplacement:
		return DisplacementSpec{Name: name, Faces: faces, DOF: 3, Value: s.DisplacementMM}
	default: // KindFixed
		return FixedSpec{Name: name, Faces: faces}
	}
}

// addConstraintFromSelection snapshots the current host face selection and builder parameters into
// a new constraint on the aggregate and refreshes the panel. It runs OFF the session goroutine (it
// makes host calls — read selection, redraw panel), so onCommandStarted dispatches it on its own
// goroutine. params + count + insert are captured under a single e.mu.Lock to close the TOCTOU
// that existed when study() released the lock before AddConstraint ran.
func (e *Engine) addConstraintFromSelection() {
	sel, err := e.api.Model().Selection()
	if err != nil {
		e.reportStatus("CalculiX: could not read the selection — " + err.Error())
		return
	}
	faces := decodeSelectedFaces(sel.Refs)
	if len(faces) == 0 {
		e.reportStatus("CalculiX: select a face of the part, then Add from selection.")
		return
	}
	// Capture params, name, and insert under a single lock: closing the TOCTOU where
	// settings could drift between study() returning and the constraint being appended.
	// projectAnalysis takes no lock itself, so calling it here is safe.
	e.mu.Lock()
	settings, _ := projectAnalysis(e.analysis)
	name := fmt.Sprintf("C%d", len(e.analysis.Constraints()))
	obj := e.analysis.AddConstraint(name, objectForKind(e.builderKind, faces, settings))
	count := len(e.analysis.Constraints())
	kind := ConstraintKind(obj.Kind)
	e.mu.Unlock()
	_, _ = e.ShowPanel()
	e.reportStatus(fmt.Sprintf("CalculiX: added a %s constraint on %d face(s); %d total.",
		builderKindOrDefault(kind), len(faces), count))
}

// clearConstraints empties the explicit constraint list on the aggregate (the study then falls
// back to the synthesized default) and refreshes the panel.
func (e *Engine) clearConstraints() {
	e.mu.Lock()
	e.analysis.ClearConstraints()
	e.mu.Unlock()
	_, _ = e.ShowPanel()
	e.reportStatus("CalculiX: cleared all added constraints.")
}
