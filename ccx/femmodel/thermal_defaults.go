// SPDX-License-Identifier: GPL-2.0-only
package femmodel

// ThermalDefaults holds the thermal boundary-condition parameters synthesized at solve time
// for a heat-transfer / thermomechanical study. It is a study-wide template (not a browser-tree
// node), mirroring LoadDefaults and SupportDefaults. HeatDriveMode is a neutral string here —
// the ccx layer maps it to its HeatDrive display enum (flux/convection/body source/radiation).
type ThermalDefaults struct {
	HeatDriveMode string  // how loaded faces exchange heat: flux, convection, body source, radiation
	DeltaK        float64 // temperature change (K) for a thermomechanical study
	ColdTempK     float64 // prescribed temperature on the support face (K)
	HeatFluxQ     float64 // surface heat flux on the remaining faces
	FilmCoeff     float64 // convective film coefficient h (convection mode)
	SinkTempK     float64 // ambient/sink temperature for convection (K)
	BodyHeatRate  float64 // volumetric internal heat generation (body-source mode)
	Emissivity    float64 // surface emissivity 0..1 (radiation mode)
	RadAmbientK   float64 // ambient temperature radiated to (K) (radiation mode)
}
