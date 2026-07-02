// SPDX-License-Identifier: GPL-2.0-only

package femmodel

import "strconv"

// Analysis is the root aggregate: exactly one Solver and one Mesh, at least one Material (the
// ScopeAll one is the fallback), and the result-display objects. It is the source of truth the
// add-in projects onto its solver pipeline. Mutators preserve the invariants; ids are unique
// within the aggregate.
type Analysis struct {
	solver         SolverObject
	mesh           MeshObject
	materials      []MaterialObject
	results        []ResultObject
	constraints    []ConstraintObject
	nextMat        int
	nextResult     int
	nextConstraint int
}

// NewDefaultAnalysis returns the v1 defaults, matching the add-in's defaultSettings(): linear-static,
// quadratic tets, auto mesh size, a mild-steel fallback material, and a von-Mises result.
func NewDefaultAnalysis() *Analysis {
	a := &Analysis{
		solver: newSolverObject("solver", "static", 6, 0),
		mesh:   newMeshObject("mesh", 0, true),
	}
	a.AddMaterial("Steel", 210, 0.3, 7.85, 0, true)
	steel, _ := a.DefaultMaterial()
	steel.ThermalAlpha, steel.Conductivity, steel.SpecificHeat = 1.2e-5, 50, 5e8
	steel.ElectricalSigma, steel.MaterialModel = 1, "linear elastic"
	steel.NeoHookeC10, steel.NeoHookeD1 = 1.0, 0.1
	steel.YoungHotGPa, steel.HotTempK = 0, 100
	a.SetDefaultMaterial(steel)
	a.AddResult("von Mises stress", 0)
	sv := a.Solver()
	sv.BodyScope, sv.ContactMode, sv.FrictionMu = "all solid bodies", false, 0.3
	a.SetSolver(sv)
	return a
}

// Solver returns the single solver object.
func (a *Analysis) Solver() SolverObject { return a.solver }

// Mesh returns the single mesh object.
func (a *Analysis) Mesh() MeshObject { return a.mesh }

// Materials returns the materials in insertion order.
func (a *Analysis) Materials() []MaterialObject { return a.materials }

// Results returns the result objects in insertion order.
func (a *Analysis) Results() []ResultObject { return a.results }

// SetSolver replaces the solver object (preserving its id).
func (a *Analysis) SetSolver(s SolverObject) { s.id = a.solver.id; a.solver = s }

// SetMesh replaces the mesh object (preserving its id).
func (a *Analysis) SetMesh(m MeshObject) { m.id = a.mesh.id; a.mesh = m }

// SetDefaultMaterial replaces the ScopeAll fallback material's mechanical fields, preserving its
// id and ScopeAll flag (upholding the ≥1-ScopeAll-material invariant). If no ScopeAll material
// exists yet, it updates the first material.
func (a *Analysis) SetDefaultMaterial(m MaterialObject) {
	for i := range a.materials {
		if a.materials[i].ScopeAll {
			m.id, m.name, m.ScopeAll = a.materials[i].id, a.materials[i].name, true
			a.materials[i] = m
			return
		}
	}
	if len(a.materials) > 0 {
		m.id, m.name = a.materials[0].id, a.materials[0].name
		m.ScopeAll = a.materials[0].ScopeAll
		a.materials[0] = m
	}
}

// SetPrimaryResult replaces the first result object's fields, preserving its id (keeps ≥1 Result).
func (a *Analysis) SetPrimaryResult(r ResultObject) {
	if len(a.results) == 0 {
		return
	}
	r.id = a.results[0].id
	a.results[0] = r
}

// AddMaterial appends a material with a fresh unique id and returns it.
func (a *Analysis) AddMaterial(name string, young, poisson, density, yield float64, scopeAll bool) MaterialObject {
	a.nextMat++
	m := newMaterialObject("mat"+strconv.Itoa(a.nextMat), name, young, poisson, density, yield, scopeAll)
	a.materials = append(a.materials, m)
	return m
}

// AddResult appends a result-display object with a fresh unique id and returns it.
func (a *Analysis) AddResult(field string, deformScale float64) ResultObject {
	a.nextResult++
	r := newResultObject("result"+strconv.Itoa(a.nextResult), field, deformScale)
	a.results = append(a.results, r)
	return r
}

// DefaultMaterial returns the ScopeAll fallback material (the first one if none is explicitly
// ScopeAll), and false only when there is no material at all.
func (a *Analysis) DefaultMaterial() (MaterialObject, bool) {
	if len(a.materials) == 0 {
		return MaterialObject{}, false
	}
	for _, m := range a.materials {
		if m.ScopeAll {
			return m, true
		}
	}
	return a.materials[0], true
}

// PrimaryResult returns the first result-display object, false when there is none.
func (a *Analysis) PrimaryResult() (ResultObject, bool) {
	if len(a.results) == 0 {
		return ResultObject{}, false
	}
	return a.results[0], true
}

// Constraints returns the explicit constraint list in creation order.
func (a *Analysis) Constraints() []ConstraintObject { return a.constraints }

// AddConstraint appends a constraint with a fresh unique id and the given name, returning it.
func (a *Analysis) AddConstraint(name string, o ConstraintObject) ConstraintObject {
	a.nextConstraint++
	o.id = "con" + strconv.Itoa(a.nextConstraint)
	o.name = name
	a.constraints = append(a.constraints, o)
	return o
}

// ClearConstraints empties the explicit constraint list.
func (a *Analysis) ClearConstraints() { a.constraints = nil }
