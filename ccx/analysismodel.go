// SPDX-License-Identifier: GPL-2.0-only

package ccx

// MaterialProps is a material resolved into the CalculiX unit convention (mm, t, s →
// N, MPa). The deck writer emits these verbatim, so all unit conversion from the host's
// material data happens once, upstream (see units.go).
type MaterialProps struct {
	Name            string  // *MATERIAL name
	YoungMPa        float64 // *ELASTIC Young's modulus (MPa = N/mm^2)
	Poisson         float64 // *ELASTIC Poisson's ratio
	DensityTonneMM3 float64 // *DENSITY (t/mm^3); only emitted when a body load needs it
}

// FixedConstraint pins a node set against translation. DOFLow..DOFHigh are CalculiX
// degree-of-freedom indices (1..3 for the translations of a solid C3D element).
type FixedConstraint struct {
	Name    string // node-set name
	Nodes   []int  // node ids
	DOFLow  int    // first constrained DOF (1)
	DOFHigh int    // last constrained DOF (3 for a solid)
}

// ForceLoad applies a total force along Dir, distributed equally as nodal loads over its
// node set (the standard CalculiX *CLOAD idiom for a face load on solid elements).
type ForceLoad struct {
	Name   string     // label (diagnostic)
	Nodes  []int      // node ids the load is spread over
	Dir    [3]float64 // unit direction
	TotalN float64    // total force magnitude (N)
}

// PressureLoad applies a uniform pressure (MPa) normal to a set of element-faces via a
// *DLOAD Pn card. Positive pressure pushes into the face (the CalculiX sign convention).
type PressureLoad struct {
	Name  string     // label (diagnostic)
	Faces []ElemFace // element-faces the pressure acts on
	MPa   float64    // pressure magnitude (N/mm^2)
}

// GravityLoad applies a body force (gravitational acceleration) over the whole model via a
// *DLOAD GRAV card; it requires *DENSITY on the material.
type GravityLoad struct {
	Accel float64    // acceleration magnitude (mm/s^2)
	Dir   [3]float64 // unit direction (e.g. {0,0,-1} for downward)
}

// AnalysisModel is one fully-resolved study ready to be written as a CalculiX deck: the
// solid mesh, the material, and the loads/boundary conditions, in CalculiX units.
type AnalysisModel struct {
	Analysis  AnalysisType
	Mesh      *TetMesh
	Material  MaterialProps
	Fixed     []FixedConstraint
	Forces    []ForceLoad
	Pressures []PressureLoad
	Gravity   *GravityLoad
}

// needsDensity reports whether any body load requires *DENSITY to be written. A static
// study with only nodal/pressure surface loads does not; a gravity body load does.
func (m *AnalysisModel) needsDensity() bool { return m.Gravity != nil }

// loadDirection returns the model's load direction for the visual aids, defaulting to -Z.
func loadDirection(m *AnalysisModel) [3]float64 {
	switch {
	case len(m.Forces) > 0:
		return m.Forces[0].Dir
	case m.Gravity != nil:
		return m.Gravity.Dir
	default:
		return [3]float64{0, 0, -1}
	}
}
