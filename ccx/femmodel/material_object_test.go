// SPDX-License-Identifier: GPL-2.0-only

package femmodel

import "testing"

func TestMaterialObject(t *testing.T) {
	m := newMaterialObject("mat1", "Steel", 210, 0.3, 7.85, 0, true)
	if m.ObjectID() != "mat1" || m.Category() != CategoryMaterial || m.Name() != "Steel" {
		t.Fatalf("material identity wrong: %+v", m)
	}
	if m.YoungGPa != 210 || m.Poisson != 0.3 || m.DensityGCm3 != 7.85 || m.YieldMPa != 0 || !m.ScopeAll {
		t.Fatalf("material fields wrong: %+v", m)
	}
}
