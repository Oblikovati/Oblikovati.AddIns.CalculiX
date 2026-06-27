// SPDX-License-Identifier: GPL-2.0-only

package ccx

import (
	"strconv"
	"strings"
)

// allElementsSet is the element set every solid element belongs to.
const allElementsSet = "Eall"

// writeMesh emits the *NODE block and one *ELEMENT block per material section, each carrying
// its own ELSET so a multi-body part lands its bodies in separate element sets (the handle a
// *SOLID SECTION binds a material to). A single-section study writes one ELSET=Eall block,
// identical to a uniform-material deck. Elements are already in CalculiX node ordering
// (mshparse re-numbers C3D10 tets), so the connectivity is written verbatim.
func writeMesh(d *deckBuf, mesh *TetMesh, sections []MaterialSection) {
	d.line("*NODE, NSET=Nall")
	for _, n := range mesh.Nodes {
		d.line("%d, %.10g, %.10g, %.10g", n.ID, n.X, n.Y, n.Z)
	}
	byID := elementByID(mesh)
	elemType := mesh.ElementType()
	for _, sec := range sections {
		d.line("*ELEMENT, TYPE=%s, ELSET=%s", elemType, sec.ElsetName)
		for _, id := range sec.ElementIDs {
			if e, ok := byID[id]; ok {
				d.line("%d, %s", e.ID, joinInts(e.Nodes))
			}
		}
	}
	writeAllElementsSet(d, sections)
}

// writeAllElementsSet defines the Eall set spanning every element, so a body load (gravity,
// centrifugal) can address the whole model. When the per-body sections already use a single
// Eall set (the uniform-material case), Eall is defined by the *ELEMENT block and nothing more
// is needed; a multi-body mesh unites its per-body sets (Eb0, Eb1, …) into Eall here.
func writeAllElementsSet(d *deckBuf, sections []MaterialSection) {
	for _, sec := range sections {
		if sec.ElsetName == allElementsSet {
			return
		}
	}
	d.line("*ELSET, ELSET=%s", allElementsSet)
	for _, sec := range sections {
		d.line("%s", sec.ElsetName)
	}
}

// elementByID indexes a mesh's elements by id, so a section's element-id list resolves to
// connectivity without rescanning the element slice per section.
func elementByID(mesh *TetMesh) map[int]TetElement {
	out := make(map[int]TetElement, len(mesh.Elements))
	for _, e := range mesh.Elements {
		out[e.ID] = e
	}
	return out
}

// joinInts formats a node-id list as the comma-separated connectivity CalculiX expects.
func joinInts(ids []int) string {
	parts := make([]string, len(ids))
	for i, v := range ids {
		parts[i] = strconv.Itoa(v)
	}
	return strings.Join(parts, ", ")
}
