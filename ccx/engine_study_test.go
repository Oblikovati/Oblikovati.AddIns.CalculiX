// SPDX-License-Identifier: GPL-2.0-only

package ccx

import (
	"reflect"
	"testing"
)

// study() must project the default Engine (a default analysis aggregate) to exactly
// defaultSettings() — the behavior-preserving guard for the ownership flip.
func TestEngineStudyProjectsDefaults(t *testing.T) {
	e := NewEngine(nil)
	got, specs := e.study()
	if !reflect.DeepEqual(got, defaultSettings()) {
		t.Fatalf("study() drifted from defaults:\n got=%+v\nwant=%+v", got, defaultSettings())
	}
	if len(specs) != 0 {
		t.Fatalf("expected no constraints, got %d", len(specs))
	}
}
