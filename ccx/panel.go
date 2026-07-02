// SPDX-License-Identifier: GPL-2.0-only

package ccx

import (
	"strconv"
	"strings"

	"oblikovati.org/api/client"
	"oblikovati.org/api/types"
	"oblikovati.org/api/wire"
)

// PanelID is the stable dockable-window id the CalculiX add-in owns.
const PanelID = "com.oblikovati.calculix.panel"

// ShowPanel creates (or replaces) the CalculiX study-parameters dockable window: the editable
// study settings plus a Run button. Edits arrive as panel.valueChanged events (applyPanelEdit).
// builderKind is engine state (not part of the aggregate projection) so it is overlaid onto s
// under the lock after study() returns, keeping panelControls a pure function of StudySettings.
func (e *Engine) ShowPanel() (wire.OKResult, error) {
	s, _ := e.study()
	e.mu.Lock()
	s.BuilderKind = e.builderKind
	e.mu.Unlock()
	return e.api.DockableWindows().Set(wire.DockableWindowSpec{
		ID:       PanelID,
		Title:    "CalculiX FEA",
		Dock:     types.DockRight,
		Visible:  true,
		Controls: panelControls(s),
	})
}

// panelControls builds the parameter controls, grouped into titled sections that mirror
// the layout of the FreeCAD CalculiX solver task panel (Solver Parameters / Mesh /
// Material / Loads & boundary conditions, then a "Run CalculiX" button). The host's
// control vocabulary has no real group box, so each section is a heading label followed
// by its controls and a separator.
func panelControls(s StudySettings) []wire.PanelControlSpec {
	return joinControls(
		header("Solver CalculiX Control", "Select the fixed face first, then the loaded face(s)."),
		section("Solver Parameters",
			client.PanelDropdown("analysis", "Analysis type", analysisTypeOptions(), string(s.Analysis)),
			client.PanelTextBox("eigenmodes", "Modes (frequency/buckling)", formatNum(float64(s.Eigenmodes))),
			client.PanelTextBox("transient_time", "Transient total time (s, 0=steady)", formatNum(s.TransientTimeS)),
		),
		section("Mesh",
			client.PanelTextBox("mesh_size", "Max element size (mm, 0=auto)", formatNum(s.MeshSizeMM)),
			client.PanelDropdown("element_order", "Element order", elementOrderOptions(), elementOrderLabel(s.ElementOrder)),
		),
		section("Result",
			client.PanelDropdown("result_field", "Result field", resultFieldOptions(), string(s.ResultField)),
		),
		materialSection(s),
		supportSection(s),
		loadsSection(s),
		contactSection(s),
		constraintSection(s),
		[]wire.PanelControlSpec{client.PanelButton("run", "Run CalculiX", RunStudyCommandID)},
	)
}

// constraintSection builds the constraint-builder group: pick a constraint type, then Add From
// Selection snapshots the picked faces + the relevant parameter fields into the study's explicit
// constraint list (which then replaces the implicit one-support/one-load default). Clear empties
// it. With no list widget in the host vocabulary, the list is add/clear-only — the count label is
// the feedback that a constraint landed.
func constraintSection(s StudySettings) []wire.PanelControlSpec {
	return section("Constraints (multi-constraint builder)",
		client.PanelDropdown("constraint_type", "Constraint type", constraintKindOptions(), string(builderKindOrDefault(s.BuilderKind))),
		client.PanelButton("add_constraint", "Add from selection", AddConstraintCommandID),
		client.PanelLabel("constraint_count", "Constraints added: "+strconv.Itoa(len(s.Constraints))),
		client.PanelButton("clear_constraints", "Clear constraints", ClearConstraintsCommandID),
	)
}

// constraintKindOptions lists the builder's constraint-type choices in display order.
func constraintKindOptions() []string {
	kinds := builderKinds()
	out := make([]string, len(kinds))
	for i, k := range kinds {
		out[i] = string(k)
	}
	return out
}

