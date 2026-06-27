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
	ExpansionPerK   float64 // *EXPANSION thermal coefficient (1/K); used by thermal stress
	Conductivity    float64 // *CONDUCTIVITY (consistent units); used by heat transfer
	ElectricalSigma float64 // electrical conductivity (consistent units); used by the electrostatic analogy
	SpecificHeat    float64 // *SPECIFIC HEAT (consistent units); used by transient coupled analysis
}

// TransientStep parameterizes a time-dependent step: the initial time increment and the
// total step time (the CalculiX "tinc, tper" data line). A nil TransientStep means the step
// is steady-state.
type TransientStep struct {
	IncrementS float64 // initial time increment (s)
	TotalS     float64 // total step time (s)
}

// TemperatureBC prescribes a fixed temperature on a node set (the temperature degree of
// freedom 11) for a heat-transfer analysis.
type TemperatureBC struct {
	Name  string
	Nodes []int
	TempK float64
}

// HeatFlux applies a surface heat flux to a set of element-faces (*DFLUX Sn) for a
// heat-transfer analysis.
type HeatFlux struct {
	Name  string
	Faces []ElemFace
	Flux  float64
}

// FilmBC applies convective film heat exchange to a set of element-faces (*FILM Fn) for a
// heat-transfer analysis: each face exchanges q = Coeff·(T − SinkTempK) with the ambient.
type FilmBC struct {
	Name      string
	Faces     []ElemFace
	Coeff     float64 // film coefficient h
	SinkTempK float64 // ambient/sink temperature
}

// BodyHeat applies a uniform volumetric internal heat generation over the whole body via a
// *DLOAD BF card (power per unit volume) — internal heating like resistive or nuclear heating.
type BodyHeat struct {
	Rate float64 // volumetric generation (consistent units)
}

// RadiationBC applies radiative heat exchange to a set of element-faces (*RADIATE Rn) for a
// heat-transfer analysis: each face radiates q = ε·σ·(T⁴ − AmbientK⁴) to the surroundings.
type RadiationBC struct {
	Name       string
	Faces      []ElemFace
	Emissivity float64 // surface emissivity (0..1)
	AmbientK   float64 // ambient temperature (K)
}

// ThermalLoad applies a uniform temperature change over the whole body for an uncoupled
// thermal-stress analysis: a *TEMPERATURE field in the step, relative to a stress-free
// reference of zero, producing thermal expansion against the material's *EXPANSION.
type ThermalLoad struct {
	DeltaK float64 // temperature change from the stress-free state (K)
}

// FixedConstraint pins a node set against translation. DOFLow..DOFHigh are CalculiX
// degree-of-freedom indices (1..3 for the translations of a solid C3D element).
type FixedConstraint struct {
	Name    string // node-set name
	Nodes   []int  // node ids
	DOFLow  int    // first constrained DOF (1)
	DOFHigh int    // last constrained DOF (3 for a solid)
}

