// SPDX-License-Identifier: GPL-2.0-only

package femmodel

// ConstraintObject is one study constraint as pure intent: a neutral Kind tag, the host face
// reference keys it binds to, and the typed parameters of the 8 face-based builder kinds. Only the
// fields relevant to Kind are meaningful (a discriminated union, like MaterialObject). ccx maps it
// to a ConstraintSpec (which does the mesh binding) — femmodel never learns the CalculiX taxonomy.
type ConstraintObject struct {
	id, name       string
	Kind           string     // ConstraintKind underlying string: "fixed","roller","force",…
	Faces          []string   // host face reference keys
	StiffnessTotal float64    // elastic support (N/mm)
	TotalN         float64    // force
	Dir            [3]float64 // force direction
	MPa            float64    // pressure
	GradientMPa    float64    // hydrostatic γ
	SurfaceZ       float64    // hydrostatic free-surface height
	DOF            int        // displacement DOF
	Value          float64    // displacement magnitude
}

func (o ConstraintObject) ObjectID() string   { return o.id }
func (o ConstraintObject) Category() Category { return CategoryConstraint }
func (o ConstraintObject) Name() string       { return o.name }
