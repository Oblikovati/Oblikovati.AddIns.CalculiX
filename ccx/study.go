// SPDX-License-Identifier: GPL-2.0-only

package ccx

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"oblikovati.org/api/wire"
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
	ReactionN        float64            // total support reaction magnitude (N, static); 0 when absent
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
		return fmt.Sprintf("CalculiX: %d elements, peak %s %.3g %s, max displacement %.3g mm%s.",
			r.ElementCount, r.FieldLabel, r.FieldPeak, r.FieldUnit, r.MaxDisplacement, r.reactionSuffix())
	}
}

// reactionSuffix appends the total support reaction to the static status line when one was read.
func (r *StudyResult) reactionSuffix() string {
	if r.ReactionN <= 0 {
		return ""
	}
	return fmt.Sprintf(", support reaction %.3g N", r.ReactionN)
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
	settings, _ := e.study()

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
	mesh, groups, model, err := e.prepareStudy(bins, settings, faces, dir)
	if err != nil {
		return nil, err
	}
	stem, err := runDeck(bins, model, dir)
	if err != nil {
		return nil, err
	}
	return e.collectResults(stem, mesh, groups, faces, model)
}

// prepareStudy resolves the active part into a solved-ready model: enumerate the solid
// bodies, mesh and merge them, bind the picked faces, resolve each body's material, and
// assemble + validate the analysis model.
func (e *Engine) prepareStudy(bins solverBinaries, settings StudySettings, faces []string, dir string) (*TetMesh, *FaceGroups, *AnalysisModel, error) {
	solids, err := e.solidBodies()
	if err != nil {
		return nil, nil, nil, err
	}
	solids, err = e.scopeBodies(settings, solids, faces)
	if err != nil {
		return nil, nil, nil, err
	}
	mesh, err := e.meshSolidBodies(bins, settings, solids, dir)
	if err != nil {
		return nil, nil, nil, err
	}
	groups, err := e.buildFaceGroups(faces, mesh, solids)
	if err != nil {
		return nil, nil, nil, err
	}
	materials, err := e.resolveBodyMaterials(settings, solids)
	if err != nil {
		return nil, nil, nil, err
	}
	model := buildModel(settings, mesh, groups, faces, materials)
	if err := checkPrerequisites(model); err != nil {
		return nil, nil, nil, err
	}
	return mesh, groups, model, nil
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
	case AnalysisCoupledThermal:
		return 2 // a cold support face and a hot face (the gradient)
	default:
		return minFaces(s.LoadType)
	}
}

// resolveBodyMaterials returns the CalculiX material for each solid body (index-aligned to
// solids): the host's assigned material, or the panel material when a body has none.
func (e *Engine) resolveBodyMaterials(settings StudySettings, solids []wire.BodyInfo) ([]MaterialProps, error) {
	fallback := settings.material()
	out := make([]MaterialProps, len(solids))
	for i, b := range solids {
		mat, err := e.bodyMaterial(b, fallback)
		if err != nil {
			return nil, err
		}
		out[i] = mat
	}
	return out, nil
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
		ReactionN:        readReaction(stem + ".dat"),
		GraphicsClientID: resultClientID,
	}, nil
}

