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

// AnalysisModel is one fully-resolved study ready to be written as a CalculiX deck: the
// solid mesh, the material, and the loads/boundary conditions, in CalculiX units.
type AnalysisModel struct {
	Analysis AnalysisType
	Mesh     *TetMesh
	Material MaterialProps
	Fixed    []FixedConstraint
	Forces   []ForceLoad
}

// needsDensity reports whether any body load requires *DENSITY to be written. Static
// studies with only nodal forces do not; this grows as gravity/dynamics land.
func (m *AnalysisModel) needsDensity() bool { return false }
