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

// writeInitialTemperature emits the *INITIAL CONDITIONS temperature on all nodes — the
// stress-free reference for thermal expansion and the starting field for a transient solve.
// A coupled study always writes it (a transient *COUPLED TEMPERATURE-DISPLACEMENT step is a
// fatal error without an initial temperature, even at zero); other analyses write it only
// when a non-zero reference is set. The uncoupled thermomech path writes its own zero
// reference through thermalWriter.
func writeInitialTemperature(d *deckBuf, m *AnalysisModel) {
	if !m.isCoupledThermal() && m.InitialTempK == 0 {
		return
	}
	d.line("*INITIAL CONDITIONS, TYPE=TEMPERATURE")
	d.line("%s, %.10g", allNodesSet, m.InitialTempK)
}
