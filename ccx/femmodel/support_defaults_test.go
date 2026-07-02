// SPDX-License-Identifier: GPL-2.0-only
package femmodel

import "testing"

func TestDefaultSupport(t *testing.T) {
	s := NewDefaultAnalysis().Support()
	if s.SupportType != "fixed" {
		t.Fatalf("SupportType = %q, want \"fixed\"", s.SupportType)
	}
	if s.SpringStiffMM != 1000 {
		t.Fatalf("SpringStiffMM = %v, want 1000", s.SpringStiffMM)
	}
}

func TestSetSupport(t *testing.T) {
	a := NewDefaultAnalysis()
	a.SetSupport(SupportDefaults{SupportType: "elastic (spring)", SpringStiffMM: 42})
	got := a.Support()
	if got.SupportType != "elastic (spring)" || got.SpringStiffMM != 42 {
		t.Fatalf("Support() = %+v, want {elastic (spring) 42}", got)
	}
}
