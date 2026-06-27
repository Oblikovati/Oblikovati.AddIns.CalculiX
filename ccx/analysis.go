// SPDX-License-Identifier: GPL-2.0-only

package ccx

// AnalysisType selects the CalculiX *STEP procedure. The full set is enumerated up
// front so the deck/step writers can grow into it; the v1 vertical slice solves
// AnalysisStatic, and the others are wired in over later milestones.
type AnalysisType string

const (
	// AnalysisStatic is a linear-static stress analysis (*STATIC) — the v1 default.
	AnalysisStatic AnalysisType = "static"
	// AnalysisFrequency extracts natural frequencies / eigenmodes (*FREQUENCY).
	AnalysisFrequency AnalysisType = "frequency"
	// AnalysisBuckling computes buckling load factors (*BUCKLE).
	AnalysisBuckling AnalysisType = "buckling"
	// AnalysisThermomech is a coupled/uncoupled temperature-displacement analysis.
	AnalysisThermomech AnalysisType = "thermomech"
	// AnalysisElectromagnetic is an electromagnetic analysis (CalculiX electromagnetics).
	AnalysisElectromagnetic AnalysisType = "electromagnetic"
)

// analysisTypeOptions lists the panel dropdown choices in display order.
func analysisTypeOptions() []string {
	return []string{
		string(AnalysisStatic),
		string(AnalysisFrequency),
		string(AnalysisBuckling),
		string(AnalysisThermomech),
		string(AnalysisElectromagnetic),
	}
}

// ElementOrder selects the tetrahedral element order gmsh generates and the deck
// emits. Quadratic (C3D10) is the default: linear C3D4 tets are far too stiff in
// bending to match an analytic beam oracle.
type ElementOrder int

const (
	// LinearTet is the 4-node C3D4 element (fast, stiff in bending).
	LinearTet ElementOrder = 1
	// QuadraticTet is the 10-node C3D10 element (accurate, the default).
	QuadraticTet ElementOrder = 2
)

// StudySettings holds the panel-editable study parameters. Which faces carry the load
// vs the support is resolved from the host selection at run time (first selected face is
// the fixed support, the rest carry the load); the load magnitude and the material come
// from here until a richer setup UI and per-body material resolution land.
type StudySettings struct {
	Analysis     AnalysisType // *STEP procedure
	MeshSizeMM   float64      // gmsh characteristic length (mm); 0 = auto
	ElementOrder ElementOrder // tet element order
	DeformScale  float64      // displacement magnification for the deformed-shape render; 0 = auto

	YoungGPa float64 // material Young's modulus (GPa)
	Poisson  float64 // material Poisson's ratio
	LoadN    float64 // total force on the loaded faces (N), applied in -Z
}

// defaultSettings returns the v1 defaults: linear-static, quadratic tets, auto sizing,
// mild-steel-like elastic properties and a unit load.
func defaultSettings() StudySettings {
	return StudySettings{
		Analysis:     AnalysisStatic,
		MeshSizeMM:   0,
		ElementOrder: QuadraticTet,
		DeformScale:  0,
		YoungGPa:     210,
		Poisson:      0.3,
		LoadN:        100,
	}
}

// material returns the settings' material as CalculiX-unit elastic properties.
func (s StudySettings) material() MaterialProps {
	return MaterialProps{Name: "MATERIAL", YoungMPa: s.YoungGPa * gpaToMPa, Poisson: s.Poisson}
}
