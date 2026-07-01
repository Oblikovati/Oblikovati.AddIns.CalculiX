// SPDX-License-Identifier: GPL-2.0-only

package ccx

import "fmt"

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

// matIndexOf parses a leaf node ID of the form "mat:N" and returns N.
func matIndexOf(node string) (int, bool) { return indexOf(node, "mat:%d") }

// conIndexOf parses a leaf node ID of the form "con:N" and returns N. Category nodes
// ("constraints") do not match and return (0, false).
func conIndexOf(node string) (int, bool) { return indexOf(node, "con:%d") }

// resultIndexOf parses a leaf node ID of the form "result:N" and returns N.
func resultIndexOf(node string) (int, bool) { return indexOf(node, "result:%d") }

// indexOf is the shared Sscanf-based parser; it returns (i, true) on an exact match of
// format against node, and (0, false) on any error (wrong prefix, trailing chars, etc.).
func indexOf(node, format string) (int, bool) {
	var i int
	if _, err := fmt.Sscanf(node, format, &i); err == nil {
		return i, true
	}
	return 0, false
}
