// SPDX-License-Identifier: GPL-2.0-only

package femmodel

// ResultObject is the result-display object: which scalar field colours the result and the
// deformed-shape magnification (0 = auto). Post-processing filters attach here in Phase 5.
type ResultObject struct {
	id          string
	Field       string
	DeformScale float64
}

func newResultObject(id, field string, deformScale float64) ResultObject {
	return ResultObject{id: id, Field: field, DeformScale: deformScale}
}

func (o ResultObject) ObjectID() string  { return o.id }
func (o ResultObject) Category() Category { return CategoryResult }
func (o ResultObject) Name() string       { return "Results" }
