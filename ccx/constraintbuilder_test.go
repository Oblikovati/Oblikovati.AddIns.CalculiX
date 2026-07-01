// SPDX-License-Identifier: GPL-2.0-only

package ccx

import (
	"encoding/json"
	"testing"

	"oblikovati.org/api/wire"
)

// builderHost is a permissive fake: it returns a fixed face selection and accepts every other
// host call (panel redraw, status text) so the constraint-builder flow can run headless.
type builderHost struct{ refs []string }

func (h *builderHost) Call(method string, _ []byte) ([]byte, error) {
	if method == wire.MethodModelSelection {
		return json.Marshal(wire.SelectionResult{Count: len(h.refs), Refs: h.refs})
	}
	return []byte("{}"), nil
}

func TestNewConstraintSpecFactory(t *testing.T) {
	s := defaultSettings()
	s.LoadN = 500
	s.SpringStiffMM = 2000
	cases := map[ConstraintKind]func(ConstraintSpec) bool{
		KindFixed:          func(c ConstraintSpec) bool { _, ok := c.(FixedSpec); return ok },
		KindRoller:         func(c ConstraintSpec) bool { _, ok := c.(RollerSpec); return ok },
		KindSymmetry:       func(c ConstraintSpec) bool { _, ok := c.(SymmetrySpec); return ok },
		KindElasticSupport: func(c ConstraintSpec) bool { e, ok := c.(ElasticSupportSpec); return ok && e.StiffnessTotal == 2000 },
		KindForce:          func(c ConstraintSpec) bool { f, ok := c.(ForceSpec); return ok && f.TotalN == 500 },
		KindPressure:       func(c ConstraintSpec) bool { _, ok := c.(PressureSpec); return ok },
		KindHydrostatic:    func(c ConstraintSpec) bool { _, ok := c.(HydrostaticSpec); return ok },
		KindDisplacement:   func(c ConstraintSpec) bool { _, ok := c.(DisplacementSpec); return ok },
	}
	for kind, ok := range cases {
		spec := newConstraintSpec(kind, "C0", []string{"f"}, s)
		if spec.Kind() != kind || !ok(spec) {
			t.Errorf("newConstraintSpec(%s) produced %T with wrong kind/params", kind, spec)
		}
	}
}

func TestAddConstraintFromSelection(t *testing.T) {
	e := NewEngine(&builderHost{refs: []string{encodeFaceRef("kA"), encodeFaceRef("kB")}})
	e.extras.BuilderKind = KindRoller

	e.addConstraintFromSelection()
	if len(e.extras.Constraints) != 1 {
		t.Fatalf("expected 1 constraint after add, got %d", len(e.extras.Constraints))
	}
	roller, ok := e.extras.Constraints[0].(RollerSpec)
	if !ok || len(roller.Faces) != 2 || roller.Name != "C0" {
		t.Fatalf("added constraint should be a RollerSpec named C0 over 2 faces, got %+v", e.extras.Constraints[0])
	}

	// A second add appends with a fresh unique name.
	e.extras.BuilderKind = KindPressure
	e.addConstraintFromSelection()
	if len(e.extras.Constraints) != 2 || e.extras.Constraints[1].Kind() != KindPressure {
		t.Fatalf("second add should append a pressure constraint, got %+v", e.extras.Constraints)
	}

	e.clearConstraints()
	if len(e.extras.Constraints) != 0 {
		t.Fatalf("clear should empty the list, got %d", len(e.extras.Constraints))
	}
}

func TestAddConstraintWithoutSelectionAddsNothing(t *testing.T) {
	e := NewEngine(&builderHost{refs: nil})
	e.addConstraintFromSelection()
	if len(e.extras.Constraints) != 0 {
		t.Fatalf("an empty selection must add no constraint, got %d", len(e.extras.Constraints))
	}
}
