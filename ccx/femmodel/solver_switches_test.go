// SPDX-License-Identifier: GPL-2.0-only

package femmodel

import "testing"

func TestDefaultSolverHasStudySwitches(t *testing.T) {
	sv := NewDefaultAnalysis().Solver()
	if sv.BodyScope != "all solid bodies" || sv.ContactMode || sv.FrictionMu != 0.3 {
		t.Fatalf("study-switch defaults wrong: scope=%q contact=%v mu=%g", sv.BodyScope, sv.ContactMode, sv.FrictionMu)
	}
}

func TestSetSolverCarriesStudySwitches(t *testing.T) {
	a := NewDefaultAnalysis()
	sv := a.Solver()
	sv.BodyScope, sv.ContactMode, sv.FrictionMu = "bodies with a selected face", true, 0.15
	a.SetSolver(sv)
	got := a.Solver()
	if got.BodyScope != "bodies with a selected face" || !got.ContactMode || got.FrictionMu != 0.15 {
		t.Fatalf("switches not persisted: %+v", got)
	}
}