// contactSection builds the multi-body interface control group: whether touching bodies are
// bonded (the default *TIE) or in unilateral contact, and the friction coefficient used when
// contact is active.
func contactSection(s StudySettings) []wire.PanelControlSpec {
	return section("Contact",
		client.PanelDropdown("contact_mode", "Body interfaces", contactModeOptions(), contactModeLabel(s.ContactMode)),
		client.PanelTextBox("friction", "Friction coefficient μ", formatNum(s.FrictionMu)),
		client.PanelDropdown("body_scope", "Analyse", bodyScopeOptions(), string(bodyScopeOrDefault(s.BodyScope))),
	)
}

// bodyScopeOrDefault treats the zero value as the default (all solid bodies), so an unset
// setting renders as the unchanged whole-part scope.
func bodyScopeOrDefault(scope BodyScope) BodyScope {
	if scope == "" {
		return BodyScopeAll
	}
	return scope
}

// contactModeOptions / contactModeLabel map the bonded-vs-contact toggle to dropdown labels.
func contactModeOptions() []string { return []string{"bonded", "contact"} }

func contactModeLabel(contact bool) string {
	if contact {
		return "contact"
	}
	return "bonded"
}

// materialSection builds the Material control group (the FreeCAD "Material" task box).
func materialSection(s StudySettings) []wire.PanelControlSpec {
	return section("Material",
		client.PanelDropdown("material_model", "Material model", materialModelOptions(), string(materialModelOrDefault(s.MaterialModel))),
		client.PanelTextBox("neo_c10", "Neo-Hookean C10 (MPa, rubber)", formatNum(s.NeoHookeC10)),
		client.PanelTextBox("neo_d1", "Neo-Hookean D1 (1/MPa, rubber)", formatNum(s.NeoHookeD1)),
		client.PanelTextBox("young", "Young's modulus (GPa)", formatNum(s.YoungGPa)),
		client.PanelTextBox("young_hot", "Young's modulus at hot temp (GPa, 0=const)", formatNum(s.YoungHotGPa)),
		client.PanelTextBox("hot_temp", "Hot temperature (K) for E(T)", formatNum(s.HotTempK)),
		client.PanelTextBox("poisson", "Poisson's ratio", formatNum(s.Poisson)),
		client.PanelTextBox("yield", "Yield stress (MPa, 0=elastic)", formatNum(s.YieldMPa)),
		client.PanelTextBox("density", "Density (g/cm³)", formatNum(s.DensityGCm3)),
		client.PanelTextBox("alpha", "Thermal expansion (1/K)", formatNum(s.ThermalAlpha)),
		client.PanelTextBox("conductivity", "Thermal conductivity", formatNum(s.Conductivity)),
		client.PanelTextBox("elec_sigma", "Electrical conductivity", formatNum(s.ElectricalSigma)),
		client.PanelTextBox("specific_heat", "Specific heat (transient)", formatNum(s.SpecificHeat)),
	)
}

// materialModelOrDefault treats the zero value as the default (linear elastic).
func materialModelOrDefault(m MaterialModel) MaterialModel {
	if m == "" {
		return MaterialLinear
	}
	return m
}

// supportSection builds the support control group: whether the first selected face is rigidly
// clamped or rests on an elastic spring foundation, and the foundation stiffness.
func supportSection(s StudySettings) []wire.PanelControlSpec {
	return section("Support",
		client.PanelDropdown("support_type", "Support face", supportTypeOptions(), string(supportTypeOrDefault(s.SupportType))),
		client.PanelTextBox("spring_stiffness", "Spring stiffness (N/mm, elastic)", formatNum(s.SpringStiffMM)),
	)
}

// supportTypeOrDefault treats the zero value as the default (fixed), so an unset setting renders
// as the unchanged rigid clamp.
func supportTypeOrDefault(t SupportType) SupportType {
	if t == "" {
		return SupportFixed
	}
	return t
}

