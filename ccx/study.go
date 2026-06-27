// SPDX-License-Identifier: GPL-2.0-only

package ccx

import (
	"fmt"
	"os"
	"path/filepath"
)

// StudyResult summarizes one FEA run.
type StudyResult struct {
	FrdPath          string  // the ccx .frd result file
	NodeCount        int     // mesh node count
	ElementCount     int     // mesh tet-element count
	PeakVonMisesMPa  float64 // maximum nodal von Mises stress
	MaxDisplacement  float64 // maximum nodal displacement magnitude (mm)
	GraphicsClientID string  // the client-graphics group the result was pushed under
}

// studyBodyIndex is the body the v1 study analyses (the active part's first body).
const studyBodyIndex = 0

// RunStudyOnHost is the end-to-end add-in flow for the active part: read the selected
// faces, pull and weld the surface, volume-mesh it with gmsh, bind the picked faces to
// mesh node sets, write the CalculiX deck, solve, parse the .frd, and render the von
// Mises field as client graphics. Convention for the v1 slice: the FIRST selected face
// is the fixed support; the remaining selected faces carry the load (panel magnitude, -Z).
func (e *Engine) RunStudyOnHost() (*StudyResult, error) {
	bins, err := findSolverBinaries()
	if err != nil {
		return nil, err
	}
	e.mu.Lock()
	settings := e.settings
	e.mu.Unlock()

	faces, err := e.selectedFaces(settings.LoadType)
	if err != nil {
		return nil, err
	}
	return e.runStudy(bins, settings, faces)
}

// runStudy executes the mesh -> bind -> deck -> solve -> render pipeline in a fresh
// temporary working directory.
func (e *Engine) runStudy(bins solverBinaries, settings StudySettings, faces []string) (*StudyResult, error) {
	dir, err := os.MkdirTemp("", "ccx-study")
	if err != nil {
		return nil, fmt.Errorf("study workdir: %w", err)
	}
	mesh, err := e.meshActiveBody(bins, settings, dir)
	if err != nil {
		return nil, err
	}
	groups, err := e.buildFaceGroups(studyBodyIndex, faces, mesh)
	if err != nil {
		return nil, err
	}
	model := buildModel(settings, mesh, groups, faces)
	res, err := solveStudyDeck(bins, model, dir)
	if err != nil {
		return nil, err
	}
	return e.renderStudy(mesh, res, model, groups, faces, dir)
}

// selectedFaces returns the picked faces' raw reference keys (decoded from the host's
// "face/<base64>" selection form). A surface load needs a support face plus at least one
// loaded face; a gravity body load needs only the support face.
func (e *Engine) selectedFaces(load LoadType) ([]string, error) {
	sel, err := e.api.Model().Selection()
	if err != nil {
		return nil, fmt.Errorf("read selection: %w", err)
	}
	faces := decodeSelectedFaces(sel.Refs)
	if min := minFaces(load); len(faces) < min {
		return nil, fmt.Errorf("select at least %d face(s) — the first is fixed%s (selected %d faces of %d entities)",
			min, loadHint(load), len(faces), len(sel.Refs))
	}
	return faces, nil
}

// meshActiveBody pulls and welds the active body's surface and volume-meshes it.
func (e *Engine) meshActiveBody(bins solverBinaries, settings StudySettings, dir string) (*TetMesh, error) {
	surface, err := e.pullSurface(studyBodyIndex)
	if err != nil {
		return nil, err
	}
	opts := MeshOptions{SizeMM: settings.MeshSizeMM, Order: settings.ElementOrder}
	return NewGmshMesher(bins.gmsh).Mesh(surface, opts, dir)
}

// renderStudy paints the stress result plus the support/load visual aids, and returns the
// run summary.
func (e *Engine) renderStudy(mesh *TetMesh, res *ResultField, model *AnalysisModel, groups *FaceGroups, faces []string, dir string) (*StudyResult, error) {
	peakVM, err := e.renderResult(mesh, res)
	if err != nil {
		return nil, fmt.Errorf("render result: %w", err)
	}
	if err := e.renderConstraints(mesh, groups, faces, model); err != nil {
		return nil, fmt.Errorf("render constraints: %w", err)
	}
	return &StudyResult{
		FrdPath:          filepath.Join(dir, "study.frd"),
		NodeCount:        len(mesh.Nodes),
		ElementCount:     len(mesh.Elements),
		PeakVonMisesMPa:  peakVM,
		MaxDisplacement:  peak(dispMagnitude(res)),
		GraphicsClientID: resultClientID,
	}, nil
}

// buildModel assembles the analysis model from the settings, mesh, and face bindings: the
// first selected face is fully fixed; the load (force/pressure/gravity) is applied per the
// settings to the remaining faces (or, for gravity, to the whole body).
func buildModel(settings StudySettings, mesh *TetMesh, groups *FaceGroups, faces []string) *AnalysisModel {
	m := &AnalysisModel{
		Analysis: settings.Analysis,
		Mesh:     mesh,
		Material: settings.material(),
		Fixed:    []FixedConstraint{{Name: "FIX", Nodes: groups.Nodes[faces[0]], DOFLow: 1, DOFHigh: 3}},
	}
	applyLoad(m, settings, groups, faces[1:])
	return m
}

// applyLoad attaches the configured load to the model.
func applyLoad(m *AnalysisModel, settings StudySettings, groups *FaceGroups, loadFaces []string) {
	switch settings.LoadType {
	case LoadGravity:
		m.Gravity = &GravityLoad{Accel: settings.GravityG * standardGravityMMs2, Dir: [3]float64{0, 0, -1}}
	case LoadPressure:
		var faces []ElemFace
		for _, key := range loadFaces {
			faces = append(faces, groups.ElemFaces[key]...)
		}
		m.Pressures = []PressureLoad{{Name: "LOAD", Faces: faces, MPa: settings.PressureMPa}}
	default: // LoadForce
		var nodes []int
		for _, key := range loadFaces {
			nodes = append(nodes, groups.Nodes[key]...)
		}
		m.Forces = []ForceLoad{{Name: "LOAD", Nodes: dedupeInts(nodes), Dir: [3]float64{0, 0, -1}, TotalN: settings.LoadN}}
	}
}

// minFaces is the number of selected faces a load type needs: gravity needs only the
// support; force/pressure need the support plus loaded faces.
func minFaces(load LoadType) int {
	if load == LoadGravity {
		return 1
	}
	return 2
}

// loadHint describes the remaining-face requirement for the selection error message.
func loadHint(load LoadType) string {
	if load == LoadGravity {
		return " (gravity loads the whole body)"
	}
	return ", the rest carry the load"
}

// solveStudyDeck writes the deck, runs ccx, and parses the result field.
func solveStudyDeck(bins solverBinaries, model *AnalysisModel, dir string) (*ResultField, error) {
	stem := filepath.Join(dir, "study")
	if err := writeFile(stem+".inp", func(f *os.File) error { return WriteDeck(f, model) }); err != nil {
		return nil, err
	}
	if err := runCcx(bins.ccx, stem); err != nil {
		return nil, err
	}
	f, err := os.Open(stem + ".frd")
	if err != nil {
		return nil, fmt.Errorf("open frd: %w", err)
	}
	defer f.Close()
	return parseFRD(f)
}

// dedupeInts returns the unique ids preserving first-seen order.
func dedupeInts(ids []int) []int {
	seen := make(map[int]bool, len(ids))
	out := ids[:0:0]
	for _, id := range ids {
		if !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	return out
}
