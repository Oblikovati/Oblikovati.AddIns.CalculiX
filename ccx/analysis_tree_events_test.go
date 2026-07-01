// SPDX-License-Identifier: GPL-2.0-only

package ccx

import (
	"testing"

	"oblikovati.org/api/wire"
)

func TestConIndexOf(t *testing.T) {
	if i, ok := conIndexOf("con:3"); !ok || i != 3 {
		t.Fatalf("conIndexOf(con:3) = %d,%v", i, ok)
	}
	if _, ok := conIndexOf("constraints"); ok {
		t.Fatal("category node must not parse as con:N")
	}
	// Exact match: trailing characters and a missing index must be rejected.
	for _, bad := range []string{"con:3extra", "con:", "con:x", "mat:3"} {
		if _, ok := conIndexOf(bad); ok {
			t.Fatalf("conIndexOf(%q) must reject, got ok=true", bad)
		}
	}
}

// TestAnalysisMenuRunRoutesToStudy verifies that a menu "run" on the "analysis" node drives
// the study path. launchStudy sets e.running=true synchronously, spawns the goroutine, and
// runAndReport sets it back when done — waitIdle polls that guard, then we assert a status
// call happened (reportStatus → api.Status().SetText = wire.MethodStatusSetText).
func TestAnalysisMenuRunRoutesToStudy(t *testing.T) {
	h := &recordingHost{}
	e := NewEngine(h)
	e.handleAnalysisNode("analysis", "menu", "run")
	waitIdle(e)
	if !h.saw(wire.MethodStatusSetText) {
		t.Fatal("menu 'run' did not launch the study (no status call observed)")
	}
}

// TestHandleAnalysisNodeDoubleClickCallsShowPanel verifies that a double-click on any node
// triggers a ShowPanel host call (the DockableWindows.Set wire method).
func TestHandleAnalysisNodeDoubleClickCallsShowPanel(t *testing.T) {
	h := &recordingHost{}
	e := NewEngine(h)
	e.handleAnalysisNode("solver", "double", "")
	// ShowPanel runs on its own goroutine; poll until the host call appears.
	waitFor(t, func() bool { return h.saw(wire.MethodDockableWindowsSet) })
}

// TestHandleAnalysisNodeMenuEditOpensPanel verifies the "edit" catch-all (solver/mesh/mat/result)
// opens the panel.
func TestHandleAnalysisNodeMenuEditOpensPanel(t *testing.T) {
	h := &recordingHost{}
	e := NewEngine(h)
	e.handleAnalysisNode("solver", "menu", "edit")
	waitFor(t, func() bool { return h.saw(wire.MethodDockableWindowsSet) })
}
