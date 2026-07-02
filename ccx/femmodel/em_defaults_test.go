// SPDX-License-Identifier: GPL-2.0-only
package femmodel

import "testing"

func TestDefaultEM(t *testing.T) {
	em := NewDefaultAnalysis().EM()
	if em.EMDriveMode != "voltage" {
		t.Fatalf("EMDriveMode = %q, want \"voltage\"", em.EMDriveMode)
	}
	if em.VoltageV != 5 || em.CurrentDensity != 1 {
		t.Fatalf("EM magnitudes = {%v %v}, want {5 1}", em.VoltageV, em.CurrentDensity)
	}
}

func TestSetEM(t *testing.T) {
	a := NewDefaultAnalysis()
	a.SetEM(EMDefaults{EMDriveMode: "current", VoltageV: 12, CurrentDensity: 7})
	got := a.EM()
	if got.EMDriveMode != "current" || got.VoltageV != 12 || got.CurrentDensity != 7 {
		t.Fatalf("EM() = %+v, want {current 12 7}", got)
	}
}
