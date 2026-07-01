// SPDX-License-Identifier: GPL-2.0-only

package femmodel

import "testing"

func TestSetDefaultMaterialPreservesIDAndScope(t *testing.T) {
	a := NewDefaultAnalysis()
	orig, _ := a.DefaultMaterial()
	repl := MaterialObject{YoungGPa: 69, Poisson: 0.33, DensityGCm3: 2.70, YieldMPa: 40}
	a.SetDefaultMaterial(repl)
	got, ok := a.DefaultMaterial()
	if !ok || got.ObjectID() != orig.ObjectID() {
		t.Fatalf("id not preserved: got %q want %q", got.ObjectID(), orig.ObjectID())
	}
	if !got.ScopeAll {
		t.Fatalf("ScopeAll not preserved: %+v", got)
	}
	if got.YoungGPa != 69 || got.Poisson != 0.33 || got.DensityGCm3 != 2.70 || got.YieldMPa != 40 {
		t.Fatalf("fields not updated: %+v", got)
	}
}

func TestSetPrimaryResultPreservesID(t *testing.T) {
	a := NewDefaultAnalysis()
	orig, _ := a.PrimaryResult()
	a.SetPrimaryResult(ResultObject{Field: "displacement magnitude", DeformScale: 5})
	got, ok := a.PrimaryResult()
	if !ok || got.ObjectID() != orig.ObjectID() {
		t.Fatalf("id not preserved: got %q want %q", got.ObjectID(), orig.ObjectID())
	}
	if got.Field != "displacement magnitude" || got.DeformScale != 5 {
		t.Fatalf("fields not updated: %+v", got)
	}
}
