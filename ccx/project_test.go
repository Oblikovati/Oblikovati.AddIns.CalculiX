// SPDX-License-Identifier: GPL-2.0-only

package ccx

import (
	"reflect"
	"testing"

	"oblikovati.org/calculix/ccx/femmodel"
)

// The seam must preserve the v1 defaults exactly: projecting the default Analysis must reproduce
// defaultSettings() field-for-field. This is the safety net that lets later phases migrate fields
// into the tree one at a time without drifting the solve inputs.
func TestProjectDefaultAnalysisEqualsDefaultSettings(t *testing.T) {
	got, specs := projectAnalysis(femmodel.NewDefaultAnalysis())
	want := defaultSettings()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("projection drifted from defaults:\n got=%+v\nwant=%+v", got, want)
	}
	if len(specs) != 0 {
		t.Fatalf("expected no constraints from a default analysis, got %d", len(specs))
	}
}

// Overrides on the tree must flow through the projection.
func TestProjectAppliesTreeOverrides(t *testing.T) {
	a := femmodel.NewDefaultAnalysis()
	a.SetSolver(femmodel.SolverObject{AnalysisType: "frequency", Eigenmodes: 12, TransientTimeS: 0})
	a.SetMesh(femmodel.MeshObject{MaxSizeMM: 2.5, Quadratic: false})
	got, _ := projectAnalysis(a)
	if got.Analysis != AnalysisFrequency || got.Eigenmodes != 12 {
		t.Fatalf("solver override lost: %+v", got)
	}
	if got.MeshSizeMM != 2.5 || got.ElementOrder != LinearTet {
		t.Fatalf("mesh override lost: %+v", got)
	}
}
