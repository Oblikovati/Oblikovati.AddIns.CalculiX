// SPDX-License-Identifier: GPL-2.0-only

// Package femmodel is the pure Analysis domain of the CalculiX add-in: a tree of first-class FEM
// objects (Solver, Mesh, Material, Constraint, Result) under an Analysis aggregate. It imports
// neither the host nor oblikovati.org/api — the add-in's ccx package projects it onto the solver
// pipeline (see ccx/project.go). Keeping it pure makes the model unit-testable on every platform.
package femmodel

// Category is the kind of a FEM object within an Analysis.
type Category int

const (
	CategorySolver Category = iota
	CategoryMesh
	CategoryMaterial
	CategoryConstraint
	CategoryResult
)

var categoryNames = map[Category]string{
	CategorySolver: "Solver", CategoryMesh: "Mesh", CategoryMaterial: "Material",
	CategoryConstraint: "Constraint", CategoryResult: "Result",
}

// String returns the category's stable display name.
func (c Category) String() string {
	if n, ok := categoryNames[c]; ok {
		return n
	}
	return "Category(?)"
}
