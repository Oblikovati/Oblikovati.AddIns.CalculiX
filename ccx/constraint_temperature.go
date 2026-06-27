// SPDX-License-Identifier: GPL-2.0-only

package ccx

// temperatureDOF is the CalculiX temperature degree of freedom (a *BOUNDARY on DOF 11
// prescribes a nodal temperature in a heat-transfer analysis).
const temperatureDOF = 11

// temperatureWriter prescribes a fixed temperature on a node set: an *NSET plus a
// *BOUNDARY on the temperature DOF.
type temperatureWriter struct{ c *TemperatureBC }

func (t temperatureWriter) WriteSets(d *deckBuf) {
	d.line("*NSET, NSET=%s", t.c.Name)
	writeNodeSet(d, t.c.Nodes)
}

func (t temperatureWriter) WriteStep(d *deckBuf) {
	d.line("*BOUNDARY")
	d.line("%s, %d, %d, %.10g", t.c.Name, temperatureDOF, temperatureDOF, t.c.TempK)
}
