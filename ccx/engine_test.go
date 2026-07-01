// SPDX-License-Identifier: GPL-2.0-only

package ccx

import (
	"encoding/json"
	"sync"
	"testing"
	"time"

	"oblikovati.org/api/wire"
)

// recordingHost is a named fake HostCaller (no live host): it records the wire methods
// it is asked to call and returns an empty OK body, enough to drive the M0 scaffold
// (command + panel registration, the Notify → study → status path).
type recordingHost struct {
	mu          sync.Mutex
	calls       []string
	createdCmds []wire.CreateCommandArgs // every commands.create request, decoded for placement assertions
}

func (h *recordingHost) Call(method string, payload []byte) ([]byte, error) {
	h.mu.Lock()
	h.calls = append(h.calls, method)
	if method == wire.MethodCommandsCreate {
		var a wire.CreateCommandArgs
		if json.Unmarshal(payload, &a) == nil {
			h.createdCmds = append(h.createdCmds, a)
		}
	}
	h.mu.Unlock()
	return []byte("{}"), nil
}

// createdCommandTabs decodes every recorded MethodCommandsCreate payload and returns
// a map of command ID → Tab so tests can assert placement without knowing order.
func (h *recordingHost) createdCommandTabs() map[string]string {
	h.mu.Lock()
	defer h.mu.Unlock()
	tabs := make(map[string]string, len(h.createdCmds))
	for _, a := range h.createdCmds {
		tabs[a.ID] = a.Tab
	}
	return tabs
}

func (h *recordingHost) saw(method string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, m := range h.calls {
		if m == method {
			return true
		}
	}
	return false
}

func TestSetupRegistersCommandAndPanel(t *testing.T) {
	h := &recordingHost{}
	if err := NewEngine(h).Setup(); err != nil {
		t.Fatalf("Setup: %v", err)
	}
	for _, m := range []string{wire.MethodCommandsCreate, wire.MethodDockableWindowsSet, wire.MethodBrowserSetPane} {
		if !h.saw(m) {
			t.Errorf("Setup never called %q (calls: %v)", m, h.calls)
		}
	}
}

func TestRunStudyOnHostErrorsWithoutSolver(t *testing.T) {
	// With no built solver on the relative path and no selection, the study must fail
	// loudly rather than silently producing nothing.
	if _, err := NewEngine(&recordingHost{}).RunStudyOnHost(); err == nil {
		t.Fatal("RunStudyOnHost should error when the solver/selection is unavailable")
	}
}

func TestApplyPanelEditUpdatesSettings(t *testing.T) {
	e := NewEngine(&recordingHost{})
	e.applyPanelEdit("mesh_size", "5 mm")
	e.applyPanelEdit("element_order", "linear (C3D4)")
	e.applyPanelEdit("analysis", string(AnalysisFrequency))
	s, _ := e.study()
	if s.MeshSizeMM != 5 {
		t.Errorf("MeshSizeMM = %v, want 5", s.MeshSizeMM)
	}
	if s.ElementOrder != LinearTet {
		t.Errorf("ElementOrder = %v, want LinearTet", s.ElementOrder)
	}
	if s.Analysis != AnalysisFrequency {
		t.Errorf("Analysis = %v, want frequency", s.Analysis)
	}
}

// TestNotifyCommandStartedRunsStudy verifies a command.started event for our command
// drives the study path and reports a status message (the M0 not-implemented failure).
func TestNotifyCommandStartedRunsStudy(t *testing.T) {
	h := &recordingHost{}
	e := NewEngine(h)
	ev, _ := json.Marshal(map[string]string{"type": wire.EventCommandStarted, "command": RunStudyCommandID})
	e.Notify(ev)
	waitIdle(e)
	if !h.saw(wire.MethodStatusSetText) {
		t.Errorf("study run never reported status (calls: %v)", h.calls)
	}
}

// TestNotifyRoutesBrowserNode verifies that a browser.node event on our pane routes to
// handleAnalysisNode and the study path, which surfaces a status message.
func TestNotifyRoutesBrowserNode(t *testing.T) {
	h := &recordingHost{}
	e := NewEngine(h)
	ev := []byte(`{"type":"browser.node","pane":"com.oblikovati.calculix.tree","node":"analysis","gesture":"menu","menuItem":"run"}`)
	e.Notify(ev)
	waitFor(t, func() bool { return h.saw(wire.MethodStatusSetText) }) // study path reports status
}

// waitIdle blocks until the study goroutine launched by Notify has finished.
func waitIdle(e *Engine) {
	for {
		e.mu.Lock()
		running := e.running
		e.mu.Unlock()
		if !running {
			return
		}
		time.Sleep(time.Millisecond)
	}
}

// waitFor polls cond until it returns true or 2 s elapse, at which point it calls
// t.Fatal. Used when an async goroutine drives a host call we cannot guard on e.running.
func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("waitFor: condition not satisfied within 2 s")
}

// commandStartedEvent builds the bytes for a command.started event carrying the given command id.
func commandStartedEvent(id string) []byte {
	ev, _ := json.Marshal(map[string]string{"type": wire.EventCommandStarted, "command": id})
	return ev
}

func TestRegisteredCommandsLandOnFEATab(t *testing.T) {
	h := &recordingHost{}
	if err := NewEngine(h).RegisterCommands(); err != nil {
		t.Fatalf("RegisterCommands: %v", err)
	}
	got := h.createdCommandTabs()
	for _, id := range []string{RunStudyCommandID, AddConstraintCommandID, ClearConstraintsCommandID,
		ShowPanelCommandID, ShowTreeCommandID} {
		if got[id] != "FEA" {
			t.Errorf("command %q on tab %q, want FEA", id, got[id])
		}
	}
}

func TestShowTreeCommandReopensTree(t *testing.T) {
	h := &recordingHost{}
	e := NewEngine(h)
	e.onCommandStarted(commandStartedEvent(ShowTreeCommandID))
	waitFor(t, func() bool { return h.saw(wire.MethodBrowserSetPane) })
}

func TestShowPanelCommandReopensPanel(t *testing.T) {
	h := &recordingHost{}
	e := NewEngine(h)
	e.onCommandStarted(commandStartedEvent(ShowPanelCommandID))
	waitFor(t, func() bool { return h.saw(wire.MethodDockableWindowsSet) })
}
