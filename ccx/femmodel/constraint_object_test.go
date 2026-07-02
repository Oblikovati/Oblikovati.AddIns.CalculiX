// SPDX-License-Identifier: GPL-2.0-only

package femmodel

import "testing"

func TestAddConstraintAssignsIDAndName(t *testing.T) {
	a := NewDefaultAnalysis()
	if len(a.Constraints()) != 0 {
		t.Fatalf("default analysis should have no constraints, got %d", len(a.Constraints()))
	}
	c1 := a.AddConstraint("C0", ConstraintObject{Kind: "force", Faces: []string{"face/a"}, TotalN: 100})
	c2 := a.AddConstraint("C1", ConstraintObject{Kind: "fixed", Faces: []string{"face/b"}})
	if c1.ObjectID() == c2.ObjectID() {
		t.Fatalf("constraint ids collide: %q", c1.ObjectID())
	}
	if c1.Name() != "C0" || c1.Kind != "force" || c1.TotalN != 100 || c1.Category() != CategoryConstraint {
		t.Fatalf("constraint 1 wrong: %+v", c1)
	}
	if got := a.Constraints(); len(got) != 2 || got[1].Kind != "fixed" {
		t.Fatalf("Constraints() = %+v, want two ending fixed", got)
	}
}

func TestClearConstraints(t *testing.T) {
	a := NewDefaultAnalysis()
	a.AddConstraint("C0", ConstraintObject{Kind: "fixed"})
	a.ClearConstraints()
	if len(a.Constraints()) != 0 {
		t.Fatalf("ClearConstraints left %d", len(a.Constraints()))
	}
}
