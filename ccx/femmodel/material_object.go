// SPDX-License-Identifier: GPL-2.0-only

package femmodel

// MaterialObject is one material assignment. ScopeAll marks the fallback material applied to every
// body that has no more-specific assignment. Carries mechanical, thermal, electromagnetic,
// hyperelastic (Neo-Hookean), and temperature-dependent stiffness properties.
type MaterialObject struct {
	id           string
	name         string
	YoungGPa     float64
	Poisson      float64
	DensityGCm3  float64
	YieldMPa     float64
	ThermalAlpha    float64 // thermal expansion coefficient (1/K)
	Conductivity    float64 // thermal conductivity (consistent units)
	SpecificHeat    float64 // specific heat capacity (consistent units; transient)
	ElectricalSigma float64 // electrical conductivity (consistent units; electrostatic study)
	MaterialModel   string  // constitutive law name: "linear elastic" | "neo-hookean (rubber)"
	NeoHookeC10     float64 // Neo-Hookean C10 (MPa), for the hyperelastic model
	NeoHookeD1      float64 // Neo-Hookean D1 (1/MPa) compressibility, for the hyperelastic model
	YoungHotGPa     float64 // Young's modulus (GPa) at HotTempK; >0 builds a temperature-dependent E(T) table
	HotTempK        float64 // upper table temperature (K) at which YoungHotGPa applies
	ScopeAll        bool
}

func newMaterialObject(id, name string, young, poisson, density, yield float64, scopeAll bool) MaterialObject {
	return MaterialObject{
		id: id, name: name, YoungGPa: young, Poisson: poisson,
		DensityGCm3: density, YieldMPa: yield, ScopeAll: scopeAll,
	}
}

func (o MaterialObject) ObjectID() string   { return o.id }
func (o MaterialObject) Category() Category { return CategoryMaterial }
func (o MaterialObject) Name() string       { return o.name }
