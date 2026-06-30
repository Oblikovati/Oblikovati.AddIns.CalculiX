// SPDX-License-Identifier: GPL-2.0-only

package femmodel

// MaterialObject is one material assignment. ScopeAll marks the fallback material applied to every
// body that has no more-specific assignment. Phase 1 carries the core mechanical properties; thermal
// and electromagnetic properties migrate here in a later phase.
type MaterialObject struct {
	id          string
	name        string
	YoungGPa    float64
	Poisson     float64
	DensityGCm3 float64
	YieldMPa    float64
	ScopeAll    bool
}

func newMaterialObject(id, name string, young, poisson, density, yield float64, scopeAll bool) MaterialObject {
	return MaterialObject{
		id: id, name: name, YoungGPa: young, Poisson: poisson,
		DensityGCm3: density, YieldMPa: yield, ScopeAll: scopeAll,
	}
}

func (o MaterialObject) ObjectID() string  { return o.id }
func (o MaterialObject) Category() Category { return CategoryMaterial }
func (o MaterialObject) Name() string       { return o.name }
