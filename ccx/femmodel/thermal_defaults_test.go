// SPDX-License-Identifier: GPL-2.0-only
package femmodel

import "testing"

func TestDefaultThermal(t *testing.T) {
	th := NewDefaultAnalysis().Thermal()
	if th.HeatDriveMode != "flux" {
		t.Fatalf("HeatDriveMode = %q, want \"flux\"", th.HeatDriveMode)
	}
	if th.DeltaK != 100 || th.ColdTempK != 0 || th.HeatFluxQ != 50 {
		t.Fatalf("core temps = {%v %v %v}, want {100 0 50}", th.DeltaK, th.ColdTempK, th.HeatFluxQ)
	}
	if th.FilmCoeff != 0.5 || th.SinkTempK != 0 || th.BodyHeatRate != 1 {
		t.Fatalf("conv/body = {%v %v %v}, want {0.5 0 1}", th.FilmCoeff, th.SinkTempK, th.BodyHeatRate)
	}
	if th.Emissivity != 0.8 || th.RadAmbientK != 300 {
		t.Fatalf("radiation = {%v %v}, want {0.8 300}", th.Emissivity, th.RadAmbientK)
	}
}

func TestSetThermal(t *testing.T) {
	a := NewDefaultAnalysis()
	a.SetThermal(ThermalDefaults{HeatDriveMode: "convection", DeltaK: 5, ColdTempK: 1, HeatFluxQ: 2,
		FilmCoeff: 3, SinkTempK: 4, BodyHeatRate: 6, Emissivity: 0.2, RadAmbientK: 290})
	got := a.Thermal()
	if got.HeatDriveMode != "convection" || got.DeltaK != 5 || got.FilmCoeff != 3 || got.RadAmbientK != 290 {
		t.Fatalf("Thermal() = %+v, want mode=convection DeltaK=5 FilmCoeff=3 RadAmbientK=290", got)
	}
}