// readReaction returns the total support-reaction magnitude from the .dat file, or 0 when the
// deck requested none (e.g. an elastic-support study) or the block is absent — the reaction is a
// supplementary report, never a hard failure of an otherwise-solved study.
func readReaction(datPath string) float64 {
	f, err := os.Open(datPath)
	if err != nil {
		return 0
	}
	defer f.Close()
	v, err := parseTotalReaction(f)
	if err != nil {
		return 0
	}
	return reactionMagnitude(v)
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

// buildModel assembles the analysis model from the settings, mesh, per-body materials, and
// face bindings. A mechanical/thermal-stress study fixes the first face and loads the rest; a
// heat-transfer study instead prescribes a temperature on the first face and a heat flux on
// the rest. Each body's elements form a material section, so a part of mixed materials writes
// one *MATERIAL + *SOLID SECTION per material.
func buildModel(settings StudySettings, mesh *TetMesh, groups *FaceGroups, faces []string, materials []MaterialProps) *AnalysisModel {
	m := &AnalysisModel{
		Analysis:       settings.Analysis,
		Mesh:           mesh,
		Material:       materials[0],
		Sections:       buildSections(mesh, materials),
		EigenmodeCount: settings.eigenmodeCount(),
		ResultField:    settings.ResultField,
	}
	applyInterfaces(m, settings, mesh)
	if applyFieldAnalysisBCs(m, settings, groups, faces) {
		return m
	}
	// The mechanical (static / modal / thermal-stress) path resolves a list of constraint specs:
	// the user's explicit constraints, or — when none were added — the synthesized default that
	// reproduces the implicit first-face-support / remaining-faces-load convention.
	specs := settings.Constraints
	if len(specs) == 0 {
		specs = defaultConstraints(settings, faces)
	}
	resolveSpecs(specs, &ResolveContext{Model: m, Mesh: mesh, Groups: groups})
	return m
}

// applyFieldAnalysisBCs applies the boundary conditions for the DOF-11 field analyses
// (heat transfer, electrostatic, coupled thermomechanical), returning whether it handled the
// analysis. The mechanical static / modal / thermal-stress analyses are handled by the caller.
func applyFieldAnalysisBCs(m *AnalysisModel, settings StudySettings, groups *FaceGroups, faces []string) bool {
	switch settings.Analysis {
	case AnalysisHeatTransfer:
		applyThermalBCs(m, settings, groups, faces)
	case AnalysisElectromagnetic:
		applyElectrostaticBCs(m, settings, groups, faces)
	case AnalysisCoupledThermal:
		applyCoupledThermal(m, settings, groups, faces)
	default:
		return false
	}
	return true
}

// applyInterfaces binds the touching body interfaces as either unilateral contact (when the
// study requests contact, with its friction coefficient and a penalty stiffness scaled off the
// stiffest material) or bonded *TIE (the default). A single-body study yields neither.
func applyInterfaces(m *AnalysisModel, settings StudySettings, mesh *TetMesh) {
	if !settings.ContactMode {
		m.Ties = detectTies(mesh)
		return
	}
	m.Contacts = detectContacts(mesh, settings.FrictionMu, contactStiffnessFactor*m.maxYoungMPa())
}

// buildSections groups the merged mesh's elements by their source body into per-body
// material sections (ELSET "Eb0", "Eb1", …), so each body is assigned its own material. A
// single-body part yields one section; bodies sharing a material are deduplicated into one
// *MATERIAL at deck-write time.
func buildSections(mesh *TetMesh, materials []MaterialProps) []MaterialSection {
	byBody := map[int][]int{}
	for _, el := range mesh.Elements {
		byBody[el.Body] = append(byBody[el.Body], el.ID)
	}
	var sections []MaterialSection
	for body := 0; body < len(materials); body++ {
		ids := byBody[body]
		if len(ids) == 0 {
			continue
		}
		sections = append(sections, MaterialSection{
			ElsetName:  fmt.Sprintf("Eb%d", body),
			Material:   materials[body],
			ElementIDs: ids,
		})
	}
	return sections
}

// applyCoupledThermal sets a coupled temperature-displacement model: the first selected face
// is the mechanical support and is held at the cold/reference temperature; the remaining faces
// are held hotter (cold + ΔT). The temperature field between them is solved by conduction, and
// its thermal expansion (relative to the cold reference) drives the displacement/stress in the
// same step. A non-zero transient time makes it time-dependent.
func applyCoupledThermal(m *AnalysisModel, settings StudySettings, groups *FaceGroups, faces []string) {
	m.Fixed = []FixedConstraint{{Name: "FIX", Nodes: groups.Nodes[faces[0]], DOFLow: 1, DOFHigh: 3}}
	var hot []int
	for _, key := range faces[1:] {
		hot = append(hot, groups.Nodes[key]...)
	}
	m.Temperatures = []TemperatureBC{
		{Name: "TCOLD", Nodes: groups.Nodes[faces[0]], TempK: settings.ColdTempK},
		{Name: "THOT", Nodes: dedupeInts(hot), TempK: settings.ColdTempK + settings.DeltaK},
	}
	m.InitialTempK = settings.ColdTempK
	if settings.TransientTimeS > 0 {
		m.Transient = &TransientStep{IncrementS: settings.TransientTimeS / 10, TotalS: settings.TransientTimeS}
	}
}

// applyThermalBCs sets a heat-transfer model's boundary conditions: a prescribed temperature
// on the first selected face, and on the rest either a surface heat flux (flux drive) or a
// convective film exchange with the ambient (convection drive).
func applyThermalBCs(m *AnalysisModel, settings StudySettings, groups *FaceGroups, faces []string) {
	m.Temperatures = []TemperatureBC{{Name: "TEMP", Nodes: groups.Nodes[faces[0]], TempK: settings.ColdTempK}}
	var ef []ElemFace
	for _, key := range faces[1:] {
		ef = append(ef, groups.ElemFaces[key]...)
	}
	switch settings.HeatDriveMode {
	case HeatDriveFilm:
		m.Films = []FilmBC{{Name: "FILM", Faces: ef, Coeff: settings.FilmCoeff, SinkTempK: settings.SinkTempK}}
	case HeatDriveBody:
		// Internal generation with the remaining faces also held at the prescribed temperature,
		// so heat flows out through both ends (the classic generating-slab problem).
		m.Temperatures = append(m.Temperatures, TemperatureBC{Name: "TEMP2", Nodes: dedupeInts(faceNodes(groups, faces[1:])), TempK: settings.ColdTempK})
		m.BodyHeat = &BodyHeat{Rate: settings.BodyHeatRate}
	case HeatDriveRadiation:
		// The prescribed (hot) face drives heat that the remaining faces radiate to ambient;
		// seed the nonlinear T⁴ solve with the prescribed temperature.
		m.Radiations = []RadiationBC{{Name: "RAD", Faces: ef, Emissivity: settings.Emissivity, AmbientK: settings.RadAmbientK}}
		m.InitialTempK = settings.ColdTempK
	default:
		m.HeatFluxes = []HeatFlux{{Name: "FLUX", Faces: ef, Flux: settings.HeatFluxQ}}
	}
}

// faceNodes gathers the node ids of a set of bound faces.
func faceNodes(groups *FaceGroups, keys []string) []int {
	var nodes []int
	for _, key := range keys {
		nodes = append(nodes, groups.Nodes[key]...)
	}
	return nodes
}

// applyElectrostaticBCs sets an electric-conduction model's boundary conditions, by drive mode:
// applied voltage (a prescribed potential on both ends) or injected current (a current density
// on the loaded faces, grounded on the first). Both pin/feed the temperature DOF (11), reusing
// the heat-transfer Dirichlet and flux writers.
func applyElectrostaticBCs(m *AnalysisModel, settings StudySettings, groups *FaceGroups, faces []string) {
	if settings.EMDriveMode == EMCurrent {
		applyCurrentDrive(m, settings, groups, faces)
		return
	}
	applyVoltageDrive(m, settings, groups, faces)
}

// applyVoltageDrive prescribes the applied potential on the first face and ground (0 V) on the
// rest. With both ends prescribed (a potential drop across the conductor), the steady field is
// the linear Laplace solution, independent of the conductivity magnitude.
func applyVoltageDrive(m *AnalysisModel, settings StudySettings, groups *FaceGroups, faces []string) {
	var ground []int
	for _, key := range faces[1:] {
		ground = append(ground, groups.Nodes[key]...)
	}
	m.Temperatures = []TemperatureBC{
		{Name: "VHIGH", Nodes: groups.Nodes[faces[0]], TempK: settings.VoltageV},
		{Name: "VGND", Nodes: dedupeInts(ground), TempK: 0},
	}
}

// applyCurrentDrive grounds the first face (0 V) and injects a uniform current density on the
// loaded faces (*DFLUX). The current flows through the conductor to ground, so the potential
// scales with the resistance (1/conductivity) — the conductivity now drives the result.
func applyCurrentDrive(m *AnalysisModel, settings StudySettings, groups *FaceGroups, faces []string) {
	m.Temperatures = []TemperatureBC{{Name: "VGND", Nodes: groups.Nodes[faces[0]], TempK: 0}}
	var ef []ElemFace
	for _, key := range faces[1:] {
		ef = append(ef, groups.ElemFaces[key]...)
	}
	m.HeatFluxes = []HeatFlux{{Name: "ICURR", Faces: ef, Flux: settings.CurrentDensity}}
}

// minFaces is the number of selected faces a load type needs: a body load (gravity,
// centrifugal) needs only the support; force/pressure need the support plus loaded faces.
func minFaces(load LoadType) int {
	if isBodyLoad(load) {
		return 1
	}
	return 2
}

// isBodyLoad reports whether a load acts over the whole body (no loaded face needed).
func isBodyLoad(load LoadType) bool {
	return load == LoadGravity || load == LoadCentrifugal
}

// loadHint describes the remaining-face requirement for the selection error message.
func loadHint(load LoadType) string {
	if isBodyLoad(load) {
		return " (the load acts over the whole body)"
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
