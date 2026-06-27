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
		writeMaterialBlock(d, mat, m)
	}
}

// writeMaterialBlock emits one *MATERIAL and the property cards the analysis requires:
// elastic constants (mechanical analyses), density (body load / modal / transient), expansion
// (thermal stress and coupled), conductivity (heat / electrostatic / coupled), and specific
// heat (transient coupled, for the heat-capacity term).
func writeMaterialBlock(d *deckBuf, mat MaterialProps, m *AnalysisModel) {
	d.line("*MATERIAL, NAME=%s", mat.Name)
	if needsElastic(m.Analysis) {
		writeElastic(d, mat)
		if m.hasPlasticity() && mat.YieldMPa > 0 {
			// Perfect (ideal) plasticity: one stress/plastic-strain point at zero plastic
			// strain caps the stress at the yield value (no hardening). *PLASTIC must follow
			// *ELASTIC,TYPE=ISO.
			d.line("*PLASTIC")
			d.line("%.10g, 0.", mat.YieldMPa)
		}
	}
	if m.needsDensity() {
		d.line("*DENSITY")
		d.line("%.10g", mat.DensityTonneMM3)
	}
	if m.needsExpansion() {
		d.line("*EXPANSION, ZERO=0.")
		d.line("%.10g", mat.ExpansionPerK)
	}
	if cond, ok := conductivityForMaterial(mat, m.Analysis); ok {
		d.line("*CONDUCTIVITY")
		d.line("%.10g", cond)
	}
	if m.isTransient() {
		d.line("*SPECIFIC HEAT")
		d.line("%.10g", mat.SpecificHeat)
	}
}

// writeElastic emits the elastic constitutive card: an isotropic *ELASTIC (Young + Poisson),
// or *ELASTIC, TYPE=ENGINEERING CONSTANTS with the nine orthotropic constants (CalculiX caps a
// data line at eight values, so G23 spills to a second line).
func writeElastic(d *deckBuf, mat MaterialProps) {
	if h := mat.Hyper; h != nil {
		d.line("*HYPERELASTIC, NEO HOOKE")
		d.line("%.10g, %.10g", h.C10, h.D1)
		return
	}
	if len(mat.ElasticTable) > 0 {
		d.line("*ELASTIC")
		for _, p := range mat.ElasticTable {
			d.line("%.10g, %.10g, %.10g", p.YoungMPa, p.Poisson, p.TempK)
		}
		return
	}
	if o := mat.Ortho; o != nil {
		d.line("*ELASTIC, TYPE=ENGINEERING CONSTANTS")
		d.line("%.10g, %.10g, %.10g, %.10g, %.10g, %.10g, %.10g, %.10g",
			o.E1MPa, o.E2MPa, o.E3MPa, o.Nu12, o.Nu13, o.Nu23, o.G12MPa, o.G13MPa)
		d.line("%.10g", o.G23MPa)
		return
	}
	d.line("*ELASTIC")
	d.line("%.10g, %.10g", mat.YoungMPa, mat.Poisson)
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
	case AnalysisHeatTransfer, AnalysisCoupledThermal:
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
