// SPDX-License-Identifier: GPL-2.0-only

package ccx

import (
	"math"
	"strings"
	"testing"
)

func TestWriteContactsDeck(t *testing.T) {
	mesh := &TetMesh{
		Nodes:    []Node{{ID: 1}, {ID: 2}, {ID: 3}, {ID: 4}},
		Elements: []TetElement{{ID: 1, Nodes: []int{1, 2, 3, 4}}},
	}
	deck := writeDeckString(t, &AnalysisModel{
		Analysis: AnalysisStatic,
		Mesh:     mesh,
		Material: MaterialProps{Name: "STEEL", YoungMPa: 210000, Poisson: 0.3},
		Contacts: []ContactPair{{
			Name:       "CONTACT0",
			Slave:      []ElemFace{{Elem: 7, Face: 1}},
			Master:     []ElemFace{{Elem: 9, Face: 4}},
			Stiffness:  1e7,
			FrictionMu: 0.3,
		}},
	})
	for _, want := range []string{
		"*SURFACE, NAME=CONTACT0_S, TYPE=ELEMENT",
		"7, S1",
		"*SURFACE, NAME=CONTACT0_M, TYPE=ELEMENT",
		"9, S4",
		"*SURFACE INTERACTION, NAME=CONTACT0_SI",
		"*SURFACE BEHAVIOR, PRESSURE-OVERCLOSURE=LINEAR",
		"10000000",
		"*FRICTION",
		"0.3, 10000000",
		"*CONTACT PAIR, INTERACTION=CONTACT0_SI, TYPE=SURFACE TO SURFACE",
		"CONTACT0_S, CONTACT0_M",
	} {
		if !strings.Contains(deck, want) {
			t.Errorf("contact deck missing %q\n%s", want, deck)
		}
	}
	// The contact cards are model-level — written before the *STEP.
	if strings.Index(deck, "*CONTACT PAIR") > strings.Index(deck, "*STEP") {
		t.Error("*CONTACT PAIR must be written before *STEP")
	}
	// Contact is nonlinear, so the *STATIC step must carry a time-increment data line.
	if !strings.Contains(deck, "*STATIC\n0.1, 1.0") {
		t.Errorf("contact study must use an incremented *STATIC step\n%s", deck)
	}
}

// TestFrictionlessContactOmitsFriction verifies a μ=0 pair writes the penalty behaviour but no
// *FRICTION card (frictionless sliding).
func TestFrictionlessContactOmitsFriction(t *testing.T) {
	mesh := &TetMesh{
		Nodes:    []Node{{ID: 1}, {ID: 2}, {ID: 3}, {ID: 4}},
		Elements: []TetElement{{ID: 1, Nodes: []int{1, 2, 3, 4}}},
	}
	deck := writeDeckString(t, &AnalysisModel{
		Analysis: AnalysisStatic,
		Mesh:     mesh,
		Material: MaterialProps{Name: "STEEL", YoungMPa: 210000, Poisson: 0.3},
		Contacts: []ContactPair{{Name: "CONTACT0", Slave: []ElemFace{{Elem: 1, Face: 1}}, Master: []ElemFace{{Elem: 1, Face: 2}}, Stiffness: 1e7}},
	})
	if strings.Contains(deck, "*FRICTION") {
		t.Errorf("frictionless contact must not write *FRICTION\n%s", deck)
	}
}

// TestStackedBlocksTransmitCompression is the contact oracle: two boxes meshed SEPARATELY are
// stacked and put in unilateral contact (not bonded). The bottom face is fixed; the top face is
// pushed DOWN (compression). A working contact pair carries the compressive pressure across the
// interface, so the assembly shortens like the monolithic bar it represents,
//
//	|delta| = P*L / (A*E)
//
// and the interface stays closed. (A tension load would instead open the contact and free the
// upper body — the case a *TIE handles and contact deliberately does not.) This validates
// detectContacts + the penalty surface interaction end to end through the real vendored ccx.
func TestStackedBlocksTransmitCompression(t *testing.T) {
	bins := requireSolver(t)
	const (
		h, half = 10.0, 50.0 // mm: two 10×10×50 boxes → a 10×10×100 column
		L       = 2 * half
		young   = 210000.0 // MPa
		p       = 2000.0   // N compressive load (−z) on the top face
	)
	dir := t.TempDir()
	lower := meshBox(t, bins, h, h, half, t.TempDir())
	upper := translateMeshZ(meshBox(t, bins, h, h, half, t.TempDir()), half)
	mesh := mergeTetMeshes([]*TetMesh{lower, upper})

	contacts := detectContacts(mesh, 0.3, contactStiffnessFactor*young)
	if len(contacts) != 1 {
		t.Fatalf("detectContacts found %d pairs, want 1 (the stacked interface)", len(contacts))
	}
	if len(contacts[0].Slave) == 0 || len(contacts[0].Master) == 0 {
		t.Fatalf("contact has empty surfaces: slave=%d master=%d", len(contacts[0].Slave), len(contacts[0].Master))
	}

	root := selectNodes(mesh, func(n Node) bool { return n.Z < eps(L) })
	tip := selectNodes(mesh, func(n Node) bool { return n.Z > L-eps(L) })
	if len(root) == 0 || len(tip) == 0 {
		t.Fatalf("selection failed (root=%d tip=%d)", len(root), len(tip))
	}
	model := &AnalysisModel{
		Analysis: AnalysisStatic,
		Mesh:     mesh,
		Material: MaterialProps{Name: "STEEL", YoungMPa: young, Poisson: 0},
		Fixed:    []FixedConstraint{{Name: "ROOT", Nodes: root, DOFLow: 1, DOFHigh: 3}},
		Forces:   []ForceLoad{{Name: "TIP", Nodes: tip, Dir: [3]float64{0, 0, -1}, TotalN: p}},
		Contacts: contacts,
	}
	res := solveModel(t, bins, model, dir)

	got := math.Abs(meanUZ(res, tip))
	want := p * L / (h * h * young)
	relErr := math.Abs(got-want) / want
	t.Logf("contact column shortening: FE=%.5f mm, monolithic P·L/AE=%.5f mm, rel err=%.1f%%", got, want, relErr*100)
	if relErr > 0.05 {
		t.Errorf("contact shortening %.5f mm differs from monolithic %.5f mm by %.1f%% (>5%%)", got, want, relErr*100)
	}
}