// loadsSection builds the loads & boundary-conditions control group.
func loadsSection(s StudySettings) []wire.PanelControlSpec {
	return section("Loads & boundary conditions",
		client.PanelDropdown("load_type", "Load type", loadTypeOptions(), string(s.LoadType)),
		client.PanelTextBox("load", "Force on loaded faces (N)", formatNum(s.LoadN)),
		client.PanelTextBox("pressure", "Pressure on loaded faces (MPa)", formatNum(s.PressureMPa)),
		client.PanelTextBox("hydro_gradient", "Hydrostatic gradient ρg (MPa/mm)", formatNum(s.HydroGradientMPaMM)),
		client.PanelTextBox("hydro_surface", "Fluid surface height z (mm)", formatNum(s.HydroSurfaceZ)),
		client.PanelTextBox("gravity", "Gravity (× g)", formatNum(s.GravityG)),
		client.PanelTextBox("rotation", "Rotation about Z (rad/s)", formatNum(s.RotationRadS)),
		client.PanelTextBox("displacement", "Enforced displacement (mm, +Z)", formatNum(s.DisplacementMM)),
		client.PanelTextBox("delta_t", "Temperature change ΔT (K)", formatNum(s.DeltaK)),
		client.PanelTextBox("cold_temp", "Prescribed temperature (K)", formatNum(s.ColdTempK)),
		client.PanelDropdown("heat_drive", "Heat drive", heatDriveOptions(), string(s.HeatDriveMode)),
		client.PanelTextBox("heat_flux", "Heat flux on loaded faces", formatNum(s.HeatFluxQ)),
		client.PanelTextBox("film_coeff", "Film coefficient (convection)", formatNum(s.FilmCoeff)),
		client.PanelTextBox("sink_temp", "Ambient/sink temperature (K)", formatNum(s.SinkTempK)),
		client.PanelTextBox("body_heat", "Body heat generation (volumetric)", formatNum(s.BodyHeatRate)),
		client.PanelTextBox("emissivity", "Emissivity (radiation)", formatNum(s.Emissivity)),
		client.PanelTextBox("rad_ambient", "Radiation ambient temp (K)", formatNum(s.RadAmbientK)),
		client.PanelDropdown("em_drive", "EM drive", emDriveOptions(), string(s.EMDriveMode)),
		client.PanelTextBox("voltage", "Applied voltage on first face (V)", formatNum(s.VoltageV)),
		client.PanelTextBox("current_density", "Injected current density", formatNum(s.CurrentDensity)),
		client.PanelTextBox("deform_scale", "Deformation scale (0=auto)", formatNum(s.DeformScale)),
	)
}

// header builds the panel's title + a one-line usage hint.
func header(title, hint string) []wire.PanelControlSpec {
	return []wire.PanelControlSpec{
		client.PanelLabel("hdr", title),
		client.PanelLabel("hint", hint),
		client.PanelSeparator(),
	}
}

// section builds a titled control group: a heading label, the controls, and a trailing
// separator (the dockable-window analog of a FreeCAD QGroupBox).
func section(title string, controls ...wire.PanelControlSpec) []wire.PanelControlSpec {
	out := []wire.PanelControlSpec{client.PanelLabel(labelID(title), title)}
	out = append(out, controls...)
	return append(out, client.PanelSeparator())
}

// joinControls flattens the section groups into one control list.
func joinControls(groups ...[]wire.PanelControlSpec) []wire.PanelControlSpec {
	var out []wire.PanelControlSpec
	for _, g := range groups {
		out = append(out, g...)
	}
	return out
}

// labelID derives a stable control id for a section heading from its title.
func labelID(title string) string {
	return "sec_" + strings.ToLower(strings.ReplaceAll(strings.Fields(title)[0], "&", "and"))
}

// applyPanelEdit writes one edited study parameter back into the engine, keyed by control id.
// The 11 tree-owned controls (solver/mesh/material/result) reach the femmodel aggregate via their
// per-object helpers; everything else writes to e.extras.
func (e *Engine) applyPanelEdit(controlID, value string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.applySolverEdit(controlID, value) {
		return
	}
	if e.applyMeshAggEdit(controlID, value) {
		return
	}
	if e.applyResultAggEdit(controlID, value) {
		return
	}
	e.applyMaterialOrLoadEdit(controlID, value)
}

