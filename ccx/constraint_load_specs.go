// SPDX-License-Identifier: GPL-2.0-only

package ccx

// ForceSpec spreads a total force over a selected face (the *CLOAD idiom).
type ForceSpec struct {
	Name   string
	Faces  []string
	Dir    [3]float64
	TotalN float64
}

func (ForceSpec) Kind() ConstraintKind { return KindForce }

func (s ForceSpec) Resolve(rc *ResolveContext) {
	rc.Model.Forces = append(rc.Model.Forces, ForceLoad{
		Name: s.Name, Nodes: groupNodes(rc.Groups, s.Faces), Dir: s.Dir, TotalN: s.TotalN,
	})
}

// PressureSpec applies a uniform pressure normal to a selected face (*DLOAD Pn).
type PressureSpec struct {
	Name  string
	Faces []string
	MPa   float64
}

func (PressureSpec) Kind() ConstraintKind { return KindPressure }

func (s PressureSpec) Resolve(rc *ResolveContext) {
	rc.Model.Pressures = append(rc.Model.Pressures, PressureLoad{
		Name: s.Name, Faces: groupElemFaces(rc.Groups, s.Faces), MPa: s.MPa,
	})
}

// HydrostaticSpec applies a depth-varying fluid pressure normal to a selected face: the pressure
// at each element-face is computed from its centroid depth below the free surface.
type HydrostaticSpec struct {
	Name        string
	Faces       []string
	GradientMPa float64 // pressure gradient γ (MPa/mm = ρg)
	SurfaceZ    float64 // fluid free-surface height (mm)
}

func (HydrostaticSpec) Kind() ConstraintKind { return KindHydrostatic }

func (s HydrostaticSpec) Resolve(rc *ResolveContext) {
	faces := groupElemFaces(rc.Groups, s.Faces)
	rc.Model.Pressures = append(rc.Model.Pressures, PressureLoad{Name: s.Name, Faces: faces,
		PerFaceMPa: hydrostaticPressures(rc.Mesh, faces, s.GradientMPa, s.SurfaceZ)})
}

// DisplacementSpec enforces a prescribed displacement on a selected face along one DOF.
type DisplacementSpec struct {
	Name  string
	Faces []string
	DOF   int
	Value float64
}

func (DisplacementSpec) Kind() ConstraintKind { return KindDisplacement }

func (s DisplacementSpec) Resolve(rc *ResolveContext) {
	rc.Model.Displacements = append(rc.Model.Displacements, DisplacementBC{
		Name: s.Name, Nodes: groupNodes(rc.Groups, s.Faces), DOF: s.DOF, Value: s.Value,
	})
}

// GravitySpec applies self-weight over the whole body; it needs no selected face.
type GravitySpec struct {
	Accel float64
	Dir   [3]float64
}

func (GravitySpec) Kind() ConstraintKind { return KindGravity }

func (s GravitySpec) Resolve(rc *ResolveContext) {
	rc.Model.Gravity = &GravityLoad{Accel: s.Accel, Dir: s.Dir}
}

// CentrifugalSpec applies a rotational body force about an axis over the whole body.
type CentrifugalSpec struct {
	Omega2    float64
	AxisPoint [3]float64
	AxisDir   [3]float64
}

func (CentrifugalSpec) Kind() ConstraintKind { return KindCentrifugal }

func (s CentrifugalSpec) Resolve(rc *ResolveContext) {
	rc.Model.Centrifugal = &CentrifugalLoad{Omega2: s.Omega2, AxisPoint: s.AxisPoint, AxisDir: s.AxisDir}
}

// ThermalLoadSpec applies a uniform temperature change over the whole body (thermal-stress).
type ThermalLoadSpec struct {
	DeltaK float64
}

func (ThermalLoadSpec) Kind() ConstraintKind { return KindThermalLoad }

func (s ThermalLoadSpec) Resolve(rc *ResolveContext) {
	rc.Model.Thermal = &ThermalLoad{DeltaK: s.DeltaK}
}
