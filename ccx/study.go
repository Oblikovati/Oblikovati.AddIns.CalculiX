// SPDX-License-Identifier: GPL-2.0-only

package ccx

import "errors"

// StudyResult summarizes one FEA run.
type StudyResult struct {
	FrdPath          string  // the ccx .frd result file
	NodeCount        int     // mesh node count
	ElementCount     int     // mesh tet-element count
	PeakVonMisesMPa  float64 // maximum nodal von Mises stress
	MaxDisplacement  float64 // maximum nodal displacement magnitude (model units)
	GraphicsClientID string  // the client-graphics group the result was pushed under
}

// errNotImplemented is the placeholder result until the M1 pipeline lands. The cgo
// shell + command + panel are wired (M0 scaffold); RunStudyOnHost is built out across
// the surface → volume → deck → solve → render stages in M1.
var errNotImplemented = errors.New("CalculiX study pipeline not yet implemented (M0 scaffold)")

// RunStudyOnHost is the end-to-end add-in flow for the active part: resolve the study
// (material + selected faces), pull and weld the surface mesh, volume-mesh with gmsh,
// write the CalculiX deck, solve with ccx, parse the .frd results, and render the
// stress/displacement field as client graphics.
//
// M0 returns errNotImplemented so the wiring (command → Notify → goroutine → status)
// is exercisable before the heavy stages exist.
func (e *Engine) RunStudyOnHost() (*StudyResult, error) {
	return nil, errNotImplemented
}
