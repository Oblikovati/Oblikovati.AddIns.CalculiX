// SPDX-License-Identifier: GPL-2.0-only

package ccx

import (
	"math"

	"oblikovati.org/api/types"
	"oblikovati.org/api/wire"
)

// Client-graphics groups for the constraint visual aids (separate from the result so they
// can be toggled or replaced independently).
const (
	supportsClientID  = "ccx.supports"
	loadsClientID     = "ccx.loads"
	potentialClientID = "ccx.potential"
)

var (
	// supportColor is the cyan of the fixed-support cubes; loadColor the red of the load
	// arrows — the conventional FEA support/load colours.
	supportColor = []float32{0.20, 0.70, 1.0, 1.0}
	loadColor    = []float32{1.0, 0.25, 0.12, 1.0}
	// highPotentialColor is the red of the applied-voltage face; the ground face reuses the
	// cyan supportColor — a clear high/low pairing for an electrostatic study.
	highPotentialColor = []float32{1.0, 0.25, 0.12, 1.0}
)

// maxConstraintGlyphs caps the glyphs drawn per face so a fine mesh does not bury the
// model in symbols; the node set is sampled evenly to this many.
const maxConstraintGlyphs = 24

// renderConstraints draws solid 3D fixed-support cubes and load arrows over the model,
// mirroring the support/load symbols a dedicated FEA setup shows. The first selected face
// is the support. A surface load (force/pressure) draws arrows on the loaded faces; a
// gravity body load draws arrows spread over the whole body. Coordinates are converted
// from the mesh's millimetres back to host model units.
func (e *Engine) renderConstraints(mesh *TetMesh, groups *FaceGroups, faces []string, model *AnalysisModel) error {
	index := mesh.nodeByID()
	length := glyphScale(mesh)
	if model.Analysis == AnalysisElectromagnetic {
		return e.drawPotentialFaces(groups, faces, index, length)
	}
	if err := e.drawSupports(groups.Nodes[faces[0]], index, length); err != nil {
		return err
	}
	if model.Gravity != nil {
		return e.drawBodyLoad(mesh, model.Gravity.Dir, index, length)
	}
	return e.drawLoads(groups, faces[1:], loadDirection(model), index, length)
}

// drawPotentialFaces marks an electrostatic study's boundary faces: red cubes on the
// applied-voltage face, cyan cubes on the ground face(s) — a high/low pairing in place of
// the support/arrow symbols, which carry no meaning for a potential field.
func (e *Engine) drawPotentialFaces(groups *FaceGroups, faces []string, index map[int]Node, length float64) error {
	if err := e.drawCubes(potentialClientID, highPotentialColor, groups.Nodes[faces[0]], index, length); err != nil {
		return err
	}
	var ground []int
	for _, key := range faces[1:] {
		ground = append(ground, groups.Nodes[key]...)
	}
	return e.drawCubes(supportsClientID, supportColor, ground, index, length)
}

// drawBodyLoad paints arrows spread over the body's surface to indicate a gravity body
// force (which has no single loaded face).
func (e *Engine) drawBodyLoad(mesh *TetMesh, dir [3]float64, index map[int]Node, length float64) error {
	g := &glyphMesh{}
	for _, nid := range sampleNodes(surfaceNodeIDs(mesh), maxConstraintGlyphs) {
		g.arrow(modelPoint(index[nid]), dir, length)
	}
	return e.pushGlyphs(loadsClientID, g, loadColor)
}

// surfaceNodeIDs returns the unique corner-node ids on the mesh surface.
func surfaceNodeIDs(mesh *TetMesh) []int {
	seen := map[int]bool{}
	var ids []int
	for _, bf := range mesh.Surface {
		for _, n := range bf.Corners {
			if !seen[n] {
				seen[n] = true
				ids = append(ids, n)
			}
		}
	}
	return ids
}

// drawSupports paints a solid cyan cube at each fixed-face node.
func (e *Engine) drawSupports(nodes []int, index map[int]Node, length float64) error {
	return e.drawCubes(supportsClientID, supportColor, nodes, index, length)
}

