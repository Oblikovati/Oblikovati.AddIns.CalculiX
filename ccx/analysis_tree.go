// SPDX-License-Identifier: GPL-2.0-only

package ccx

import (
	"fmt"

	"oblikovati.org/api/wire"
	"oblikovati.org/calculix/ccx/femmodel"
)

// AnalysisBrowserPaneID is the add-in's browser pane holding the FEM Analysis tree.
const AnalysisBrowserPaneID = "com.oblikovati.calculix.tree"

// ShowAnalysisTree (re)declares the Analysis browser pane from the current aggregate — the
// read-only FreeCAD-FEM-style tree (Analysis ▸ Solver/Mesh/Materials/Constraints/Results). It
// snapshots the model under lock, then makes the host call OUTSIDE the lock.
func (e *Engine) ShowAnalysisTree() (wire.OKResult, error) {
	e.mu.Lock()
	nodes := analysisNodes(e.analysis)
	e.mu.Unlock()
	return e.api.Browser().SetPane(wire.BrowserPaneSpec{
		ID: AnalysisBrowserPaneID, Title: "Analysis", Nodes: nodes,
	})
}

// analysisNodes projects the aggregate into the tree — pure and directly testable. Example:
//
//	nodes := analysisNodes(femmodel.NewDefaultAnalysis())
//	// nodes[0].ID == "analysis"
func analysisNodes(a *femmodel.Analysis) []wire.BrowserNodeSpec {
	return []wire.BrowserNodeSpec{{
		ID: "analysis", Label: "Analysis", IconSVG: iconSVG("analysis"), Expanded: true, Menu: analysisRootMenu(),
		Children: []wire.BrowserNodeSpec{
			{ID: "solver", Label: "Solver: " + a.Solver().AnalysisType, IconSVG: iconSVG("solver"), Menu: editMenu()},
			{ID: "mesh", Label: "Mesh", IconSVG: iconSVG("mesh"), Menu: editMenu()},
			materialsNode(a.Materials()),
			constraintsNode(len(a.Constraints())),
			resultsNode(a.Results()),
		},
	}}
}

func materialsNode(mats []femmodel.MaterialObject) wire.BrowserNodeSpec {
	glyph := iconSVG("material")
	kids := make([]wire.BrowserNodeSpec, len(mats))
	for i, m := range mats {
		kids[i] = wire.BrowserNodeSpec{ID: fmt.Sprintf("mat:%d", i), Label: m.Name(), IconSVG: glyph, Menu: editMenu()}
	}
	return wire.BrowserNodeSpec{ID: "materials", Label: "Materials", IconSVG: glyph, Expanded: true, Children: kids}
}

func constraintsNode(n int) wire.BrowserNodeSpec {
	glyph := iconSVG("constraint")
	kids := make([]wire.BrowserNodeSpec, n)
	for i := range kids {
		kids[i] = wire.BrowserNodeSpec{ID: fmt.Sprintf("con:%d", i), Label: fmt.Sprintf("Constraint %d", i+1), IconSVG: glyph}
	}
	return wire.BrowserNodeSpec{
		ID: "constraints", Label: "Constraints & Loads", IconSVG: glyph, Expanded: true,
		Menu: constraintsMenu(), Children: kids,
	}
}

func resultsNode(results []femmodel.ResultObject) wire.BrowserNodeSpec {
	glyph := iconSVG("result")
	kids := make([]wire.BrowserNodeSpec, len(results))
	for i, r := range results {
		kids[i] = wire.BrowserNodeSpec{ID: fmt.Sprintf("result:%d", i), Label: r.Field, IconSVG: glyph, Menu: editMenu()}
	}
	return wire.BrowserNodeSpec{ID: "results", Label: "Results", IconSVG: glyph, Expanded: true, Children: kids}
}

func analysisRootMenu() []wire.BrowserMenuItem {
	return []wire.BrowserMenuItem{{ID: "run", Label: "Run Study"}}
}

func constraintsMenu() []wire.BrowserMenuItem {
	return []wire.BrowserMenuItem{
		{ID: "add", Label: "Add From Selection"},
		{ID: "clear", Label: "Clear"},
	}
}

func editMenu() []wire.BrowserMenuItem {
	return []wire.BrowserMenuItem{{ID: "edit", Label: "Edit…"}}
}
