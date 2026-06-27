// SPDX-License-Identifier: GPL-2.0-only

package ccx

// centrifugalWriter emits a *DLOAD CENTRIF body force over every element (Eall): the angular
// velocity squared (rad/s)^2, a point on the rotation axis, and the axis direction. CalculiX
// applies ρ·ω²·r outward from the axis to each element, so it relies on *DENSITY.
type centrifugalWriter struct{ c *CentrifugalLoad }

func (centrifugalWriter) WriteSets(*deckBuf) {} // centrifugal acts on Eall; no set needed

func (c centrifugalWriter) WriteStep(d *deckBuf) {
	if c.c.Omega2 == 0 {
		return
	}
	p := c.c.AxisPoint
	n := normalize(c.c.AxisDir)
	d.line("*DLOAD")
	d.line("%s, CENTRIF, %.10g, %.10g, %.10g, %.10g, %.10g, %.10g, %.10g",
		allElementsSet, c.c.Omega2, p[0], p[1], p[2], n[0], n[1], n[2])
}
