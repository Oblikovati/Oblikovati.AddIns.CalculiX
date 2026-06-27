// SPDX-License-Identifier: GPL-2.0-only

package ccx

import (
	"strconv"
	"strings"
)

// allElementsSet is the element set every solid element belongs to.
const allElementsSet = "Eall"

// writeMesh emits the *NODE and *ELEMENT blocks. Elements are already in CalculiX node
// ordering (mshparse re-numbers C3D10 tets), so the connectivity is written verbatim.
func writeMesh(d *deckBuf, mesh *TetMesh) {
	d.line("*NODE, NSET=Nall")
	for _, n := range mesh.Nodes {
		d.line("%d, %.10g, %.10g, %.10g", n.ID, n.X, n.Y, n.Z)
	}
	d.line("*ELEMENT, TYPE=%s, ELSET=%s", mesh.ElementType(), allElementsSet)
	for _, e := range mesh.Elements {
		d.line("%d, %s", e.ID, joinInts(e.Nodes))
	}
}

// joinInts formats a node-id list as the comma-separated connectivity CalculiX expects.
func joinInts(ids []int) string {
	parts := make([]string, len(ids))
	for i, v := range ids {
		parts[i] = strconv.Itoa(v)
	}
	return strings.Join(parts, ", ")
}
