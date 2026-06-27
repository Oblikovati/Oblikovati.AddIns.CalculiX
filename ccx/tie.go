// SPDX-License-Identifier: GPL-2.0-only

package ccx

import (
	"fmt"
	"math"
	"sort"
)

// TieConstraint bonds two coincident interface surfaces of separately-meshed bodies with a
// CalculiX *TIE (tied / mesh-tie contact): it glues the slave faces' nodes to the master
// faces, generating MPCs so load transfers across the otherwise non-conformal interface as
// if the bodies were one. Both surfaces are written as element-face (*SURFACE TYPE=ELEMENT)
// surfaces; the slave is listed first, the master second (the CalculiX data-line order).
type TieConstraint struct {
	Name   string
	Slave  []ElemFace
	Master []ElemFace
}

// detectTies finds bonded interfaces in a merged multi-body mesh: pairs of per-body boundary
// face groups (gmsh surface tags, offset per body in mergeTetMeshes) that are geometrically
// coincident — same location, opposing outward normals — i.e. two bodies touching along a
// shared face. Each such pair becomes one *TIE, so the bonded assembly transmits load and an
// otherwise-unconstrained body is held through the tie. A single-body mesh yields none.
func detectTies(mesh *TetMesh) []TieConstraint {
	faceIndex := faceElemIndex(mesh)
	elemBody := elementBodyIndex(mesh)
	tol := tieMatchTolerance(mesh)
	// gmsh sometimes splits one flat face into several triangular patches; merge the coplanar
	// patches of a body back into one face so an interface is matched whole, regardless of how
	// either side was patched.
	groups := mergeCoplanarGroups(tieGroups(mesh, faceIndex, elemBody), mesh, tol)

	var ties []TieConstraint
	used := make([]bool, len(groups))
	for i := range groups {
		if used[i] {
			continue
		}
		for j := i + 1; j < len(groups); j++ {
			if used[j] || groups[i].body == groups[j].body || !coincidentFaces(groups[i], groups[j], tol) {
				continue
			}
			ties = append(ties, TieConstraint{
				Name:   fmt.Sprintf("TIE%d", len(ties)),
				Slave:  resolveElemFaces(groups[i].facets, faceIndex),
				Master: resolveElemFaces(groups[j].facets, faceIndex),
			})
			used[i], used[j] = true, true
			break
		}
	}
	return ties
}

// tieGroup is one body's boundary face group reduced to what interface matching needs: its
// source body, a representative centroid and outward normal, and its corner-triples (for the
// element-face surface).
type tieGroup struct {
	body     int
	centroid [3]float64
	normal   [3]float64
	facets   [][3]int
}

// tieGroups builds the per-face boundary groups in a deterministic order (sorted by gmsh
// surface tag), each tagged with its source body.
func tieGroups(mesh *TetMesh, faceIndex map[[3]int]ElemFace, elemBody map[int]int) []tieGroup {
	byTag := groupBoundaryByFace(mesh)
	tags := make([]int, 0, len(byTag))
	for tag := range byTag {
		tags = append(tags, tag)
	}
	sort.Ints(tags)
	groups := make([]tieGroup, 0, len(tags))
	for _, tag := range tags {
		agg := byTag[tag]
		groups = append(groups, tieGroup{
			body:     groupBody(agg, faceIndex, elemBody),
			centroid: agg.centroid(),
			normal:   agg.normal(),
			facets:   agg.facets,
		})
	}
	return groups
}

// mergeCoplanarGroups fuses a body's boundary face groups that lie on the same plane (same
// outward normal, same offset along it) into one group, recomputing the centroid from the
// combined facets. This makes interface matching independent of how gmsh patched a flat face.
func mergeCoplanarGroups(groups []tieGroup, mesh *TetMesh, tol float64) []tieGroup {
	index := mesh.nodeByID()
	used := make([]bool, len(groups))
	var merged []tieGroup
	for i := range groups {
		if used[i] {
			continue
		}
		facets := append([][3]int{}, groups[i].facets...)
		for j := i + 1; j < len(groups); j++ {
			if !used[j] && groups[j].body == groups[i].body && samePlane(groups[i], groups[j], tol) {
				facets = append(facets, groups[j].facets...)
				used[j] = true
			}
		}
		merged = append(merged, tieGroup{
			body:     groups[i].body,
			centroid: facetsCentroid(facets, index),
			normal:   groups[i].normal,
			facets:   facets,
		})
	}
	return merged
}

