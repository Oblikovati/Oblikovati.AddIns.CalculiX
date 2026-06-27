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

// checkMaterial validates the elastic constants and the extra properties a body/thermal
// load requires.
func checkMaterial(m *AnalysisModel) error {
	if m.Material.YoungMPa <= 0 {
		return errors.New("set a positive Young's modulus")
	}
	if m.Material.Poisson <= -1 || m.Material.Poisson >= 0.5 {
		return fmt.Errorf("the Poisson's ratio %.3g is outside the valid range (-1, 0.5)", m.Material.Poisson)
	}
	if m.Gravity != nil && m.Material.DensityTonneMM3 <= 0 {
		return errors.New("a gravity load needs a positive material density")
	}
	if m.Thermal != nil && m.Material.ExpansionPerK <= 0 {
		return errors.New("a thermal study needs a positive thermal expansion coefficient")
	}
	return nil
}

// modal reports whether the analysis is a free eigenvalue problem (no applied load).
func modal(a AnalysisType) bool {
	return a == AnalysisFrequency
}

// hasSupportNodes reports whether at least one fixed constraint pins some nodes.
func hasSupportNodes(m *AnalysisModel) bool {
	for _, f := range m.Fixed {
		if len(f.Nodes) > 0 {
			return true
		}
	}
	return false
}

// hasLoad reports whether the model carries any non-zero load.
func hasLoad(m *AnalysisModel) bool {
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
	if m.Gravity != nil && m.Gravity.Accel != 0 {
		return true
	}
	return m.Thermal != nil && m.Thermal.DeltaK != 0
}