// applySolverEdit routes analysis/eigenmodes/transient_time to the Analysis.Solver aggregate.
func (e *Engine) applySolverEdit(controlID, value string) bool {
	sv := e.analysis.Solver()
	switch controlID {
	case "analysis":
		sv.AnalysisType = strings.TrimSpace(value)
	case "eigenmodes":
		sv.Eigenmodes = int(panelNum(value, float64(sv.Eigenmodes)))
	case "transient_time":
		sv.TransientTimeS = panelNum(value, sv.TransientTimeS)
	default:
		return false
	}
	e.analysis.SetSolver(sv)
	return true
}

// applyMeshAggEdit routes mesh_size/element_order to the Analysis.Mesh aggregate.
func (e *Engine) applyMeshAggEdit(controlID, value string) bool {
	m := e.analysis.Mesh()
	switch controlID {
	case "mesh_size":
		m.MaxSizeMM = panelNum(value, m.MaxSizeMM)
	case "element_order":
		m.Quadratic = parseElementOrder(value, elementOrder(m.Quadratic)) == QuadraticTet
	default:
		return false
	}
	e.analysis.SetMesh(m)
	return true
}

// applyResultAggEdit routes result_field/deform_scale to the Analysis.PrimaryResult aggregate.
func (e *Engine) applyResultAggEdit(controlID, value string) bool {
	r, ok := e.analysis.PrimaryResult()
	if !ok {
		return false
	}
	switch controlID {
	case "result_field":
		r.Field = strings.TrimSpace(value)
	case "deform_scale":
		r.DeformScale = panelNum(value, r.DeformScale)
	default:
		return false
	}
	e.analysis.SetPrimaryResult(r)
	return true
}

// applyMaterialOrLoadEdit handles the material, load, and constraint-builder control edits.
func (e *Engine) applyMaterialOrLoadEdit(controlID, value string) {
	if controlID == "constraint_type" {
		e.builderKind = ConstraintKind(strings.TrimSpace(value))
		return
	}
	if e.applyMaterialEdit(controlID, value) {
		return
	}
	if e.applyAggLoadEdit(controlID, value) {
		return
	}
	e.applyLoadEdit(controlID, value)
}

// applyMaterialEdit handles the material-property controls, returning whether it matched.
// All material controls update the Analysis aggregate: mechanical fields via
// applyAggMechanicalMatEdit, thermal via applyAggThermalMatEdit, and electrical /
// hyperelastic / temperature-dependent via applyAggEMHyperMatEdit.
func (e *Engine) applyMaterialEdit(controlID, value string) bool {
	if e.applyAggMechanicalMatEdit(controlID, value) {
		return true
	}
	if e.applyAggThermalMatEdit(controlID, value) {
		return true
	}
	return e.applyAggEMHyperMatEdit(controlID, value)
}

// applyAggMechanicalMatEdit routes the 4 core mechanical material fields to the Analysis aggregate.
func (e *Engine) applyAggMechanicalMatEdit(controlID, value string) bool {
	mat, ok := e.analysis.DefaultMaterial()
	if !ok {
		return false
	}
	switch controlID {
	case "young":
		mat.YoungGPa = panelNum(value, mat.YoungGPa)
	case "poisson":
		mat.Poisson = panelNum(value, mat.Poisson)
	case "yield":
		mat.YieldMPa = panelNum(value, mat.YieldMPa)
	case "density":
		mat.DensityGCm3 = panelNum(value, mat.DensityGCm3)
	default:
		return false
	}
	e.analysis.SetDefaultMaterial(mat)
	return true
}

// applyAggThermalMatEdit routes the 3 thermal material fields to the Analysis aggregate.
func (e *Engine) applyAggThermalMatEdit(controlID, value string) bool {
	mat, ok := e.analysis.DefaultMaterial()
	if !ok {
		return false
	}
	switch controlID {
	case "alpha":
		mat.ThermalAlpha = panelNum(value, mat.ThermalAlpha)
	case "conductivity":
		mat.Conductivity = panelNum(value, mat.Conductivity)
	case "specific_heat":
		mat.SpecificHeat = panelNum(value, mat.SpecificHeat)
	default:
		return false
	}
	e.analysis.SetDefaultMaterial(mat)
	return true
}

