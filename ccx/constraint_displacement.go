// SPDX-License-Identifier: GPL-2.0-only

package ccx

// displacementWriter enforces a prescribed displacement on a node set: an *NSET plus a
// *BOUNDARY card giving the single DOF its non-zero value (the difference from a fixed
// support, which omits the value and so pins the DOF at zero).
type displacementWriter struct{ c *DisplacementBC }

func (w displacementWriter) WriteSets(d *deckBuf) {
	d.line("*NSET, NSET=%s", w.c.Name)
	writeNodeSet(d, w.c.Nodes)
}

func (w displacementWriter) WriteStep(d *deckBuf) {
	d.line("*BOUNDARY")
	d.line("%s, %d, %d, %.10g", w.c.Name, w.c.DOF, w.c.DOF, w.c.Value)
}
