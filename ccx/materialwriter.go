// SPDX-License-Identifier: GPL-2.0-only

package ccx

// writeMaterial emits the *MATERIAL block: linear-elastic constants always, and density
// only when a body load needs it (kept out of pure nodal-force statics to avoid an
// unused-property note from the solver).
func writeMaterial(d *deckBuf, m MaterialProps, withDensity bool) {
	d.line("*MATERIAL, NAME=%s", m.Name)
	d.line("*ELASTIC")
	d.line("%.10g, %.10g", m.YoungMPa, m.Poisson)
	if withDensity {
		d.line("*DENSITY")
		d.line("%.10g", m.DensityTonneMM3)
	}
}

// writeSolidSection assigns the material to every solid element via the Eall set.
func writeSolidSection(d *deckBuf, matName string) {
	d.line("*SOLID SECTION, ELSET=%s, MATERIAL=%s", allElementsSet, matName)
}
