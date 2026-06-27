// SPDX-License-Identifier: GPL-2.0-only

package ccx

// hydrostaticPressures returns the depth-varying fluid pressure on each element-face: p = γ·(z_s
// − z_c) below the free surface z_s, clamped to zero above it, where z_c is the face's centroid
// height and γ the pressure gradient (ρg). This is the per-face pressure a hydrostatic load
// writes, so a submerged wall feels more pressure the deeper its face sits.
func hydrostaticPressures(mesh *TetMesh, faces []ElemFace, gradientMPaMM, surfaceZ float64) []float64 {
	byID := elementByID(mesh)
	node := mesh.nodeByID()
	out := make([]float64, len(faces))
	for i, ef := range faces {
		p := gradientMPaMM * (surfaceZ - elemFaceCentroidZ(byID, node, ef))
		if p < 0 {
			p = 0 // above the free surface there is no fluid pressing
		}
		out[i] = p
	}
	return out
}

// elemFaceCentroidZ returns the mean height of an element-face's three corner nodes, the depth
// reference for its hydrostatic pressure.
func elemFaceCentroidZ(byID map[int]TetElement, node map[int]Node, ef ElemFace) float64 {
	el, ok := byID[ef.Elem]
	if !ok || ef.Face < 1 || ef.Face > len(tetFaceCorners) || len(el.Nodes) < 4 {
		return 0
	}
	var z float64
	for _, c := range tetFaceCorners[ef.Face-1] {
		z += node[el.Nodes[c]].Z
	}
	return z / 3
}
