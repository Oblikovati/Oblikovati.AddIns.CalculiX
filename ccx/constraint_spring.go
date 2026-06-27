// SPDX-License-Identifier: GPL-2.0-only

package ccx

import "strconv"

// springWriter emits a grounded elastic foundation for one support face: a SPRING1 element on
// every node in each of the three global directions, plus the *SPRING stiffness for each
// direction. SPRING1 is CalculiX's grounded spring — one node, acting along a single global DOF
// — so a node held in x, y and z is sprung to ground in every direction. The cards are
// model-level (before *STEP), so this is written from WriteSets; WriteStep adds nothing.
type springWriter struct{ c *SpringSupport }

// springDOFs are the global translational directions a grounded support spring acts in.
var springDOFs = [3]int{1, 2, 3}

func (s springWriter) WriteSets(d *deckBuf) {
	if len(s.c.Nodes) == 0 {
		return
	}
	perNode := s.c.StiffnessTotal / float64(len(s.c.Nodes))
	for axis, dof := range springDOFs {
		elset := s.elsetName(dof)
		writeSpringElements(d, elset, s.c.Nodes, s.c.FirstElem+axis*len(s.c.Nodes))
		d.line("*SPRING, ELSET=%s", elset)
		d.line("%d", dof)
		d.line("%.10g", perNode)
	}
}

func (springWriter) WriteStep(*deckBuf) {} // spring cards are model-level, written in WriteSets

// elsetName names the per-direction spring element set (e.g. "FIX_SPRING1").
func (s springWriter) elsetName(dof int) string {
	return s.c.Name + "_SPRING" + strconv.Itoa(dof)
}

// writeSpringElements emits one SPRING1 element per node, numbering them from firstElem so they
// never collide with the solid mesh's element ids.
func writeSpringElements(d *deckBuf, elset string, nodes []int, firstElem int) {
	d.line("*ELEMENT, TYPE=SPRING1, ELSET=%s", elset)
	for i, n := range nodes {
		d.line("%d, %d", firstElem+i, n)
	}
}
