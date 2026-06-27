// SPDX-License-Identifier: GPL-2.0-only

package ccx

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// StudyResult summarizes one FEA run. Static studies report stress/displacement; modal
// and buckling studies report their eigenvalues (Modes) with a kind and unit.
type StudyResult struct {
	FrdPath          string             // the ccx .frd result file
	NodeCount        int                // mesh node count
	ElementCount     int                // mesh tet-element count
	FieldLabel       string             // the rendered stress-result field ("von Mises stress", …)
	FieldPeak        float64            // peak value of that field (static)
	FieldUnit        string             // unit of that field ("MPa" / "mm")
	MaxDisplacement  float64            // maximum nodal displacement magnitude (mm, static)
	Modes            []float64          // natural frequencies (Hz) or buckling factors
	ModeKind         string             // "natural frequencies" / "buckling factors"
	ModeUnit         string             // "Hz" / "x load"
	Scalar           *ScalarFieldResult // set for a DOF-11 field study (heat / electrostatic)
	GraphicsClientID string             // the client-graphics group the result was pushed under
}

// ScalarFieldResult is the range of a nodal DOF-11 field: temperature for a heat-transfer
// study, electric potential for the electrostatic analogy. Label/Unit make the status line
// read in the analysis's own terms.
type ScalarFieldResult struct {
	Label string
	Min   float64
	Max   float64
	Unit  string
}

// Summary renders the one-line status message for the run, formatted for the analysis type.
func (r *StudyResult) Summary() string {
	switch {
	case r.Scalar != nil:
		return fmt.Sprintf("CalculiX: %d elements, %s %.4g..%.4g %s",
			r.ElementCount, r.Scalar.Label, r.Scalar.Min, r.Scalar.Max, r.Scalar.Unit)
	case len(r.Modes) > 0:
		return fmt.Sprintf("CalculiX: %d elements, %s: %s", r.ElementCount, r.ModeKind, formatModes(r.Modes, r.ModeUnit))
	default:
		return fmt.Sprintf("CalculiX: %d elements, peak %s %.3g %s, max displacement %.3g mm.",
			r.ElementCount, r.FieldLabel, r.FieldPeak, r.FieldUnit, r.MaxDisplacement)
	}
}

// formatModes joins the first few mode values with their unit for the status bar.
func formatModes(modes []float64, unit string) string {
	const maxShown = 4
	parts := make([]string, 0, maxShown)
	for i, v := range modes {
		if i >= maxShown {
			parts = append(parts, "…")
			break
		}
		parts = append(parts, fmt.Sprintf("%.4g %s", v, unit))
	}
	return strings.Join(parts, ", ")
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

	faces, err := e.selectedFaces(settings)
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
	if err := checkPrerequisites(model); err != nil {
		return nil, err
	}
	stem, err := runDeck(bins, model, dir)
	if err != nil {
		return nil, err
	}
	return e.collectResults(stem, mesh, groups, faces, model)
}

// collectResults reads and renders the analysis-appropriate result fields.
func (e *Engine) collectResults(stem string, mesh *TetMesh, groups *FaceGroups, faces []string, model *AnalysisModel) (*StudyResult, error) {
	switch model.Analysis {
	case AnalysisFrequency, AnalysisBuckling:
		return e.collectModal(stem, mesh, groups, faces, model)
	case AnalysisHeatTransfer:
		return e.collectScalarField(stem, mesh, groups, faces, model, "temperature", "K")
	case AnalysisElectromagnetic:
		return e.collectScalarField(stem, mesh, groups, faces, model, "electric potential", "V")
	default:
		return e.collectStatic(stem, mesh, groups, faces, model)
	}
}

// selectedFaces returns the picked faces' raw reference keys (decoded from the host's
// "face/<base64>" selection form). A surface load needs a support face plus at least one
// loaded face; a gravity body load needs only the support face.
func (e *Engine) selectedFaces(settings StudySettings) ([]string, error) {
	sel, err := e.api.Model().Selection()
	if err != nil {
		return nil, fmt.Errorf("read selection: %w", err)
	}
	faces := decodeSelectedFaces(sel.Refs)
	if min := facesNeeded(settings); len(faces) < min {
		return nil, fmt.Errorf("select at least %d face(s) — the first is fixed%s (selected %d faces of %d entities)",
			min, loadHint(settings.LoadType), len(faces), len(sel.Refs))
	}
	return faces, nil
}

