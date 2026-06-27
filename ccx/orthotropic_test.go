// SPDX-License-Identifier: GPL-2.0-only

package ccx

import (
	"math"
	"strings"
	"testing"

	"oblikovati.org/api/types"
	"oblikovati.org/api/wire"
)

func TestDeckWritesOrthotropic(t *testing.T) {
	deck := writeDeckString(t, &AnalysisModel{
		Analysis: AnalysisStatic,
		Mesh:     unitTet(),
		Material: MaterialProps{Name: "WOOD", Ortho: &OrthoElastic{
			E1MPa: 11000, E2MPa: 900, E3MPa: 500, Nu12: 0.4, Nu13: 0.4, Nu23: 0.3,
			G12MPa: 700, G13MPa: 700, G23MPa: 50,
		}},
		Fixed:  []FixedConstraint{{Name: "FIX", Nodes: []int{1}, DOFLow: 1, DOFHigh: 3}},
		Forces: []ForceLoad{{Name: "L", Nodes: []int{2}, Dir: [3]float64{1, 0, 0}, TotalN: 1}},
	})
	for _, want := range []string{
		"*ELASTIC, TYPE=ENGINEERING CONSTANTS",
		"11000, 900, 500, 0.4, 0.4, 0.3, 700, 700", // E1,E2,E3,nu12,nu13,nu23,G12,G13
		"\n50\n", // G23 on its own line (the 9th constant)
	} {
		if !strings.Contains(deck, want) {
			t.Errorf("orthotropic deck missing %q\n%s", want, deck)
		}
	}
	if strings.Contains(deck, "*ELASTIC\n") {
		t.Error("an orthotropic material should not write an isotropic *ELASTIC")
	}
}

// TestOrthoFromInfoConvertsAxes: a host orthotropic material resolves to the nine engineering
// constants in MPa, and an isotropic material resolves to nil (the Young/Poisson path).
func TestOrthoFromInfoConvertsAxes(t *testing.T) {
	o := orthoFromInfo(wire.MaterialInfo{
		IsotropyClass: string(types.Orthotropic),
		Anisotropic:   types.AnisotropicElastic{E1: 11, E2: 0.9, E3: 0.5, G12: 0.7, G13: 0.7, G23: 0.05, Nu12: 0.4},
	})
	if o == nil || math.Abs(o.E1MPa-11000) > 1e-6 || math.Abs(o.G23MPa-50) > 1e-6 {
		t.Errorf("ortho conversion = %+v, want E1=11000 MPa, G23=50 MPa", o)
	}
	if orthoFromInfo(wire.MaterialInfo{IsotropyClass: string(types.Isotropic)}) != nil {
		t.Error("an isotropic material should resolve to nil ortho")
	}
}

// TestOrthotropicAxialStiffness is the orthotropic oracle: a bar aligned with material axis 1
// and pulled axially stretches according to its axis-1 modulus alone,
//
//	delta = P*L / (A*E1),
//
// independent of E2/E3 (uniaxial stress). Runs through the real ccx via the *ELASTIC,
// TYPE=ENGINEERING CONSTANTS path; using E1 (not the isotropic Young's modulus) confirms the
// orthotropic constants drive the solution.
func TestOrthotropicAxialStiffness(t *testing.T) {
	bins := requireSolver(t)
	const (
		L, side = 200.0, 10.0 // mm, bar along x (= material axis 1)
		e1      = 100000.0    // MPa, axis-1 modulus
		p       = 20000.0     // N axial (+x) on the end face
	)
	dir := t.TempDir()
	mesh := meshBeam(t, bins, L, side, dir)
	root := selectNodes(mesh, func(n Node) bool { return n.X < eps(L) })
	tip := selectNodes(mesh, func(n Node) bool { return n.X > L-eps(L) })
	if len(root) == 0 || len(tip) == 0 {
		t.Fatalf("selection failed (root=%d tip=%d)", len(root), len(tip))
	}

	model := &AnalysisModel{
		Analysis: AnalysisStatic,
		Mesh:     mesh,
		Material: MaterialProps{Name: "ORTHO", Ortho: &OrthoElastic{
			E1MPa: e1, E2MPa: 70000, E3MPa: 70000, Nu12: 0.3, Nu13: 0.3, Nu23: 0.3,
			G12MPa: 38000, G13MPa: 38000, G23MPa: 27000,
		}},
		Fixed:  []FixedConstraint{{Name: "ROOT", Nodes: root, DOFLow: 1, DOFHigh: 3}},
		Forces: []ForceLoad{{Name: "PULL", Nodes: tip, Dir: [3]float64{1, 0, 0}, TotalN: p}},
	}
	res := solveModel(t, bins, model, dir)

	got := meanUX(res, tip)
	want := p * L / (side * side * e1)
	relErr := math.Abs(got-want) / want
	t.Logf("orthotropic axial extension: FE=%.5f mm, analytic P·L/(A·E1)=%.5f mm, rel err=%.1f%%", got, want, relErr*100)
	if relErr > 0.05 {
		t.Errorf("axial extension %.5f mm differs from analytic %.5f mm by %.1f%% (>5%%)", got, want, relErr*100)
	}
}

// meanUX returns the mean x-displacement over a node set.
func meanUX(res *ResultField, nodes []int) float64 {
	sum := 0.0
	for _, id := range nodes {
		sum += res.Disp[id][0]
	}
	return sum / float64(len(nodes))
}
