// SPDX-License-Identifier: GPL-2.0-only

package ccx

// writeMaterial emits the *MATERIAL block, including only the properties the analysis
// needs: elastic constants for the mechanical analyses, density for a body load or modal
// study, expansion for thermal stress, and conductivity for heat transfer. Unneeded
// properties are omitted to avoid unused-property notes from the solver.
func writeMaterial(d *deckBuf, m *AnalysisModel) {
	mat := m.Material
	d.line("*MATERIAL, NAME=%s", mat.Name)
	if m.Analysis != AnalysisHeatTransfer {
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
	if m.Analysis == AnalysisHeatTransfer {
		d.line("*CONDUCTIVITY")
		d.line("%.10g", mat.Conductivity)
	}
}

// writeSolidSection assigns the material to every solid element via the Eall set.
func writeSolidSection(d *deckBuf, matName string) {
	d.line("*SOLID SECTION, ELSET=%s, MATERIAL=%s", allElementsSet, matName)
}
