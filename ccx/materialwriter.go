// SPDX-License-Identifier: GPL-2.0-only

package ccx

// writeMaterial emits the *MATERIAL block, including only the properties the analysis
// needs: elastic constants for the mechanical analyses, density for a body load or modal
// study, expansion for thermal stress, and conductivity for heat transfer or the
// electrostatic analogy. Unneeded properties are omitted to avoid unused-property notes
// from the solver.
func writeMaterial(d *deckBuf, m *AnalysisModel) {
	mat := m.Material
	d.line("*MATERIAL, NAME=%s", mat.Name)
	if needsElastic(m.Analysis) {
		d.line("*ELASTIC")
		d.line("%.10g, %.10g", mat.YoungMPa, mat.Poisson)
	}
	if m.needsDensity() {
		d.line("*DENSITY")
		d.line("%.10g", mat.DensityTonneMM3)
	}
	if m.Thermal != nil {
		d.line("*EXPANSION, ZERO=0.")
		d.line("%.10g", mat.ExpansionPerK)
	}
	if cond, ok := conductivityFor(m); ok {
		d.line("*CONDUCTIVITY")
		d.line("%.10g", cond)
	}
}

// needsElastic reports whether the analysis solves a mechanical (displacement) field that
// requires elastic constants. The two field-only analyses — heat transfer and its
// electrostatic analogy — do not.
func needsElastic(a AnalysisType) bool {
	return a != AnalysisHeatTransfer && a != AnalysisElectromagnetic
}

// conductivityFor returns the *CONDUCTIVITY coefficient for the analyses that need one:
// the thermal conductivity for heat transfer, the electrical conductivity for the
// electrostatic analogy. The boolean is false for analyses that write no conductivity.
func conductivityFor(m *AnalysisModel) (float64, bool) {
	switch m.Analysis {
	case AnalysisHeatTransfer:
		return m.Material.Conductivity, true
	case AnalysisElectromagnetic:
		return m.Material.ElectricalSigma, true
	default:
		return 0, false
	}
}

// writeSolidSection assigns the material to every solid element via the Eall set.
func writeSolidSection(d *deckBuf, matName string) {
	d.line("*SOLID SECTION, ELSET=%s, MATERIAL=%s", allElementsSet, matName)
}
