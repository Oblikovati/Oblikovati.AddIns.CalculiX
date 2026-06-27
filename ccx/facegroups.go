// SPDX-License-Identifier: GPL-2.0-only

package ccx

import (
	"fmt"
	"math"
)

// FaceGroups maps each selected host face (by reference key) to the mesh node set on that
// face, used to attach loads and boundary conditions.
type FaceGroups struct {
	Nodes map[string][]int
}

// normalAlignMin is the minimum |dot| of two unit normals to consider two facets coplanar
// (about 25°). Box/prism faces meet at sharp edges, so this cleanly separates them.
const normalAlignMin = 0.9

// faceAgg accumulates a group of mesh boundary facets sharing a gmsh surface id: a
// representative centroid and normal, plus the union of their node ids.
type faceAgg struct {
	centroidSum [3]float64
	normalSum   [3]float64
	count       int
	nodes       map[int]bool
}

// buildFaceGroups binds each selected host face to a mesh node set. gmsh has already
// partitioned the mesh surface into per-face groups (BoundaryFacet.Face); each host face
// is matched to the gmsh group with the aligned normal and nearest centroid. This is
// exact for the planar/prismatic faces of the v1 slice; a curved host face that gmsh
// splits into several patches matches only its nearest patch (a documented follow-up).
func (e *Engine) buildFaceGroups(bodyIndex int, faceKeys []string, mesh *TetMesh) (*FaceGroups, error) {
	groups := groupBoundaryByFace(mesh)
	out := &FaceGroups{Nodes: make(map[string][]int, len(faceKeys))}
	for _, key := range faceKeys {
		host, err := e.pullFaceFacets(bodyIndex, key)
		if err != nil {
			return nil, err
		}
		hc, hn := surfaceCentroidNormal(host)
		match := matchFace(groups, hc, hn)
		if match == nil {
			return nil, fmt.Errorf("face %s did not match any mesh surface group", key)
		}
		out.Nodes[key] = match.nodeList()
	}
	return out, nil
}

// groupBoundaryByFace aggregates the mesh's boundary facets by their gmsh surface id.
func groupBoundaryByFace(mesh *TetMesh) map[int]*faceAgg {
	index := mesh.nodeByID()
	groups := map[int]*faceAgg{}
	for _, bf := range mesh.Surface {
		c, n := facetCentroidNormal(bf, index)
		agg := groups[bf.Face]
		if agg == nil {
			agg = &faceAgg{nodes: map[int]bool{}}
			groups[bf.Face] = agg
		}
		agg.accumulate(c, n, bf.Nodes)
	}
	return groups
}

// accumulate folds one facet's centroid, normal, and nodes into the aggregate.
func (a *faceAgg) accumulate(centroid, normal [3]float64, nodes []int) {
	for k := 0; k < 3; k++ {
		a.centroidSum[k] += centroid[k]
		a.normalSum[k] += normal[k]
	}
	a.count++
	for _, n := range nodes {
		a.nodes[n] = true
	}
}

// centroid returns the mean facet centroid of the group.
func (a *faceAgg) centroid() [3]float64 {
	inv := 1.0 / float64(a.count)
	return [3]float64{a.centroidSum[0] * inv, a.centroidSum[1] * inv, a.centroidSum[2] * inv}
}

// normal returns the (unit) mean facet normal of the group.
func (a *faceAgg) normal() [3]float64 { return normalize(a.normalSum) }

// nodeList returns the group's node ids.
func (a *faceAgg) nodeList() []int {
	ids := make([]int, 0, len(a.nodes))
	for n := range a.nodes {
		ids = append(ids, n)
	}
	return ids
}

// matchFace returns the boundary group whose normal aligns with hn and whose centroid is
// closest to hc, or nil if none aligns.
func matchFace(groups map[int]*faceAgg, hc, hn [3]float64) *faceAgg {
	var best *faceAgg
	bestDist := math.Inf(1)
	for _, agg := range groups {
		if math.Abs(dot(agg.normal(), hn)) < normalAlignMin {
			continue
		}
		if d := distance(agg.centroid(), hc); d < bestDist {
			bestDist, best = d, agg
		}
	}
	return best
}

// facetCentroidNormal returns a boundary facet's corner centroid and unit normal.
func facetCentroidNormal(bf BoundaryFacet, index map[int]Node) ([3]float64, [3]float64) {
	a := nodeXYZ(index[bf.Corners[0]])
	b := nodeXYZ(index[bf.Corners[1]])
	c := nodeXYZ(index[bf.Corners[2]])
	centroid := [3]float64{(a[0] + b[0] + c[0]) / 3, (a[1] + b[1] + c[1]) / 3, (a[2] + b[2] + c[2]) / 3}
	return centroid, triNormal(a, b, c)
}

// surfaceCentroidNormal returns the mean triangle centroid and unit mean normal of a
// host face's tessellation.
func surfaceCentroidNormal(s *SurfaceMesh) ([3]float64, [3]float64) {
	var cs, ns [3]float64
	for _, tri := range s.Tris {
		a, b, c := s.Verts[tri[0]], s.Verts[tri[1]], s.Verts[tri[2]]
		n := triNormal(a, b, c)
		for k := 0; k < 3; k++ {
			cs[k] += (a[k] + b[k] + c[k]) / 3
			ns[k] += n[k]
		}
	}
	if len(s.Tris) > 0 {
		inv := 1.0 / float64(len(s.Tris))
		cs = [3]float64{cs[0] * inv, cs[1] * inv, cs[2] * inv}
	}
	return cs, normalize(ns)
}

func nodeXYZ(n Node) [3]float64 { return [3]float64{n.X, n.Y, n.Z} }

func dot(a, b [3]float64) float64 { return a[0]*b[0] + a[1]*b[1] + a[2]*b[2] }

func distance(a, b [3]float64) float64 {
	return math.Sqrt((a[0]-b[0])*(a[0]-b[0]) + (a[1]-b[1])*(a[1]-b[1]) + (a[2]-b[2])*(a[2]-b[2]))
}

func normalize(v [3]float64) [3]float64 {
	mag := math.Sqrt(dot(v, v))
	if mag == 0 {
		return [3]float64{}
	}
	return [3]float64{v[0] / mag, v[1] / mag, v[2] / mag}
}