// drawCubes paints a solid cube of the given colour at each node, under the given client
// group — the shared glyph for any "this face is pinned to a value" boundary condition.
func (e *Engine) drawCubes(clientID string, color []float32, nodes []int, index map[int]Node, length float64) error {
	g := &glyphMesh{}
	half := length * 0.16
	for _, nid := range sampleNodes(nodes, maxConstraintGlyphs) {
		g.cube(modelPoint(index[nid]), half)
	}
	return e.pushGlyphs(clientID, g, color)
}

// drawLoads paints a solid arrow at each loaded-face node, pointing along the load.
func (e *Engine) drawLoads(groups *FaceGroups, faces []string, loadDir [3]float64, index map[int]Node, length float64) error {
	g := &glyphMesh{}
	for _, key := range faces {
		for _, nid := range sampleNodes(groups.Nodes[key], maxConstraintGlyphs) {
			g.arrow(modelPoint(index[nid]), loadDir, length)
		}
	}
	return e.pushGlyphs(loadsClientID, g, loadColor)
}

// pushGlyphs pushes a glyph mesh as a lit, OnTop client-graphics group so the aids render
// above the depth-tested geometry and the result flood-plot overlay.
func (e *Engine) pushGlyphs(clientID string, g *glyphMesh, color []float32) error {
	if len(g.idx) == 0 {
		return nil
	}
	_, err := e.api.Graphics().Set(onTopGroup(clientID, wire.GraphicsPrimitive{
		Kind:        string(types.GraphicsTriangles),
		Coordinates: g.coords,
		Indices:     g.idx,
		Normals:     g.normals,
		Color:       color,
	}))
	return err
}

// onTopGroup wraps one primitive as an OnTop client-graphics group in the persistent lane,
// so the support/load aids render above the geometry and the result overlay.
func onTopGroup(clientID string, p wire.GraphicsPrimitive) wire.SetClientGraphicsArgs {
	p.OnTop = true
	p.DepthPriority = 10
	return wire.SetClientGraphicsArgs{
		ClientId: clientID,
		Lane:     string(types.GraphicsLanePersistent),
		Nodes:    []wire.GraphicsNode{{Primitives: []wire.GraphicsPrimitive{p}}},
	}
}

// anyPerpendicular returns a unit vector orthogonal to d.
func anyPerpendicular(d [3]float64) [3]float64 {
	axis := [3]float64{1, 0, 0}
	if math.Abs(d[0]) > 0.9 {
		axis = [3]float64{0, 1, 0}
	}
	return normalize(cross(d, axis))
}

// modelPoint converts a mesh node (mm) to host model units.
func modelPoint(n Node) [3]float64 {
	return [3]float64{n.X / modelUnitMM, n.Y / modelUnitMM, n.Z / modelUnitMM}
}

// glyphScale sizes the glyphs relative to the model bounding box (model units).
func glyphScale(mesh *TetMesh) float64 {
	lo, hi := meshBounds(mesh)
	diag := math.Sqrt((hi[0]-lo[0])*(hi[0]-lo[0]) + (hi[1]-lo[1])*(hi[1]-lo[1]) + (hi[2]-lo[2])*(hi[2]-lo[2]))
	return (diag / modelUnitMM) * 0.14
}

// meshBounds returns the mesh's coordinate bounding box (mm).
func meshBounds(mesh *TetMesh) ([3]float64, [3]float64) {
	lo := [3]float64{math.Inf(1), math.Inf(1), math.Inf(1)}
	hi := [3]float64{math.Inf(-1), math.Inf(-1), math.Inf(-1)}
	for _, n := range mesh.Nodes {
		for k, c := range [3]float64{n.X, n.Y, n.Z} {
			lo[k] = math.Min(lo[k], c)
			hi[k] = math.Max(hi[k], c)
		}
	}
	return lo, hi
}

// sampleNodes returns up to limit node ids spread evenly across the set.
func sampleNodes(nodes []int, limit int) []int {
	if len(nodes) <= limit {
		return nodes
	}
	step := float64(len(nodes)) / float64(limit)
	out := make([]int, 0, limit)
	for i := 0; i < limit; i++ {
		out = append(out, nodes[int(float64(i)*step)])
	}
	return out
}
