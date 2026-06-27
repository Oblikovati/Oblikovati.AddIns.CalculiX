// SPDX-License-Identifier: GPL-2.0-only

package ccx

import (
	"fmt"

	"oblikovati.org/api/wire"
)

// facetTolerance is the chordal tessellation tolerance (host model units) requested for
// the surface pull. It also sets the scale at which the volume mesher approximates curved
// faces; a panel knob can override it later.
const facetTolerance = 0.05

// pullSurface fetches the body's triangulated surface from the host and welds it into a
// watertight indexed mesh in millimetres (host coordinates are in model units = cm). The
// welded surface is the input to the volume mesher.
func (e *Engine) pullSurface(bodyIndex int) (*SurfaceMesh, error) {
	facets, err := e.api.Body().CalculateFacets(wire.CalculateFacetsArgs{
		BodyIndex: bodyIndex,
		Tolerance: facetTolerance,
	})
	if err != nil {
		return nil, fmt.Errorf("calculate facets for body %d: %w", bodyIndex, err)
	}
	coords := scaleCoords(facets.VertexCoordinates, modelUnitMM)
	return weldSurface(coords, facets.VertexIndices)
}

// pullFaceFacets fetches the triangulation of a single B-rep face (by reference key) in
// millimetres, for matching against the volume mesh's boundary facets (face-group binding).
func (e *Engine) pullFaceFacets(bodyIndex int, faceKey string) (*SurfaceMesh, error) {
	facets, err := e.api.Body().FaceCalculateFacets(wire.FaceFacetsArgs{
		BodyIndex: bodyIndex,
		FaceKey:   faceKey,
		Tolerance: facetTolerance,
	})
	if err != nil {
		return nil, fmt.Errorf("calculate facets for face %s: %w", faceKey, err)
	}
	coords := scaleCoords(facets.VertexCoordinates, modelUnitMM)
	return weldSurface(coords, facets.VertexIndices)
}

// scaleCoords returns a copy of a flat coordinate slice multiplied by factor.
func scaleCoords(coords []float64, factor float64) []float64 {
	out := make([]float64, len(coords))
	for i, c := range coords {
		out[i] = c * factor
	}
	return out
}
