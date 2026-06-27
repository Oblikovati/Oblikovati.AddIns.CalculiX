// SPDX-License-Identifier: GPL-2.0-only

package ccx

// writeMaterials emits one *MATERIAL block per distinct material the model uses (a uniform
// study writes one; a multi-body part writes one per material). Each block includes only the
// properties the analysis needs: elastic constants for the mechanical analyses, density for a
// body load or modal study, expansion for thermal stress, and conductivity for heat transfer
// or the electrostatic analogy. Unneeded properties are omitted to avoid unused-property
// notes from the solver.
func writeMaterials(d *deckBuf, m *AnalysisModel) {
	for _, mat := range m.distinctMaterials() {
		writeMaterialBlock(d, mat, m.Analysis, m.needsDensity(), m.Thermal != nil)
	}
}

// writeMaterialBlock emits one *MATERIAL and the property cards the analysis requires.
func writeMaterialBlock(d *deckBuf, mat MaterialProps, a AnalysisType, density, thermal bool) {
	d.line("*MATERIAL, NAME=%s", mat.Name)
	if needsElastic(a) {
		d.line("*ELASTIC")
		d.line("%.10g, %.10g", mat.YoungMPa, mat.Poisson)
	}
	if density {
		d.line("*DENSITY")
		d.line("%.10g", mat.DensityTonneMM3)
	}
	if thermal {
		d.line("*EXPANSION, ZERO=0.")
		d.line("%.10g", mat.ExpansionPerK)
	}
	if cond, ok := conductivityForMaterial(mat, a); ok {
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

// conductivityForMaterial returns the *CONDUCTIVITY coefficient for the analyses that need
// one: the thermal conductivity for heat transfer, the electrical conductivity for the
// electrostatic analogy. The boolean is false for analyses that write no conductivity.
func conductivityForMaterial(mat MaterialProps, a AnalysisType) (float64, bool) {
	switch a {
	case AnalysisHeatTransfer:
		return mat.Conductivity, true
	case AnalysisElectromagnetic:
		return mat.ElectricalSigma, true
	default:
		return 0, false
	}
}

// writeSolidSections assigns each material section to its element set — one *SOLID SECTION
// per body (or the single Eall set of a uniform study).
func writeSolidSections(d *deckBuf, sections []MaterialSection) {
	for _, sec := range sections {
		d.line("*SOLID SECTION, ELSET=%s, MATERIAL=%s", sec.ElsetName, sec.Material.Name)
	}
}
