// SPDX-License-Identifier: GPL-2.0-only

package ccx

// gravityWriter emits a *DLOAD GRAV body force over every element (Eall): an acceleration
// magnitude and a unit direction. It relies on *DENSITY being written for the material.
type gravityWriter struct{ c *GravityLoad }

func (gravityWriter) WriteSets(*deckBuf) {} // gravity acts on Eall; no set needed

func (g gravityWriter) WriteStep(d *deckBuf) {
	if g.c.Accel == 0 {
		return
	}
	dir := normalize(g.c.Dir)
	d.line("*DLOAD")
	d.line("%s, GRAV, %.10g, %.10g, %.10g, %.10g", allElementsSet, g.c.Accel, dir[0], dir[1], dir[2])
}
