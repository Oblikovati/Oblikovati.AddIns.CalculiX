// SPDX-License-Identifier: GPL-2.0-only

package ccx

import (
	"strconv"
	"strings"
)

// handleAnalysisNode routes a browser gesture on the Analysis pane: double-click opens the
// existing study panel; a context-menu item runs its action. (ADR-3: editing stays in the
// existing dockable panel until the host modal task panel lands.)
//
//	e.handleAnalysisNode("analysis", "menu", "run")    // → launchStudy
//	e.handleAnalysisNode("solver",   "double", "")     // → ShowPanel
func (e *Engine) handleAnalysisNode(node, gesture, menuItem string) {
	switch gesture {
	case "double":
		go func() { _, _ = e.ShowPanel() }()
	case "menu":
		e.analysisMenu(node, menuItem)
	}
}

// analysisMenu dispatches a context-menu action on an Analysis-tree node.
// "run" on the analysis root drives launchStudy's coalescing guard (same as the ribbon
// command). Constraint mutations go through runAndRefreshAnalysisTree so the pane updates.
// Everything else ("edit") opens the flat study panel.
func (e *Engine) analysisMenu(node, item string) {
	switch {
	case node == "analysis" && item == "run":
		e.launchStudy()
	case node == "constraints" && item == "add":
		e.runAndRefreshAnalysisTree(e.addConstraintFromSelection)
	case node == "constraints" && item == "clear":
		e.runAndRefreshAnalysisTree(e.clearConstraints)
	default: // solver/mesh/mat/result "edit" → open the panel
		go func() { _, _ = e.ShowPanel() }()
	}
}

// runAndRefreshAnalysisTree runs a mutating action off the session goroutine (it makes host
// calls — selection, panel redraws), then re-declares the Analysis pane so the tree reflects
// the new state.
func (e *Engine) runAndRefreshAnalysisTree(action func()) {
	go func() {
		action()
		_, _ = e.ShowAnalysisTree()
	}()
}

// conIndexOf parses a leaf node ID of the form "con:N" and returns N. Category nodes
// ("constraints") do not match and return (0, false). (mat:N / result:N parsers land with
// per-object editing in a later slice — 2.2 double-clicks any node to the existing panel.)
func conIndexOf(node string) (int, bool) { return indexOf(node, "con") }

// indexOf parses a leaf node ID of the form "<kind>:N" and returns N. The match is EXACT:
// a wrong prefix, a missing index, or any trailing characters (e.g. "con:3extra") return
// (0, false) — so a future per-object dispatch cannot be fooled by a malformed node id.
func indexOf(node, kind string) (int, bool) {
	rest, ok := strings.CutPrefix(node, kind+":")
	if !ok {
		return 0, false
	}
	i, err := strconv.Atoi(rest)
	if err != nil {
		return 0, false
	}
	return i, true
}