// samePlane reports whether two same-body face groups lie on one plane: their normals point
// the same way and their signed offsets along that normal agree.
func samePlane(a, b tieGroup, tol float64) bool {
	if dot(a.normal, b.normal) < 0.99 {
		return false
	}
	return math.Abs(dot(a.normal, a.centroid)-dot(b.normal, b.centroid)) < tol
}

// facetsCentroid returns the mean corner-centroid of a set of facets.
func facetsCentroid(facets [][3]int, index map[int]Node) [3]float64 {
	var sum [3]float64
	for _, tri := range facets {
		for _, id := range tri {
			n := index[id]
			sum[0] += n.X / 3
			sum[1] += n.Y / 3
			sum[2] += n.Z / 3
		}
	}
	if len(facets) == 0 {
		return sum
	}
	inv := 1.0 / float64(len(facets))
	return [3]float64{sum[0] * inv, sum[1] * inv, sum[2] * inv}
}

// groupBody resolves which body a face group belongs to via the parent element of its first
// resolvable facet.
func groupBody(agg *faceAgg, faceIndex map[[3]int]ElemFace, elemBody map[int]int) int {
	for _, tri := range agg.facets {
		if ef, ok := faceIndex[sortedTriple(tri[0], tri[1], tri[2])]; ok {
			return elemBody[ef.Elem]
		}
	}
	return -1
}

// elementBodyIndex maps each element id to its source body index.
func elementBodyIndex(mesh *TetMesh) map[int]int {
	out := make(map[int]int, len(mesh.Elements))
	for _, el := range mesh.Elements {
		out[el.ID] = el.Body
	}
	return out
}

// coincidentFaces reports whether two face groups form a bonded interface: their outward
// normals oppose (the two sides of a shared face) and their centroids sit at the same place.
// Anti-parallel outer faces on opposite ends of the assembly fail the centroid test (they are
// a body-length apart), so only genuinely touching faces match.
func coincidentFaces(a, b tieGroup, tol float64) bool {
	if dot(a.normal, b.normal) > -0.9 {
		return false
	}
	return distance(a.centroid, b.centroid) < tol
}

// tieMatchTolerance is the centroid-coincidence tolerance for interface detection: a small
// fraction of the mesh diagonal — far below any body's extent, so two touching faces (whose
// centroids coincide) match while two distinct faces do not.
func tieMatchTolerance(mesh *TetMesh) float64 {
	lo, hi := meshBounds(mesh)
	diag := math.Sqrt((hi[0]-lo[0])*(hi[0]-lo[0]) + (hi[1]-lo[1])*(hi[1]-lo[1]) + (hi[2]-lo[2])*(hi[2]-lo[2]))
	return diag * 0.02
}

// writeTies emits the *SURFACE pair and *TIE card for each bonded interface, before the
// *STEP (a *TIE is model-level). POSITION TOLERANCE is left to the CalculiX default (auto),
// which captures the coincident slave nodes onto the master faces.
func writeTies(d *deckBuf, ties []TieConstraint) {
	for _, t := range ties {
		writeFaceSurface(d, t.Name+"_S", t.Slave)
		writeFaceSurface(d, t.Name+"_M", t.Master)
		d.line("*TIE, NAME=%s", t.Name)
		d.line("%s_S, %s_M", t.Name, t.Name)
	}
}

// writeFaceSurface writes a TYPE=ELEMENT surface as one "element, Sn" line per element-face.
func writeFaceSurface(d *deckBuf, name string, faces []ElemFace) {
	d.line("*SURFACE, NAME=%s, TYPE=ELEMENT", name)
	for _, f := range faces {
		d.line("%d, S%d", f.Elem, f.Face)
	}
}
