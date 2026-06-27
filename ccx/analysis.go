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
	// AnalysisThermomech is an uncoupled thermal-stress analysis (prescribed temperature).
	AnalysisThermomech AnalysisType = "thermomech"
	// AnalysisCoupledThermal is a coupled temperature-displacement analysis
	// (*COUPLED TEMPERATURE-DISPLACEMENT): the temperature field is solved from the
	// prescribed face temperatures and conduction, and its (non-uniform) thermal expansion
	// drives the displacement/stress in the same step — steady-state, or transient when a
	// total time is set.
	AnalysisCoupledThermal AnalysisType = "coupled thermal-displacement"
	// AnalysisHeatTransfer solves the steady-state temperature field (*HEAT TRANSFER).
	AnalysisHeatTransfer AnalysisType = "heat transfer"
	// AnalysisElectromagnetic is an electrostatic / electric-conduction analysis: the steady
	// electric potential in a conductor, solved on the part's solid mesh via CalculiX's
	// electric-thermal analogy (potential ↔ temperature DOF 11, electrical conductivity ↔
	// *CONDUCTIVITY). True magnetostatics/induction needs the surrounding air meshed and is
	// out of scope for a solid-only mesh.
	AnalysisElectromagnetic AnalysisType = "electromagnetic"
)

