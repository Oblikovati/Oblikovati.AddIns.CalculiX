// SPDX-License-Identifier: GPL-2.0-only

package femmodel

import "testing"

func TestDefaultMaterialHasEMHyperTempDefaults(t *testing.T) {
	mat, ok := NewDefaultAnalysis().DefaultMaterial()
	if !ok {
		t.Fatal("no default material")
	}
	if mat.ElectricalSigma != 1 || mat.MaterialModel != "linear elastic" {
		t.Fatalf("EM/model defaults wrong: sigma=%g model=%q", mat.ElectricalSigma, mat.MaterialModel)
	}
	if mat.NeoHookeC10 != 1.0 || mat.NeoHookeD1 != 0.1 {
		t.Fatalf("neo-hooke defaults wrong: c10=%g d1=%g", mat.NeoHookeC10, mat.NeoHookeD1)
	}
	if mat.YoungHotGPa != 0 || mat.HotTempK != 100 {
		t.Fatalf("temp-dep defaults wrong: hot=%g tk=%g", mat.YoungHotGPa, mat.HotTempK)
	}
}

func TestSetDefaultMaterialCarriesEMHyperTemp(t *testing.T) {
	a := NewDefaultAnalysis()
	mat, _ := a.DefaultMaterial()
	mat.ElectricalSigma, mat.MaterialModel = 2, "neo-hookean (rubber)"
	mat.NeoHookeC10, mat.NeoHookeD1, mat.YoungHotGPa, mat.HotTempK = 3, 0.2, 150, 400
	a.SetDefaultMaterial(mat)
	got, _ := a.DefaultMaterial()
	if got.ElectricalSigma != 2 || got.MaterialModel != "neo-hookean (rubber)" ||
		got.NeoHookeC10 != 3 || got.NeoHookeD1 != 0.2 || got.YoungHotGPa != 150 || got.HotTempK != 400 {
		t.Fatalf("fields not persisted: %+v", got)
	}
}
