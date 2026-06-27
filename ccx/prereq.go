// SPDX-License-Identifier: GPL-2.0-only

package ccx

import (
	"errors"
	"fmt"
)

// checkPrerequisites validates a resolved model before it is handed to the solver, turning
// the common setup mistakes (no mesh, no material, no support, no load) into a clear
// up-front message instead of a cryptic solver failure or a silently empty result.
func checkPrerequisites(m *AnalysisModel) error {
	if m.Mesh == nil || len(m.Mesh.Elements) == 0 {
		return errors.New("the body did not mesh into any elements")
	}
	if m.Analysis == AnalysisHeatTransfer {
		return checkHeatPrerequisites(m)
	}
	if m.Analysis == AnalysisElectromagnetic {
		return checkElectrostaticPrerequisites(m)
	}
	if m.Analysis == AnalysisCoupledThermal {
		return checkCoupledPrerequisites(m)
	}
	if err := checkMaterial(m); err != nil {
		return err
	}
	if !hasSupportNodes(m) {
		return errors.New("the support face resolved to no mesh nodes — pick a face of the part")
	}
	if !modal(m.Analysis) && !hasLoad(m) {
		return errors.New("no load applied — set a non-zero force, pressure, gravity, or temperature change")
	}
	return nil
}

// checkCoupledPrerequisites validates a coupled temperature-displacement model: it needs
// elastic constants, a positive thermal expansion and conductivity, a mechanical support, a
// prescribed-temperature face, and a temperature difference to drive expansion (plus a
// positive specific heat when transient).
func checkCoupledPrerequisites(m *AnalysisModel) error {
	for _, mat := range m.distinctMaterials() {
		if err := checkMaterialProps(mat, false, true); err != nil {
			return err
		}
		if mat.Conductivity <= 0 {
			return fmt.Errorf("material %q: a coupled study needs a positive thermal conductivity", mat.Name)
		}
		if m.isTransient() && mat.SpecificHeat <= 0 {
			return fmt.Errorf("material %q: a transient study needs a positive specific heat", mat.Name)
		}
	}
	if !hasSupportNodes(m) {
		return errors.New("the support face resolved to no mesh nodes — pick a face of the part")
	}
	if !hasTemperatureBC(m) {
		return errors.New("the temperature face resolved to no mesh nodes — pick a face of the part")
	}
	if !hasTemperatureGradient(m) {
		return errors.New("no temperature difference — set a non-zero temperature change ΔT between the faces")
	}
	return nil
}

// hasTemperatureGradient reports whether the prescribed temperatures differ (a uniform field
// produces no thermal stress in a free body).
func hasTemperatureGradient(m *AnalysisModel) bool {
	if len(m.Temperatures) < 2 {
		return false
	}
	first := m.Temperatures[0].TempK
	for _, t := range m.Temperatures[1:] {
		if t.TempK != first {
			return true
		}
	}
	return false
}

// checkHeatPrerequisites validates a heat-transfer model: it needs conductivity, a
// prescribed-temperature face, and a heat source (flux).
func checkHeatPrerequisites(m *AnalysisModel) error {
	for _, mat := range m.distinctMaterials() {
		if mat.Conductivity <= 0 {
			return fmt.Errorf("material %q: a heat-transfer study needs a positive thermal conductivity", mat.Name)
		}
	}
	if !hasTemperatureBC(m) {
		return errors.New("the temperature face resolved to no mesh nodes — pick a face of the part")
	}
	if len(m.Films) > 0 {
		if !hasFilm(m) {
			return errors.New("no convection — set a non-zero film coefficient on the loaded face(s)")
		}
		return nil
	}
	if m.BodyHeat != nil {
		if m.BodyHeat.Rate == 0 {
			return errors.New("no internal heat — set a non-zero body heat generation rate")
		}
		return nil
	}
	if len(m.Radiations) > 0 {
		if !hasRadiation(m) {
			return errors.New("no radiation — set a non-zero emissivity on the loaded face(s)")
		}
		return nil
	}
	if !hasHeatSource(m) {
		return errors.New("no heat source — set a non-zero heat flux on the loaded face(s)")
	}
	return nil
}

// hasFilm reports whether a non-zero convective film exchange is applied.
func hasFilm(m *AnalysisModel) bool {
	for _, f := range m.Films {
		if f.Coeff != 0 && len(f.Faces) > 0 {
			return true
		}
	}
	return false
}

// checkElectrostaticPrerequisites validates an electric-conduction model: it needs a positive
// electrical conductivity, a grounded/prescribed face that resolved to nodes, and a non-zero
// drive — either a potential difference (voltage drive) or an injected current (current drive,
// signalled by the presence of surface fluxes).
func checkElectrostaticPrerequisites(m *AnalysisModel) error {
	for _, mat := range m.distinctMaterials() {
		if mat.ElectricalSigma <= 0 {
			return fmt.Errorf("material %q: an electrostatic study needs a positive electrical conductivity", mat.Name)
		}
	}
	if !hasTemperatureBC(m) {
		return errors.New("the potential face resolved to no mesh nodes — pick a face of the part")
	}
	if len(m.HeatFluxes) > 0 {
		if !hasHeatSource(m) {
			return errors.New("no current — set a non-zero current density on the loaded face(s)")
		}
		return nil
	}
	if !hasPotentialDifference(m) {
		return errors.New("no potential difference — set a non-zero applied voltage")
	}
	return nil
}

