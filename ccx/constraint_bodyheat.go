// SPDX-License-Identifier: GPL-2.0-only

package ccx

// bodyHeatWriter emits a *DFLUX BF body heat flux over every element (Eall): a uniform
// volumetric internal heat generation (power per unit volume) for a heat-transfer analysis.
// (Body heat is a *DFLUX with the BF label, not a *DLOAD — *DLOAD is the mechanical body
// force; in a heat step the volumetric flux is a thermal *DFLUX.)
type bodyHeatWriter struct{ c *BodyHeat }

func (bodyHeatWriter) WriteSets(*deckBuf) {} // body heat acts on Eall; no set needed

func (w bodyHeatWriter) WriteStep(d *deckBuf) {
	if w.c.Rate == 0 {
		return
	}
	d.line("*DFLUX")
	d.line("%s, BF, %.10g", allElementsSet, w.c.Rate)
}
