// SPDX-License-Identifier: GPL-2.0-only

package ccx

// ConstraintWriter emits one load/boundary-condition kind in two phases: WriteSets adds
// any node/element sets BEFORE the *STEP, and WriteStep adds the load/BC card INSIDE the
// step. This is the Go analog of CalculiX's per-constraint deck modules; new kinds
// (pressure, gravity, temperature, …) register a writer rather than editing WriteDeck.
type ConstraintWriter interface {
	WriteSets(d *deckBuf)
	WriteStep(d *deckBuf)
}

// constraintWriters builds the ordered writer list for a model. Boundary conditions
// precede loads, matching CalculiX's customary deck ordering.
func constraintWriters(m *AnalysisModel) []ConstraintWriter {
	var cs []ConstraintWriter
	for i := range m.Fixed {
		cs = append(cs, fixedWriter{c: &m.Fixed[i]})
	}
	for i := range m.Forces {
		cs = append(cs, forceWriter{c: &m.Forces[i]})
	}
	for i := range m.Pressures {
		cs = append(cs, pressureWriter{c: &m.Pressures[i]})
	}
	if m.Gravity != nil {
		cs = append(cs, gravityWriter{c: m.Gravity})
	}
	if m.Thermal != nil {
		cs = append(cs, thermalWriter{c: m.Thermal})
	}
	for i := range m.Temperatures {
		cs = append(cs, temperatureWriter{c: &m.Temperatures[i]})
	}
	for i := range m.HeatFluxes {
		cs = append(cs, heatFluxWriter{c: &m.HeatFluxes[i]})
	}
	return cs
}

// fixedWriter emits a fully fixed boundary condition: an *NSET of the pinned nodes and a
// *BOUNDARY card constraining their translational DOFs.
type fixedWriter struct{ c *FixedConstraint }

func (f fixedWriter) WriteSets(d *deckBuf) {
	d.line("*NSET, NSET=%s", f.c.Name)
	writeNodeSet(d, f.c.Nodes)
}

func (f fixedWriter) WriteStep(d *deckBuf) {
	d.line("*BOUNDARY")
	d.line("%s, %d, %d", f.c.Name, f.c.DOFLow, f.c.DOFHigh)
}

// forceWriter emits a *CLOAD spreading the total force equally over its node set, one
// card per node per non-zero direction component.
type forceWriter struct{ c *ForceLoad }

func (forceWriter) WriteSets(*deckBuf) {} // forces are written per node; no set needed

func (f forceWriter) WriteStep(d *deckBuf) {
	if len(f.c.Nodes) == 0 {
		return
	}
	per := f.c.TotalN / float64(len(f.c.Nodes))
	d.line("*CLOAD")
	for _, n := range f.c.Nodes {
		for dof := 1; dof <= 3; dof++ {
			if comp := f.c.Dir[dof-1]; comp != 0 {
				d.line("%d, %d, %.10g", n, dof, per*comp)
			}
		}
	}
}

// writeNodeSet writes node ids in CalculiX-friendly rows (the .inp line length is
// bounded, so the set is chunked).
func writeNodeSet(d *deckBuf, nodes []int) {
	const perLine = 10
	for i := 0; i < len(nodes); i += perLine {
		end := i + perLine
		if end > len(nodes) {
			end = len(nodes)
		}
		d.line("%s", joinInts(nodes[i:end]))
	}
}
