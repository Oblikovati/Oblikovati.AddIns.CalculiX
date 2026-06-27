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

// panelControls builds the parameter controls from the current settings.
func panelControls(s StudySettings) []wire.PanelControlSpec {
	return []wire.PanelControlSpec{
		client.PanelLabel("hdr", "— Study parameters —"),
		client.PanelDropdown("analysis", "Analysis", analysisTypeOptions(), string(s.Analysis)),
		client.PanelTextBox("mesh_size", "Mesh size (mm, 0=auto)", formatNum(s.MeshSizeMM)),
		client.PanelDropdown("element_order", "Element order", elementOrderOptions(), elementOrderLabel(s.ElementOrder)),
		client.PanelTextBox("deform_scale", "Deform scale (0=auto)", formatNum(s.DeformScale)),
		client.PanelLabel("mat_hdr", "— Material & load —"),
		client.PanelTextBox("young", "Young's modulus (GPa)", formatNum(s.YoungGPa)),
		client.PanelTextBox("poisson", "Poisson's ratio", formatNum(s.Poisson)),
		client.PanelTextBox("load", "Load (N, on selected faces)", formatNum(s.LoadN)),
		client.PanelSeparator(),
		client.PanelButton("run", "Run Stress Analysis", RunStudyCommandID),
	}
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
	case "young":
		e.settings.YoungGPa = panelNum(value, e.settings.YoungGPa)
	case "poisson":
		e.settings.Poisson = panelNum(value, e.settings.Poisson)
	case "load":
		e.settings.LoadN = panelNum(value, e.settings.LoadN)
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
