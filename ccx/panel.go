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
func (e *Engine) ShowPanel() (wire.OKResult, error) {
	e.mu.Lock()
	s := e.settings
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
		loadsSection(s),
		contactSection(s),
		[]wire.PanelControlSpec{client.PanelButton("run", "Run CalculiX", RunStudyCommandID)},
	)
}

// contactSection builds the multi-body interface control group: whether touching bodies are
// bonded (the default *TIE) or in unilateral contact, and the friction coefficient used when
// contact is active.
func contactSection(s StudySettings) []wire.PanelControlSpec {
	return section("Contact",
		client.PanelDropdown("contact_mode", "Body interfaces", contactModeOptions(), contactModeLabel(s.ContactMode)),
		client.PanelTextBox("friction", "Friction coefficient μ", formatNum(s.FrictionMu)),
	)
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
		client.PanelTextBox("young", "Young's modulus (GPa)", formatNum(s.YoungGPa)),
		client.PanelTextBox("poisson", "Poisson's ratio", formatNum(s.Poisson)),
		client.PanelTextBox("yield", "Yield stress (MPa, 0=elastic)", formatNum(s.YieldMPa)),
		client.PanelTextBox("density", "Density (g/cm³)", formatNum(s.DensityGCm3)),
		client.PanelTextBox("alpha", "Thermal expansion (1/K)", formatNum(s.ThermalAlpha)),
		client.PanelTextBox("conductivity", "Thermal conductivity", formatNum(s.Conductivity)),
		client.PanelTextBox("elec_sigma", "Electrical conductivity", formatNum(s.ElectricalSigma)),
		client.PanelTextBox("specific_heat", "Specific heat (transient)", formatNum(s.SpecificHeat)),
	)
}

// loadsSection builds the loads & boundary-conditions control group.
func loadsSection(s StudySettings) []wire.PanelControlSpec {
	return section("Loads & boundary conditions",
		client.PanelDropdown("load_type", "Load type", loadTypeOptions(), string(s.LoadType)),
		client.PanelTextBox("load", "Force on loaded faces (N)", formatNum(s.LoadN)),
		client.PanelTextBox("pressure", "Pressure on loaded faces (MPa)", formatNum(s.PressureMPa)),
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
func (e *Engine) applyPanelEdit(controlID, value string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	switch controlID {
	case "analysis":
		e.settings.Analysis = AnalysisType(strings.TrimSpace(value))
	case "mesh_size":
		e.settings.MeshSizeMM = panelNum(value, e.settings.MeshSizeMM)
	case "element_order":
		e.settings.ElementOrder = parseElementOrder(value, e.settings.ElementOrder)
	case "deform_scale":
		e.settings.DeformScale = panelNum(value, e.settings.DeformScale)
	case "eigenmodes":
		e.settings.Eigenmodes = int(panelNum(value, float64(e.settings.Eigenmodes)))
	case "transient_time":
		e.settings.TransientTimeS = panelNum(value, e.settings.TransientTimeS)
	case "result_field":
		e.settings.ResultField = ResultFieldKind(strings.TrimSpace(value))
	default:
		e.applyMaterialOrLoadEdit(controlID, value)
	}
}

// applyMaterialOrLoadEdit handles the material and load control edits.
func (e *Engine) applyMaterialOrLoadEdit(controlID, value string) {
	if e.applyMaterialEdit(controlID, value) {
		return
	}
	e.applyLoadEdit(controlID, value)
}

// applyMaterialEdit handles the material-property controls, returning whether it matched.
func (e *Engine) applyMaterialEdit(controlID, value string) bool {
	switch controlID {
	case "young":
		e.settings.YoungGPa = panelNum(value, e.settings.YoungGPa)
	case "poisson":
		e.settings.Poisson = panelNum(value, e.settings.Poisson)
	case "yield":
		e.settings.YieldMPa = panelNum(value, e.settings.YieldMPa)
	case "density":
		e.settings.DensityGCm3 = panelNum(value, e.settings.DensityGCm3)
	case "alpha":
		e.settings.ThermalAlpha = panelNum(value, e.settings.ThermalAlpha)
	case "conductivity":
		e.settings.Conductivity = panelNum(value, e.settings.Conductivity)
	case "elec_sigma":
		e.settings.ElectricalSigma = panelNum(value, e.settings.ElectricalSigma)
	case "specific_heat":
		e.settings.SpecificHeat = panelNum(value, e.settings.SpecificHeat)
	default:
		return false
	}
	return true
}

// applyLoadEdit handles the mechanical-load controls, delegating the thermal/electromagnetic
// boundary-condition controls to applyFieldBCEdit.
func (e *Engine) applyLoadEdit(controlID, value string) {
	switch controlID {
	case "load_type":
		e.settings.LoadType = LoadType(strings.TrimSpace(value))
	case "load":
		e.settings.LoadN = panelNum(value, e.settings.LoadN)
	case "pressure":
		e.settings.PressureMPa = panelNum(value, e.settings.PressureMPa)
	case "gravity":
		e.settings.GravityG = panelNum(value, e.settings.GravityG)
	case "rotation":
		e.settings.RotationRadS = panelNum(value, e.settings.RotationRadS)
	case "displacement":
		e.settings.DisplacementMM = panelNum(value, e.settings.DisplacementMM)
	default:
		e.applyFieldBCEdit(controlID, value)
	}
}

// applyFieldBCEdit handles the core thermal boundary-condition controls, delegating the
// heat-drive (convection/body/radiation) parameters to applyHeatModeEdit and the
// electromagnetic controls to applyEMEdit.
func (e *Engine) applyFieldBCEdit(controlID, value string) {
	switch controlID {
	case "delta_t":
		e.settings.DeltaK = panelNum(value, e.settings.DeltaK)
	case "cold_temp":
		e.settings.ColdTempK = panelNum(value, e.settings.ColdTempK)
	case "heat_flux":
		e.settings.HeatFluxQ = panelNum(value, e.settings.HeatFluxQ)
	case "heat_drive":
		e.settings.HeatDriveMode = HeatDrive(strings.TrimSpace(value))
	default:
		e.applyHeatModeEdit(controlID, value)
	}
}

// applyHeatModeEdit handles the convection / body-source / radiation heat-drive parameters,
// delegating anything else to applyEMEdit.
func (e *Engine) applyHeatModeEdit(controlID, value string) {
	switch controlID {
	case "film_coeff":
		e.settings.FilmCoeff = panelNum(value, e.settings.FilmCoeff)
	case "sink_temp":
		e.settings.SinkTempK = panelNum(value, e.settings.SinkTempK)
	case "body_heat":
		e.settings.BodyHeatRate = panelNum(value, e.settings.BodyHeatRate)
	case "emissivity":
		e.settings.Emissivity = panelNum(value, e.settings.Emissivity)
	case "rad_ambient":
		e.settings.RadAmbientK = panelNum(value, e.settings.RadAmbientK)
	default:
		e.applyEMEdit(controlID, value)
	}
}

// applyEMEdit handles the electromagnetic boundary-condition controls.
func (e *Engine) applyEMEdit(controlID, value string) {
	switch controlID {
	case "voltage":
		e.settings.VoltageV = panelNum(value, e.settings.VoltageV)
	case "em_drive":
		e.settings.EMDriveMode = EMDrive(strings.TrimSpace(value))
	case "current_density":
		e.settings.CurrentDensity = panelNum(value, e.settings.CurrentDensity)
	case "contact_mode":
		e.settings.ContactMode = strings.TrimSpace(value) == "contact"
	case "friction":
		e.settings.FrictionMu = panelNum(value, e.settings.FrictionMu)
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
