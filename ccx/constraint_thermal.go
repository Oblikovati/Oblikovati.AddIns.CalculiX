// SPDX-License-Identifier: GPL-2.0-only

package ccx

// allNodesSet is the node set every node belongs to (defined by the *NODE block).
const allNodesSet = "Nall"

// thermalWriter emits an uncoupled thermal-stress load: an *INITIAL CONDITIONS temperature
// of zero (the stress-free reference) before the step, and a uniform *TEMPERATURE field in
// the step. The resulting thermal strain acts against the material's *EXPANSION.
type thermalWriter struct{ c *ThermalLoad }

func (thermalWriter) WriteSets(d *deckBuf) {
	d.line("*INITIAL CONDITIONS, TYPE=TEMPERATURE")
	d.line("%s, 0.", allNodesSet)
}

func (t thermalWriter) WriteStep(d *deckBuf) {
	d.line("*TEMPERATURE")
	d.line("%s, %.10g", allNodesSet, t.c.DeltaK)
}
