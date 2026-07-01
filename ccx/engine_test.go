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
	mu    sync.Mutex
	calls []string
}

func (h *recordingHost) Call(method string, _ []byte) ([]byte, error) {
	h.mu.Lock()
	h.calls = append(h.calls, method)
	h.mu.Unlock()
	return []byte("{}"), nil
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
	for _, m := range []string{wire.MethodCommandsCreate, wire.MethodDockableWindowsSet} {
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