// hasPotentialDifference reports whether some prescribed potential is non-zero (a zero-only
// set of Dirichlet values produces a trivial, uniformly-zero field).
func hasPotentialDifference(m *AnalysisModel) bool {
	for _, t := range m.Temperatures {
		if t.TempK != 0 && len(t.Nodes) > 0 {
			return true
		}
	}
	return false
}

// hasTemperatureBC reports whether a temperature is prescribed on some nodes.
func hasTemperatureBC(m *AnalysisModel) bool {
	for _, t := range m.Temperatures {
		if len(t.Nodes) > 0 {
			return true
		}
	}
	return false
}

// hasRadiation reports whether a non-zero radiative exchange is applied.
func hasRadiation(m *AnalysisModel) bool {
	for _, r := range m.Radiations {
		if r.Emissivity != 0 && len(r.Faces) > 0 {
			return true
		}
	}
	return false
}

// hasHeatSource reports whether a non-zero heat flux is applied.
func hasHeatSource(m *AnalysisModel) bool {
	for _, h := range m.HeatFluxes {
		if h.Flux != 0 && len(h.Faces) > 0 {
			return true
		}
	}
	return false
}

// checkMaterial validates the elastic constants and the extra properties a body/thermal
// load requires, for every material in the model (a multi-body part has one per body).
func checkMaterial(m *AnalysisModel) error {
	bodyLoad := m.Gravity != nil || m.Centrifugal != nil
	for _, mat := range m.distinctMaterials() {
		if err := checkMaterialProps(mat, bodyLoad, m.Thermal != nil); err != nil {
			return err
		}
	}
	return nil
}

// checkMaterialProps validates one material's elastic constants plus the density/expansion a
// body load (gravity, centrifugal) or thermal study needs.
func checkMaterialProps(mat MaterialProps, bodyLoad, thermal bool) error {
	if h := mat.Hyper; h != nil {
		if h.C10 <= 0 || h.D1 <= 0 {
			return fmt.Errorf("material %q: a Neo-Hookean material needs a positive C10 and D1 (got C10=%.3g, D1=%.3g)", mat.Name, h.C10, h.D1)
		}
		return nil
	}
	if mat.YoungMPa <= 0 {
		return fmt.Errorf("material %q: set a positive Young's modulus", mat.Name)
	}
	if mat.Poisson <= -1 || mat.Poisson >= 0.5 {
		return fmt.Errorf("material %q: the Poisson's ratio %.3g is outside the valid range (-1, 0.5)", mat.Name, mat.Poisson)
	}
	if bodyLoad && mat.DensityTonneMM3 <= 0 {
		return fmt.Errorf("material %q: a body load (gravity/centrifugal) needs a positive material density", mat.Name)
	}
	if thermal && mat.ExpansionPerK <= 0 {
		return fmt.Errorf("material %q: a thermal study needs a positive thermal expansion coefficient", mat.Name)
	}
	return nil
}

// modal reports whether the analysis is a free eigenvalue problem (no applied load).
func modal(a AnalysisType) bool {
	return a == AnalysisFrequency
}

// hasSupportNodes reports whether the model is held against rigid-body motion: a fixed clamp
// or a grounded elastic (spring) foundation on some nodes both count as a support.
func hasSupportNodes(m *AnalysisModel) bool {
	for _, f := range m.Fixed {
		if len(f.Nodes) > 0 {
			return true
		}
	}
	for _, s := range m.Springs {
		if len(s.Nodes) > 0 && s.StiffnessTotal > 0 {
			return true
		}
	}
	return false
}

// hasLoad reports whether the model carries any non-zero load (surface load, body load,
// enforced displacement, or thermal load).
func hasLoad(m *AnalysisModel) bool {
	return hasSurfaceLoad(m) || hasBodyLoad(m) || hasEnforcedDisplacement(m) ||
		(m.Thermal != nil && m.Thermal.DeltaK != 0)
}

// hasSurfaceLoad reports a non-zero force or pressure on some face.
func hasSurfaceLoad(m *AnalysisModel) bool {
	for _, f := range m.Forces {
		if f.TotalN != 0 && len(f.Nodes) > 0 {
			return true
		}
	}
	for _, p := range m.Pressures {
		if p.MPa != 0 && len(p.Faces) > 0 {
			return true
		}
	}
	return false
}

// hasBodyLoad reports a non-zero gravity or centrifugal body force.
func hasBodyLoad(m *AnalysisModel) bool {
	if m.Gravity != nil && m.Gravity.Accel != 0 {
		return true
	}
	return m.Centrifugal != nil && m.Centrifugal.Omega2 != 0
}

// hasEnforcedDisplacement reports a non-zero prescribed displacement on some nodes.
func hasEnforcedDisplacement(m *AnalysisModel) bool {
	for _, dsp := range m.Displacements {
		if dsp.Value != 0 && len(dsp.Nodes) > 0 {
			return true
		}
	}
	return false
}