// analysisTypeOptions lists the panel dropdown choices in display order.
func analysisTypeOptions() []string {
	return []string{
		string(AnalysisStatic),
		string(AnalysisFrequency),
		string(AnalysisBuckling),
		string(AnalysisThermomech),
		string(AnalysisCoupledThermal),
		string(AnalysisHeatTransfer),
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

// LoadType selects how the picked load faces are loaded.
type LoadType string

const (
	// LoadForce applies a total force (N) over the loaded faces (*CLOAD).
	LoadForce LoadType = "force"
	// LoadPressure applies a uniform pressure (MPa) normal to the loaded faces (*DLOAD).
	LoadPressure LoadType = "pressure"
	// LoadGravity applies self-weight over the whole body (*DLOAD GRAV); no loaded face.
	LoadGravity LoadType = "gravity"
	// LoadCentrifugal applies a centrifugal body force over the whole body (*DLOAD CENTRIF)
	// for a part rotating about an axis; no loaded face.
	LoadCentrifugal LoadType = "centrifugal"
	// LoadDisplacement enforces a prescribed displacement on the loaded face(s) (a non-zero
	// *BOUNDARY on DOF 3), pulling/pushing them a set distance instead of applying a force.
	LoadDisplacement LoadType = "displacement"
)

// loadTypeOptions lists the panel dropdown choices in display order.
func loadTypeOptions() []string {
	return []string{string(LoadForce), string(LoadPressure), string(LoadGravity), string(LoadCentrifugal), string(LoadDisplacement)}
}

// EMDrive selects how an electromagnetic (electric-conduction) study is driven.
type EMDrive string

const (
	// EMVoltage prescribes the potential on both ends (applied voltage + ground): the
	// Laplace problem, where the potential field is independent of conductivity.
	EMVoltage EMDrive = "voltage"
	// EMCurrent injects a current density on the loaded face(s) (*DFLUX) and grounds the
	// first face: the Neumann problem, where the potential scales with 1/conductivity.
	EMCurrent EMDrive = "current"
)

// emDriveOptions lists the panel dropdown choices in display order.
func emDriveOptions() []string {
	return []string{string(EMVoltage), string(EMCurrent)}
}

// HeatDrive selects how the loaded faces of a heat-transfer study exchange heat.
type HeatDrive string

const (
	// HeatDriveFlux applies a fixed surface heat flux (*DFLUX).
	HeatDriveFlux HeatDrive = "flux"
	// HeatDriveFilm applies convective film cooling/heating (*FILM): q = h·(T − T_sink).
	HeatDriveFilm HeatDrive = "convection"
)

// heatDriveOptions lists the panel dropdown choices in display order.
func heatDriveOptions() []string {
	return []string{string(HeatDriveFlux), string(HeatDriveFilm)}
}

// standardGravityMMs2 is one g in CalculiX mm/s^2 units.
const standardGravityMMs2 = 9810.0

// ResultFieldKind selects which scalar field a stress (static/thermal-stress) result is
// coloured by.
type ResultFieldKind string

const (
	// ResultVonMises colours by von Mises equivalent stress (the default).
	ResultVonMises ResultFieldKind = "von Mises stress"
	// ResultDisplacement colours by displacement magnitude.
	ResultDisplacement ResultFieldKind = "displacement"
	// ResultMaxPrincipal colours by the maximum (most tensile) principal stress.
	ResultMaxPrincipal ResultFieldKind = "max principal stress"
	// ResultMinPrincipal colours by the minimum (most compressive) principal stress.
	ResultMinPrincipal ResultFieldKind = "min principal stress"
)

// resultFieldOptions lists the panel dropdown choices in display order.
func resultFieldOptions() []string {
	return []string{
		string(ResultVonMises),
		string(ResultDisplacement),
		string(ResultMaxPrincipal),
		string(ResultMinPrincipal),
	}
}

// StudySettings holds the panel-editable study parameters. Which faces carry the load
// vs the support is resolved from the host selection at run time (first selected face is
// the fixed support, the rest carry the load); the load magnitude and the material come
// from here until a richer setup UI and per-body material resolution land.
type StudySettings struct {
	Analysis     AnalysisType // *STEP procedure
	MeshSizeMM   float64      // gmsh characteristic length (mm); 0 = auto
	ElementOrder ElementOrder // tet element order
	DeformScale  float64      // displacement magnification for the deformed-shape render; 0 = auto

	YoungGPa       float64         // material Young's modulus (GPa)
	Poisson        float64         // material Poisson's ratio
	DensityGCm3    float64         // material density (g/cm^3); used by gravity and frequency
	LoadType       LoadType        // how the loaded faces are loaded
	LoadN          float64         // total force on the loaded faces (N), in -Z, for LoadForce
	PressureMPa    float64         // pressure on the loaded faces (MPa) for LoadPressure
	GravityG       float64         // gravity as a multiple of standard g for LoadGravity
	RotationRadS   float64         // angular velocity (rad/s) about the Z axis for LoadCentrifugal
	DisplacementMM float64         // enforced displacement (mm, +Z) on the loaded faces for LoadDisplacement
	Eigenmodes     int             // number of modes/factors for frequency and buckling analyses
	ThermalAlpha   float64         // thermal expansion coefficient (1/K) for thermomech
	DeltaK         float64         // temperature change (K) for a thermomech study
	Conductivity   float64         // thermal conductivity (consistent units) for heat transfer
	ColdTempK      float64         // prescribed temperature on the first (support) face (K)
	HeatFluxQ      float64         // surface heat flux on the remaining faces (heat transfer)
	HeatDriveMode  HeatDrive       // how the loaded faces of a heat study exchange heat (flux vs convection)
	FilmCoeff      float64         // convective film coefficient h (consistent units) for HeatDriveFilm
	SinkTempK      float64         // ambient/sink temperature for convection (K)
	ResultField    ResultFieldKind // which scalar field a stress result is coloured by

	VoltageV        float64 // prescribed potential on the first face for an electrostatic study (V)
	ElectricalSigma float64 // electrical conductivity (consistent units) for an electrostatic study
	EMDriveMode     EMDrive // how an electromagnetic study is driven (applied voltage vs injected current)
	CurrentDensity  float64 // injected current density on the loaded faces (consistent units) for EMCurrent

	SpecificHeat   float64 // specific heat capacity (consistent units) for transient coupled analysis
	TransientTimeS float64 // total time (s) for a transient coupled study; 0 = steady state
}

// eigenmodeCount returns the requested number of modes, clamped to a sensible minimum.
func (s StudySettings) eigenmodeCount() int {
	if s.Eigenmodes < 1 {
		return 6
	}
	return s.Eigenmodes
}

// defaultSettings returns the v1 defaults: linear-static, quadratic tets, auto sizing,
// mild-steel-like material, and a unit force load.
func defaultSettings() StudySettings {
	return StudySettings{
		Analysis:        AnalysisStatic,
		MeshSizeMM:      0,
		ElementOrder:    QuadraticTet,
		DeformScale:     0,
		YoungGPa:        210,
		Poisson:         0.3,
		DensityGCm3:     7.85,
		LoadType:        LoadForce,
		LoadN:           100,
		PressureMPa:     1,
		GravityG:        1,
		RotationRadS:    100,
		DisplacementMM:  0.1,
		Eigenmodes:      6,
		ThermalAlpha:    1.2e-5,
		DeltaK:          100,
		Conductivity:    50,
		ColdTempK:       0,
		HeatFluxQ:       50,
		HeatDriveMode:   HeatDriveFlux,
		FilmCoeff:       0.5,
		SinkTempK:       0,
		ResultField:     ResultVonMises,
		VoltageV:        5,
		ElectricalSigma: 1,
		EMDriveMode:     EMVoltage,
		CurrentDensity:  1,
		SpecificHeat:    5e8, // steel-like, consistent units (mm,t,s): ~0.5 J/(g·K)
		TransientTimeS:  0,   // steady state by default
	}
}

// material returns the settings' material as CalculiX-unit elastic properties (density in
// t/mm^3, only consumed by body loads).
func (s StudySettings) material() MaterialProps {
	return MaterialProps{
		Name:            "MATERIAL",
		YoungMPa:        s.YoungGPa * gpaToMPa,
		Poisson:         s.Poisson,
		DensityTonneMM3: s.DensityGCm3 * gCm3ToTonneMM3,
		ExpansionPerK:   s.ThermalAlpha,
		Conductivity:    s.Conductivity,
		ElectricalSigma: s.ElectricalSigma,
		SpecificHeat:    s.SpecificHeat,
	}
}