// DisplacementBC enforces a prescribed displacement on a node set: a non-zero *BOUNDARY on a
// single translational DOF, moving the face a set distance (vs a force, which sets the load).
type DisplacementBC struct {
	Name  string  // node-set name
	Nodes []int   // node ids
	DOF   int     // constrained translational DOF (1..3)
	Value float64 // enforced displacement (mm)
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

// CentrifugalLoad applies the body force of rotation about an axis via a *DLOAD CENTRIF card:
// each element feels ρ·ω²·r outward from the axis. It requires *DENSITY on the material.
type CentrifugalLoad struct {
	Omega2    float64    // angular velocity squared (rad/s)^2
	AxisPoint [3]float64 // a point on the rotation axis (mm)
	AxisDir   [3]float64 // axis direction (unit)
}

// MaterialSection assigns one material to an element set — a *SOLID SECTION over an *ELSET.
// A single-material study has one section over every element; a multi-body study has one per
// body (its element ids), so a part of mixed materials writes one *MATERIAL + *SOLID SECTION
// per distinct material (CalculiX's per-material ELSET idiom).
type MaterialSection struct {
	ElsetName  string        // *ELSET / *SOLID SECTION set name (e.g. "Eb0")
	Material   MaterialProps // the material assigned to this set
	ElementIDs []int         // ids of the elements in the set
}

// AnalysisModel is one fully-resolved study ready to be written as a CalculiX deck: the
// solid mesh, the material(s), and the loads/boundary conditions, in CalculiX units.
type AnalysisModel struct {
	Analysis       AnalysisType
	Mesh           *TetMesh
	Material       MaterialProps     // the (first/only) material; see Sections for multi-material
	Sections       []MaterialSection // per-body material sections; empty ⇒ single Material over all elements
	Fixed          []FixedConstraint
	Displacements  []DisplacementBC
	Forces         []ForceLoad
	Pressures      []PressureLoad
	Gravity        *GravityLoad
	Centrifugal    *CentrifugalLoad
	Thermal        *ThermalLoad
	Temperatures   []TemperatureBC // prescribed temperatures (heat transfer)
	HeatFluxes     []HeatFlux      // surface heat fluxes (heat transfer)
	Films          []FilmBC        // convective film exchanges (heat transfer)
	BodyHeat       *BodyHeat       // volumetric internal heat generation (heat transfer)
	Radiations     []RadiationBC   // radiative face exchanges (heat transfer)
	EigenmodeCount int             // number of modes/factors for *FREQUENCY / *BUCKLE
	ResultField    ResultFieldKind // which scalar field a stress result is coloured by
	Ties           []TieConstraint // bonded interfaces between touching bodies (*TIE)
	InitialTempK   float64         // reference/initial temperature (*INITIAL CONDITIONS); 0 = default
	Transient      *TransientStep  // time stepping for a transient coupled study; nil = steady state
}

// sections returns the model's material sections, deriving a single section over every
// element from Material when none are set explicitly. This keeps the single-material path
// (and every test that sets only Material) writing exactly one *ELSET/*MATERIAL/*SOLID
// SECTION, while multi-body studies populate Sections directly.
func (m *AnalysisModel) sections() []MaterialSection {
	if len(m.Sections) > 0 {
		return m.Sections
	}
	ids := make([]int, len(m.Mesh.Elements))
	for i, e := range m.Mesh.Elements {
		ids[i] = e.ID
	}
	return []MaterialSection{{ElsetName: allElementsSet, Material: m.Material, ElementIDs: ids}}
}

// distinctMaterials returns the model's materials deduplicated by name, preserving first
// occurrence order, so a part with several bodies sharing a material writes one *MATERIAL.
func (m *AnalysisModel) distinctMaterials() []MaterialProps {
	seen := map[string]bool{}
	var out []MaterialProps
	for _, s := range m.sections() {
		if !seen[s.Material.Name] {
			seen[s.Material.Name] = true
			out = append(out, s.Material)
		}
	}
	return out
}

// needsDensity reports whether *DENSITY must be written. A gravity body load needs it for
// the body force; a frequency analysis needs it for the mass matrix. A static stress study
// with only surface loads, and a buckling analysis (a static eigenproblem), do not.
func (m *AnalysisModel) needsDensity() bool {
	return m.Gravity != nil || m.Centrifugal != nil || m.Analysis == AnalysisFrequency || m.isTransient()
}

// isTransient reports whether the model solves a time-dependent step, which needs the
// density and specific heat for the transient heat-capacity term.
func (m *AnalysisModel) isTransient() bool { return m.Transient != nil }

// isCoupledThermal reports whether the analysis solves temperature and displacement together.
func (m *AnalysisModel) isCoupledThermal() bool { return m.Analysis == AnalysisCoupledThermal }

// needsExpansion reports whether *EXPANSION must be written: an uncoupled thermal-stress load
// or a coupled thermomechanical study both produce thermal strain against it.
func (m *AnalysisModel) needsExpansion() bool { return m.Thermal != nil || m.isCoupledThermal() }

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