// applyAggEMHyperMatEdit routes the electrical, hyperelastic, and temperature-dependent-elasticity
// material controls to the Analysis aggregate. Returns whether it matched.
func (e *Engine) applyAggEMHyperMatEdit(controlID, value string) bool {
	mat, ok := e.analysis.DefaultMaterial()
	if !ok {
		return false
	}
	switch controlID {
	case "elec_sigma":
		mat.ElectricalSigma = panelNum(value, mat.ElectricalSigma)
	case "material_model":
		mat.MaterialModel = strings.TrimSpace(value)
	case "neo_c10":
		mat.NeoHookeC10 = panelNum(value, mat.NeoHookeC10)
	case "neo_d1":
		mat.NeoHookeD1 = panelNum(value, mat.NeoHookeD1)
	case "young_hot":
		mat.YoungHotGPa = panelNum(value, mat.YoungHotGPa)
	case "hot_temp":
		mat.HotTempK = panelNum(value, mat.HotTempK)
	default:
		return false
	}
	e.analysis.SetDefaultMaterial(mat)
	return true
}

// applyAggLoadEdit routes the load-type and hydrostatic controls to the Analysis load
// template. The 5 numeric magnitude controls are delegated to applyAggLoadScalarEdit.
// Returns whether the control was recognised (false lets the caller fall through to extras).
func (e *Engine) applyAggLoadEdit(controlID, value string) bool {
	if e.applyAggLoadScalarEdit(controlID, value) {
		return true
	}
	ld := e.analysis.Load()
	switch controlID {
	case "load_type":
		ld.LoadType = strings.TrimSpace(value)
	case "hydro_gradient":
		ld.HydroGradientMPaMM = panelNum(value, ld.HydroGradientMPaMM)
	case "hydro_surface":
		ld.HydroSurfaceZ = panelNum(value, ld.HydroSurfaceZ)
	default:
		return false
	}
	e.analysis.SetLoad(ld)
	return true
}

// applyAggLoadScalarEdit routes the 5 numeric load-magnitude controls (force, pressure,
// gravity, rotation, displacement) to the Analysis load template. Returns whether it matched.
func (e *Engine) applyAggLoadScalarEdit(controlID, value string) bool {
	ld := e.analysis.Load()
	switch controlID {
	case "load":
		ld.LoadN = panelNum(value, ld.LoadN)
	case "pressure":
		ld.PressureMPa = panelNum(value, ld.PressureMPa)
	case "gravity":
		ld.GravityG = panelNum(value, ld.GravityG)
	case "rotation":
		ld.RotationRadS = panelNum(value, ld.RotationRadS)
	case "displacement":
		ld.DisplacementMM = panelNum(value, ld.DisplacementMM)
	default:
		return false
	}
	e.analysis.SetLoad(ld)
	return true
}

// applyLoadEdit handles the thermal/electromagnetic boundary-condition controls (the
// mechanical-load controls have moved to applyAggLoadEdit; support controls stay in
// applySupportEdit).
func (e *Engine) applyLoadEdit(controlID, value string) {
	if e.applySupportEdit(controlID, value) {
		return
	}
	e.applyFieldBCEdit(controlID, value)
}

// applySupportEdit handles the support controls (clamp vs elastic spring), returning whether
// it matched. The hydrostatic parameters have moved to applyAggLoadEdit.
func (e *Engine) applySupportEdit(controlID, value string) bool {
	switch controlID {
	case "support_type":
		e.extras.SupportType = SupportType(strings.TrimSpace(value))
	case "spring_stiffness":
		e.extras.SpringStiffMM = panelNum(value, e.extras.SpringStiffMM)
	default:
		return false
	}
	return true
}

