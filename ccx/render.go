// SPDX-License-Identifier: GPL-2.0-only

package ccx

import (
	"fmt"
	"os"
)

// resultClientID is the client-graphics group the stress/temperature result is pushed under.
const resultClientID = "ccx.result"

// scalarFieldMapperName is the registered color mapper the DOF-11 field (temperature /
// electric potential) flood plot uses.
const scalarFieldMapperName = "ccx.scalarfield"

// renderScalarField paints a steady-state nodal DOF-11 field (temperature for heat
// transfer, electric potential for the electrostatic analogy) over the surface as a flood
// plot spanning the field's actual range.
func (e *Engine) renderScalarField(mesh *TetMesh, values map[int]float64) error {
	coords, indices, scalars := surfaceRenderData(mesh, values)
	lo, hi := minMaxField(values)
	mapper := rampMapper(lo, hi)
	if err := e.api.Graphics().RegisterColorMapper(scalarFieldMapperName, mapper); err != nil {
		return err
	}
	_, err := e.api.Graphics().AddFloodPlot(resultClientID, coords, indices, scalars, mapper, 1.0)
	return err
}

// renderModeShape paints the first mode's displacement-magnitude field over the surface —
// the standard mode-shape visualization for a modal or buckling result.
func (e *Engine) renderModeShape(frdPath string, mesh *TetMesh) error {
	f, err := os.Open(frdPath)
	if err != nil {
		return fmt.Errorf("open frd: %w", err)
	}
	defer f.Close()
	disp, err := parseFirstModeDisp(f)
	if err != nil {
		return err
	}
	field := dispMagnitude(&ResultField{Disp: disp})
	coords, indices, scalars := surfaceRenderData(mesh, field)
	mapper := stressMapper(peak(field))
	if err := e.api.Graphics().RegisterColorMapper(stressMapperName, mapper); err != nil {
		return err
	}
	_, err = e.api.Graphics().AddFloodPlot(resultClientID, coords, indices, scalars, mapper, 1.0)
	return err
}

// renderResult paints the selected scalar field (von Mises / displacement / principal
// stress) over the mesh surface as a client-graphics flood plot spanning the field's actual
// range. Mesh coordinates are in mm; the viewport works in host model units, so coordinates
// are scaled back by 1/modelUnitMM. Returns the field's peak value, label, and unit for the
// status report.
func (e *Engine) renderResult(mesh *TetMesh, res *ResultField, kind ResultFieldKind) (float64, string, string, error) {
	field, label, unit := computeResultField(res, kind)
	coords, indices, scalars := surfaceRenderData(mesh, field)
	lo, hi := minMaxField(field)
	mapper := rampMapper(lo, hi)
	if err := e.api.Graphics().RegisterColorMapper(stressMapperName, mapper); err != nil {
		return 0, "", "", err
	}
	if _, err := e.api.Graphics().AddFloodPlot(resultClientID, coords, indices, scalars, mapper, 1.0); err != nil {
		return 0, "", "", err
	}
	return peak(field), label, unit, nil
}

// surfaceRenderData flattens the mesh surface into the (coords, triangle-indices,
// per-vertex scalar) arrays the heatmap expects. Only the corner nodes of the boundary
// facets are emitted (a linear triangle skin), each carrying its nodal von Mises value.
// Coordinates are converted mm -> host model units.
func surfaceRenderData(mesh *TetMesh, vm map[int]float64) ([]float64, []int, []float64) {
	index := mesh.nodeByID()
	slot := make(map[int]int) // node id -> 0-based render vertex
	var coords []float64
	var scalars []float64
	var indices []int
	for _, bf := range mesh.Surface {
		for _, nid := range bf.Corners {
			if _, ok := slot[nid]; !ok {
				slot[nid] = len(coords) / 3
				n := index[nid]
				coords = append(coords, n.X/modelUnitMM, n.Y/modelUnitMM, n.Z/modelUnitMM)
				scalars = append(scalars, vm[nid])
			}
			indices = append(indices, slot[nid])
		}
	}
	return coords, indices, scalars
}