// facesNeeded is how many selected faces an analysis needs: a modal or thermal-stress study
// needs only the support (no loaded face); a static or buckling study needs the support plus
// the load faces.
func facesNeeded(s StudySettings) int {
	switch s.Analysis {
	case AnalysisFrequency, AnalysisThermomech:
		return 1 // support / body field only
	case AnalysisHeatTransfer:
		return 2 // a prescribed-temperature face and a heat-flux face
	case AnalysisElectromagnetic:
		return 2 // an applied-potential face and a ground face
	default:
		return minFaces(s.LoadType)
	}
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

// collectStatic parses the static .frd, paints the von Mises field plus the support/load
// aids, and returns the stress/displacement summary.
func (e *Engine) collectStatic(stem string, mesh *TetMesh, groups *FaceGroups, faces []string, model *AnalysisModel) (*StudyResult, error) {
	res, err := parseFRDFile(stem + ".frd")
	if err != nil {
		return nil, err
	}
	fieldPeak, label, unit, err := e.renderResult(mesh, res, model.ResultField)
	if err != nil {
		return nil, fmt.Errorf("render result: %w", err)
	}
	if err := e.renderConstraints(mesh, groups, faces, model); err != nil {
		return nil, fmt.Errorf("render constraints: %w", err)
	}
	return &StudyResult{
		FrdPath:          stem + ".frd",
		NodeCount:        len(mesh.Nodes),
		ElementCount:     len(mesh.Elements),
		FieldLabel:       label,
		FieldPeak:        fieldPeak,
		FieldUnit:        unit,
		MaxDisplacement:  peak(dispMagnitude(res)),
		GraphicsClientID: resultClientID,
	}, nil
}

// collectModal reads the eigenvalues (natural frequencies or buckling factors) from the
// .dat file, paints the first mode shape as a displacement-magnitude field plus the
// constraint aids, and returns the eigenvalue summary.
func (e *Engine) collectModal(stem string, mesh *TetMesh, groups *FaceGroups, faces []string, model *AnalysisModel) (*StudyResult, error) {
	modes, kind, unit, err := readEigenvalues(stem+".dat", model.Analysis)
	if err != nil {
		return nil, err
	}
	if err := e.renderModeShape(stem+".frd", mesh); err != nil {
		return nil, fmt.Errorf("render mode shape: %w", err)
	}
	if err := e.renderConstraints(mesh, groups, faces, model); err != nil {
		return nil, fmt.Errorf("render constraints: %w", err)
	}
	return &StudyResult{
		FrdPath:          stem + ".frd",
		NodeCount:        len(mesh.Nodes),
		ElementCount:     len(mesh.Elements),
		Modes:            modes,
		ModeKind:         kind,
		ModeUnit:         unit,
		GraphicsClientID: resultClientID,
	}, nil
}

// collectScalarField reads the steady-state nodal DOF-11 field from the .frd (temperature
// for heat transfer, electric potential for the electrostatic analogy), paints it as a
// flood plot plus the boundary-condition aids, and returns the field range labelled in the
// analysis's own terms.
func (e *Engine) collectScalarField(stem string, mesh *TetMesh, groups *FaceGroups, faces []string, model *AnalysisModel, label, unit string) (*StudyResult, error) {
	f, err := os.Open(stem + ".frd")
	if err != nil {
		return nil, fmt.Errorf("open frd: %w", err)
	}
	defer f.Close()
	values, err := parseNodalTemperatures(f)
	if err != nil {
		return nil, err
	}
	if err := e.renderScalarField(mesh, values); err != nil {
		return nil, fmt.Errorf("render %s: %w", label, err)
	}
	if err := e.renderConstraints(mesh, groups, faces, model); err != nil {
		return nil, fmt.Errorf("render constraints: %w", err)
	}
	lo, hi := minMaxField(values)
	return &StudyResult{
		FrdPath:          stem + ".frd",
		NodeCount:        len(mesh.Nodes),
		ElementCount:     len(mesh.Elements),
		Scalar:           &ScalarFieldResult{Label: label, Min: lo, Max: hi, Unit: unit},
		GraphicsClientID: resultClientID,
	}, nil
}

// readEigenvalues parses the analysis-appropriate eigenvalue table and returns the values
// with a human-readable kind and unit.
func readEigenvalues(datPath string, a AnalysisType) ([]float64, string, string, error) {
	f, err := os.Open(datPath)
	if err != nil {
		return nil, "", "", fmt.Errorf("open dat: %w", err)
	}
	defer f.Close()
	if a == AnalysisBuckling {
		factors, err := parseBucklingFactors(f)
		return factors, "buckling factors", "x load", err
	}
	freqs, err := parseEigenFrequencies(f)
	return freqs, "natural frequencies", "Hz", err
}

// buildModel assembles the analysis model from the settings, mesh, and face bindings. A
// mechanical/thermal-stress study fixes the first face and loads the rest; a heat-transfer
// study instead prescribes a temperature on the first face and a heat flux on the rest.
func buildModel(settings StudySettings, mesh *TetMesh, groups *FaceGroups, faces []string) *AnalysisModel {
	m := &AnalysisModel{
		Analysis:       settings.Analysis,
		Mesh:           mesh,
		Material:       settings.material(),
		EigenmodeCount: settings.eigenmodeCount(),
		ResultField:    settings.ResultField,
	}
	if settings.Analysis == AnalysisHeatTransfer {
		applyThermalBCs(m, settings, groups, faces)
		return m
	}
	if settings.Analysis == AnalysisElectromagnetic {
		applyElectrostaticBCs(m, settings, groups, faces)
		return m
	}
	m.Fixed = []FixedConstraint{{Name: "FIX", Nodes: groups.Nodes[faces[0]], DOFLow: 1, DOFHigh: 3}}
	switch settings.Analysis {
	case AnalysisFrequency:
		// A modal (free-vibration) analysis applies no load.
	case AnalysisThermomech:
		// A thermal-stress analysis applies a uniform temperature field, no mechanical load.
		m.Thermal = &ThermalLoad{DeltaK: settings.DeltaK}
	default:
		applyLoad(m, settings, groups, faces[1:])
	}
	return m
}

// applyThermalBCs sets a heat-transfer model's boundary conditions: a prescribed
// temperature on the first selected face and a surface heat flux on the rest.
func applyThermalBCs(m *AnalysisModel, settings StudySettings, groups *FaceGroups, faces []string) {
	m.Temperatures = []TemperatureBC{{Name: "TEMP", Nodes: groups.Nodes[faces[0]], TempK: settings.ColdTempK}}
	var ef []ElemFace
	for _, key := range faces[1:] {
		ef = append(ef, groups.ElemFaces[key]...)
	}
	m.HeatFluxes = []HeatFlux{{Name: "FLUX", Faces: ef, Flux: settings.HeatFluxQ}}
}

// applyElectrostaticBCs sets an electric-conduction model's boundary conditions: the
// applied potential on the first selected face and ground (0 V) on the rest. With both
// ends prescribed (a potential drop across the conductor), the steady potential field is
// the linear Laplace solution and is independent of the conductivity magnitude — the
// conductor's *CONDUCTIVITY only scales the current, not the rendered potential. Both BCs
// pin the temperature DOF (11), reusing the heat-transfer Dirichlet writer.
func applyElectrostaticBCs(m *AnalysisModel, settings StudySettings, groups *FaceGroups, faces []string) {
	var ground []int
	for _, key := range faces[1:] {
		ground = append(ground, groups.Nodes[key]...)
	}
	m.Temperatures = []TemperatureBC{
		{Name: "VHIGH", Nodes: groups.Nodes[faces[0]], TempK: settings.VoltageV},
		{Name: "VGND", Nodes: dedupeInts(ground), TempK: 0},
	}
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

// runDeck writes the deck, runs ccx, surfaces any solver error in plain language, and
// verifies a result was produced, returning the run stem (dir/study) for the caller to read
// .frd / .dat. A *ERROR in the solver output takes priority over a raw exit code, and a
// missing/empty .frd is reported with the solver's last words rather than a cryptic failure.
func runDeck(bins solverBinaries, model *AnalysisModel, dir string) (string, error) {
	stem := filepath.Join(dir, "study")
	if err := writeFile(stem+".inp", func(f *os.File) error { return WriteDeck(f, model) }); err != nil {
		return "", err
	}
	output, runErr := runCcx(bins.ccx, stem)
	if diag := scrapeCcxErrors(output); diag != "" {
		return "", fmt.Errorf("CalculiX: %s", diag)
	}
	if runErr != nil {
		return "", fmt.Errorf("CalculiX solve failed: %w\n%s", runErr, lastLines(output, 8))
	}
	if fi, err := os.Stat(stem + ".frd"); err != nil || fi.Size() == 0 {
		return "", fmt.Errorf("CalculiX produced no results; solver output:\n%s", lastLines(output, 8))
	}
	return stem, nil
}

// solveStudyDeck runs a static deck and parses its result field (used by the analytic
// oracle tests, which build a model directly and check the displacement/stress).
func solveStudyDeck(bins solverBinaries, model *AnalysisModel, dir string) (*ResultField, error) {
	stem, err := runDeck(bins, model, dir)
	if err != nil {
		return nil, err
	}
	return parseFRDFile(stem + ".frd")
}

// parseFRDFile opens and parses a .frd result file.
func parseFRDFile(path string) (*ResultField, error) {
	f, err := os.Open(path)
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