// applyFieldBCEdit handles the core thermal boundary-condition controls, delegating the
// heat-drive (convection/body/radiation) parameters to applyHeatModeEdit and the
// electromagnetic controls to applyEMEdit.
func (e *Engine) applyFieldBCEdit(controlID, value string) {
	switch controlID {
	case "delta_t":
		e.extras.DeltaK = panelNum(value, e.extras.DeltaK)
	case "cold_temp":
		e.extras.ColdTempK = panelNum(value, e.extras.ColdTempK)
	case "heat_flux":
		e.extras.HeatFluxQ = panelNum(value, e.extras.HeatFluxQ)
	case "heat_drive":
		e.extras.HeatDriveMode = HeatDrive(strings.TrimSpace(value))
	default:
		e.applyHeatModeEdit(controlID, value)
	}
}

// applyHeatModeEdit handles the convection / body-source / radiation heat-drive parameters,
// delegating anything else to applyEMEdit.
func (e *Engine) applyHeatModeEdit(controlID, value string) {
	switch controlID {
	case "film_coeff":
		e.extras.FilmCoeff = panelNum(value, e.extras.FilmCoeff)
	case "sink_temp":
		e.extras.SinkTempK = panelNum(value, e.extras.SinkTempK)
	case "body_heat":
		e.extras.BodyHeatRate = panelNum(value, e.extras.BodyHeatRate)
	case "emissivity":
		e.extras.Emissivity = panelNum(value, e.extras.Emissivity)
	case "rad_ambient":
		e.extras.RadAmbientK = panelNum(value, e.extras.RadAmbientK)
	default:
		if e.applyAggStudySwitchEdit(controlID, value) {
			return
		}
		e.applyEMEdit(controlID, value)
	}
}

// applyAggStudySwitchEdit routes the study-wide switches (body scope, contact mode, friction) to the
// Analysis aggregate's SolverObject. Returns whether it matched.
func (e *Engine) applyAggStudySwitchEdit(controlID, value string) bool {
	sv := e.analysis.Solver()
	switch controlID {
	case "body_scope":
		sv.BodyScope = strings.TrimSpace(value)
	case "contact_mode":
		sv.ContactMode = strings.TrimSpace(value) == "contact"
	case "friction":
		sv.FrictionMu = panelNum(value, sv.FrictionMu)
	default:
		return false
	}
	e.analysis.SetSolver(sv)
	return true
}

// applyEMEdit handles the electromagnetic boundary-condition controls.
func (e *Engine) applyEMEdit(controlID, value string) {
	switch controlID {
	case "voltage":
		e.extras.VoltageV = panelNum(value, e.extras.VoltageV)
	case "em_drive":
		e.extras.EMDriveMode = EMDrive(strings.TrimSpace(value))
	case "current_density":
		e.extras.CurrentDensity = panelNum(value, e.extras.CurrentDensity)
	}
}

// elementOrderOptions / elementOrderLabel / parseElementOrder map the order enum to the
// human-readable dropdown labels.
func elementOrderOptions() []string { return []string{"linear (C3D4)", "quadratic (C3D10)"} }

func elementOrderLabel(o ElementOrder) string {
	if o == LinearTet {
		return "linear (C3D4)"
	}
	return "quadratic (C3D10)"
}

func parseElementOrder(value string, fallback ElementOrder) ElementOrder {
	switch {
	case strings.HasPrefix(value, "linear"):
		return LinearTet
	case strings.HasPrefix(value, "quadratic"):
		return QuadraticTet
	default:
		return fallback
	}
}

// formatNum renders a parameter value compactly (no trailing zeros) for the panel.
func formatNum(v float64) string { return strconv.FormatFloat(v, 'g', -1, 64) }

// panelNum reads the leading number from a form value (e.g. "5 mm" → 5), keeping the
// fallback when the field is empty or half-typed.
func panelNum(value string, fallback float64) float64 {
	fields := strings.Fields(value)
	if len(fields) == 0 {
		return fallback
	}
	v, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return fallback
	}
	return v
}
