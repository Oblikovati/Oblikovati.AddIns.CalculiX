// SPDX-License-Identifier: GPL-2.0-only

package ccx

import (
	"math"
	"strings"
	"testing"

	"oblikovati.org/api/types"
	"oblikovati.org/api/wire"
)

func TestSanitizeMatName(t *testing.T) {
	cases := map[string]string{
		"Steel":                  "Steel",
		"Shop Steel":             "Shop_Steel",
		"Al-6061 (T6)":           "Al_6061__T6_",
		"":                       "MATERIAL",
		strings.Repeat("x", 100): strings.Repeat("x", matNameMaxLen),
	}
	for in, want := range cases {
		if got := sanitizeMatName(in); got != want {
			t.Errorf("sanitizeMatName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestMaterialPropsFromInfo(t *testing.T) {
	info := wire.MaterialInfo{
		ID:          "steel",
		DisplayName: "Mild Steel",
		Density:     7.85, // g/cm^3
		Mechanical:  types.Mechanical{YoungsModulus: 210, PoissonsRatio: 0.3},
		Thermal:     types.Thermal{Conductivity: 50, ExpansionCoeff: 1.2e-5},
		Electrical:  types.Electrical{Resistivity: 1.43e-7},
	}
	m := materialPropsFromInfo(info)
	if m.Name != "Mild_Steel" {
		t.Errorf("name = %q, want Mild_Steel", m.Name)
	}
	if math.Abs(m.YoungMPa-210000) > 1e-6 {
		t.Errorf("YoungMPa = %v, want 210000 (GPa→MPa)", m.YoungMPa)
	}
	if math.Abs(m.DensityTonneMM3-7.85e-9) > 1e-18 {
		t.Errorf("density = %v, want 7.85e-9 t/mm^3", m.DensityTonneMM3)
	}
	if m.Conductivity != 50 || m.ExpansionPerK != 1.2e-5 {
		t.Errorf("thermal props = (%v, %v), want (50, 1.2e-5)", m.Conductivity, m.ExpansionPerK)
	}
	wantSigma := 1 / 1.43e-7
	if math.Abs(m.ElectricalSigma-wantSigma) > 1 {
		t.Errorf("electrical sigma = %v, want %v (1/resistivity)", m.ElectricalSigma, wantSigma)
	}
	// An unset (zero) resistivity must yield zero conductivity, not a divide-by-zero.
	if got := materialPropsFromInfo(wire.MaterialInfo{ID: "x"}).ElectricalSigma; got != 0 {
		t.Errorf("sigma for zero resistivity = %v, want 0", got)
	}
}

// TestMergeTetMeshesTagsBodies: merging two single-tet meshes offsets the second body's node
// and element ids, tags each element with its source body, and offsets the gmsh surface tag
// so the two bodies' face groups stay distinct.
func TestMergeTetMeshesTagsBodies(t *testing.T) {
	a := &TetMesh{
		Nodes:    []Node{{ID: 1}, {ID: 2}, {ID: 3}, {ID: 4}},
		Elements: []TetElement{{ID: 1, Nodes: []int{1, 2, 3, 4}}},
		Surface:  []BoundaryFacet{{Nodes: []int{1, 2, 3}, Corners: [3]int{1, 2, 3}, Face: 1}},
	}
	b := &TetMesh{
		Nodes:    []Node{{ID: 1}, {ID: 2}, {ID: 3}, {ID: 4}},
		Elements: []TetElement{{ID: 1, Nodes: []int{1, 2, 3, 4}}},
		Surface:  []BoundaryFacet{{Nodes: []int{2, 3, 4}, Corners: [3]int{2, 3, 4}, Face: 1}},
	}
	m := mergeTetMeshes([]*TetMesh{a, b})

	if len(m.Nodes) != 8 || len(m.Elements) != 2 {
		t.Fatalf("merged sizes = %d nodes, %d elems; want 8, 2", len(m.Nodes), len(m.Elements))
	}
	if m.Elements[0].Body != 0 || m.Elements[1].Body != 1 {
		t.Errorf("body tags = (%d, %d), want (0, 1)", m.Elements[0].Body, m.Elements[1].Body)
	}
	if m.Elements[1].ID != 2 || m.Elements[1].Nodes[0] != 5 {
		t.Errorf("body-1 element id/nodes not offset: id=%d nodes=%v", m.Elements[1].ID, m.Elements[1].Nodes)
	}
	if m.Surface[1].Face == m.Surface[0].Face {
		t.Errorf("surface tags collided across bodies: both %d", m.Surface[0].Face)
	}
	if m.Surface[1].Corners[0] != 6 {
		t.Errorf("body-1 facet corner not offset: %d, want 6", m.Surface[1].Corners[0])
	}
}

// TestDeckWritesPerBodyMaterials: a two-body model writes one *ELEMENT/ELSET, *MATERIAL and
// *SOLID SECTION per body; two bodies sharing a material collapse to one *MATERIAL.
func TestDeckWritesPerBodyMaterials(t *testing.T) {
	mesh := &TetMesh{
		Nodes: []Node{{ID: 1}, {ID: 2}, {ID: 3}, {ID: 4}, {ID: 5}, {ID: 6}, {ID: 7}, {ID: 8}},
		Elements: []TetElement{
			{ID: 1, Nodes: []int{1, 2, 3, 4}, Body: 0},
			{ID: 2, Nodes: []int{5, 6, 7, 8}, Body: 1},
		},
	}
	steel := MaterialProps{Name: "Steel", YoungMPa: 210000, Poisson: 0.3}
	alu := MaterialProps{Name: "Aluminium", YoungMPa: 69000, Poisson: 0.33}
	deck := writeDeckString(t, &AnalysisModel{
		Analysis: AnalysisStatic,
		Mesh:     mesh,
		Material: steel,
		Sections: []MaterialSection{
			{ElsetName: "Eb0", Material: steel, ElementIDs: []int{1}},
			{ElsetName: "Eb1", Material: alu, ElementIDs: []int{2}},
		},
	})
	for _, want := range []string{
		"*ELEMENT, TYPE=C3D4, ELSET=Eb0",
		"*ELEMENT, TYPE=C3D4, ELSET=Eb1",
		"*MATERIAL, NAME=Steel",
		"*MATERIAL, NAME=Aluminium",
		"*SOLID SECTION, ELSET=Eb0, MATERIAL=Steel",
		"*SOLID SECTION, ELSET=Eb1, MATERIAL=Aluminium",
	} {
		if !strings.Contains(deck, want) {
			t.Errorf("deck missing %q\n%s", want, deck)
		}
	}

	// Two bodies, one shared material → a single *MATERIAL, two *SOLID SECTIONs.
	shared := writeDeckString(t, &AnalysisModel{
		Analysis: AnalysisStatic,
		Mesh:     mesh,
		Material: steel,
		Sections: []MaterialSection{
			{ElsetName: "Eb0", Material: steel, ElementIDs: []int{1}},
			{ElsetName: "Eb1", Material: steel, ElementIDs: []int{2}},
		},
	})
	if n := strings.Count(shared, "*MATERIAL, NAME=Steel"); n != 1 {
		t.Errorf("shared material written %d times, want 1 (deduped)\n%s", n, shared)
	}
	if n := strings.Count(shared, "*SOLID SECTION"); n != 2 {
		t.Errorf("solid sections = %d, want 2", n)
	}
}

// TestBimaterialBarSeriesStiffness is the multi-material analytic oracle: an axially loaded
// bar split at mid-length into a stiff half (E1) and a soft half (E2) behaves as two springs
// in series, so the tip extension is
//
//	delta = P*(L/2)/(A*E1) + P*(L/2)/(A*E2)
//
// This rides the real solver through a per-material-ELSET deck on one conformal mesh (the two
// material regions share interface nodes), validating that the materials actually drive the
// solution — not merely that the deck parses.
func TestBimaterialBarSeriesStiffness(t *testing.T) {
	bins := requireSolver(t)
	const (
		h, L = 10.0, 100.0 // mm, bar along z
		e1   = 210000.0    // MPa, stiff half (z < L/2)
		e2   = 70000.0     // MPa, soft half (z >= L/2)
		nu   = 0.0         // no lateral coupling, so the 1-D series formula is exact
		p    = 1000.0      // N axial load (+z) on the far face
	)
	dir := t.TempDir()
	mesh := meshBox(t, bins, h, h, L, dir)
	root := selectNodes(mesh, func(n Node) bool { return n.Z < eps(L) })
	tip := selectNodes(mesh, func(n Node) bool { return n.Z > L-eps(L) })
	if len(root) == 0 || len(tip) == 0 {
		t.Fatalf("selection failed (root=%d tip=%d)", len(root), len(tip))
	}

	stiff := MaterialProps{Name: "Stiff", YoungMPa: e1, Poisson: nu}
	soft := MaterialProps{Name: "Soft", YoungMPa: e2, Poisson: nu}
	model := &AnalysisModel{
		Analysis: AnalysisStatic,
		Mesh:     mesh,
		Material: stiff,
		Sections: sectionsByMidplane(mesh, L/2, stiff, soft),
		Fixed:    []FixedConstraint{{Name: "ROOT", Nodes: root, DOFLow: 1, DOFHigh: 3}},
		Forces:   []ForceLoad{{Name: "TIP", Nodes: tip, Dir: [3]float64{0, 0, 1}, TotalN: p}},
	}
	res := solveModel(t, bins, model, dir)

	got := meanUZ(res, tip)
	area := h * h
	want := p*(L/2)/(area*e1) + p*(L/2)/(area*e2)
	relErr := math.Abs(got-want) / want
	t.Logf("bimaterial tip extension: FE=%.5f mm, series-spring=%.5f mm, rel err=%.1f%%", got, want, relErr*100)
	if relErr > 0.05 {
		t.Errorf("tip extension %.5f mm differs from analytic %.5f mm by %.1f%% (>5%%)", got, want, relErr*100)
	}
}

// sectionsByMidplane partitions a mesh's elements into a low-z section (matLo) and a high-z
// section (matHi) at the given z, by element centroid — building a conformal two-material
// model on a single mesh.
func sectionsByMidplane(mesh *TetMesh, z float64, matLo, matHi MaterialProps) []MaterialSection {
	index := mesh.nodeByID()
	var lo, hi []int
	for _, e := range mesh.Elements {
		if elementCentroidZ(e, index) < z {
			lo = append(lo, e.ID)
		} else {
			hi = append(hi, e.ID)
		}
	}
	return []MaterialSection{
		{ElsetName: "Eb0", Material: matLo, ElementIDs: lo},
		{ElsetName: "Eb1", Material: matHi, ElementIDs: hi},
	}
}

// elementCentroidZ returns the mean z of an element's corner nodes (the first four ids).
func elementCentroidZ(e TetElement, index map[int]Node) float64 {
	sum := 0.0
	for _, id := range e.Nodes[:4] {
		sum += index[id].Z
	}
	return sum / 4
}

// meanUZ returns the mean z-displacement over a node set.
func meanUZ(res *ResultField, nodes []int) float64 {
	sum := 0.0
	for _, id := range nodes {
		sum += res.Disp[id][2]
	}
	return sum / float64(len(nodes))
}
