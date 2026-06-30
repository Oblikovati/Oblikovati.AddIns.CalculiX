// SPDX-License-Identifier: GPL-2.0-only

package femmodel

// SolverObject is the analysis-procedure object: which CalculiX *STEP to run, how many eigenmodes
// (frequency/buckling), and the transient total time (0 = steady). AnalysisType is the canonical
// analysis name string (e.g. "static") the add-in maps to its ccx.AnalysisType.
type SolverObject struct {
	id             string
	AnalysisType   string
	Eigenmodes     int
	TransientTimeS float64
}

func newSolverObject(id, analysisType string, eigenmodes int, transientS float64) SolverObject {
	return SolverObject{id: id, AnalysisType: analysisType, Eigenmodes: eigenmodes, TransientTimeS: transientS}
}

func (o SolverObject) ObjectID() string   { return o.id }
func (o SolverObject) Category() Category { return CategorySolver }
func (o SolverObject) Name() string       { return "Solver" }
