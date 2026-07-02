// SPDX-License-Identifier: GPL-2.0-only

package femmodel

import "testing"

func TestDefaultLoad(t *testing.T) {
	ld := NewDefaultAnalysis().Load()
	if ld.LoadType != "force" || ld.LoadN != 100 || ld.PressureMPa != 1 || ld.GravityG != 1 {
		t.Fatalf("load defaults wrong (1): %+v", ld)
	}
	if ld.RotationRadS != 100 || ld.DisplacementMM != 0.1 || ld.HydroGradientMPaMM != 1e-5 || ld.HydroSurfaceZ != 0 {
		t.Fatalf("load defaults wrong (2): %+v", ld)
	}
}

func TestSetLoad(t *testing.T) {
	a := NewDefaultAnalysis()
	a.SetLoad(LoadDefaults{LoadType: "pressure", PressureMPa: 5, LoadN: 7, GravityG: 2,
		RotationRadS: 50, DisplacementMM: 0.3, HydroGradientMPaMM: 2e-5, HydroSurfaceZ: 10})
	got := a.Load()
	if got.LoadType != "pressure" || got.PressureMPa != 5 || got.HydroSurfaceZ != 10 {
		t.Fatalf("SetLoad not persisted: %+v", got)
	}
}
