// SPDX-License-Identifier: GPL-2.0-only

package femmodel

import "testing"

func TestDefaultMaterialHasThermalDefaults(t *testing.T) {
	a := NewDefaultAnalysis()
	mat, ok := a.DefaultMaterial()
	if !ok {
		t.Fatal("no default material")
	}
	if mat.ThermalAlpha != 1.2e-5 || mat.Conductivity != 50 || mat.SpecificHeat != 5e8 {
		t.Fatalf("thermal defaults wrong: alpha=%g cond=%g cp=%g", mat.ThermalAlpha, mat.Conductivity, mat.SpecificHeat)
	}
}

func TestSetDefaultMaterialCarriesThermal(t *testing.T) {
	a := NewDefaultAnalysis()
	mat, _ := a.DefaultMaterial()
	mat.ThermalAlpha, mat.Conductivity, mat.SpecificHeat = 2e-5, 80, 4e8
	a.SetDefaultMaterial(mat)
	got, _ := a.DefaultMaterial()
	if got.ThermalAlpha != 2e-5 || got.Conductivity != 80 || got.SpecificHeat != 4e8 {
		t.Fatalf("thermal not persisted: %+v", got)
	}
}
