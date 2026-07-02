// SPDX-License-Identifier: GPL-2.0-only

package femmodel

// LoadDefaults holds the parameters of the study's default load — the numbers the implicit
// convention (and the explicit builder) apply to the loaded faces. Not a tree node: the load is
// synthesized at solve time from the selection, so this carries only its params (LoadType as a
// string; ccx casts it). One per Analysis.
type LoadDefaults struct {
	LoadType           string
	LoadN              float64
	PressureMPa        float64
	GravityG           float64
	RotationRadS       float64
	DisplacementMM     float64
	HydroGradientMPaMM float64
	HydroSurfaceZ      float64
}
