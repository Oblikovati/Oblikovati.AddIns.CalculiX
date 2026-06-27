// SPDX-License-Identifier: GPL-2.0-only

package ccx

// writeMaterial emits the *MATERIAL block: linear-elastic constants always, density only
// when a body load needs it, and the thermal expansion coefficient only for a thermal
// study (kept out of other analyses to avoid unused-property notes from the solver).
func writeMaterial(d *deckBuf, m MaterialProps, withDensity, withExpansion bool) {
	d.line("*MATERIAL, NAME=%s", m.Name)
	d.line("*ELASTIC")
	d.line("%.10g, %.10g", m.YoungMPa, m.Poisson)
	if withDensity {
		d.line("*DENSITY")
		d.line("%.10g", m.DensityTonneMM3)
	}
	if withExpansion {
		d.line("*EXPANSION, ZERO=0.")
		d.line("%.10g", m.ExpansionPerK)
	}
}

// writeSolidSection assigns the material to every solid element via the Eall set.
func writeSolidSection(d *deckBuf, matName string) {
	d.line("*SOLID SECTION, ELSET=%s, MATERIAL=%s", allElementsSet, matName)
}
