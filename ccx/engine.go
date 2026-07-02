// SPDX-License-Identifier: GPL-2.0-only

// Package ccx is the host-facing core of the CalculiX FEA add-in: it turns a host
// body into a finite-element stress-analysis study (surface mesh → volume mesh →
// CalculiX input deck → solve → stress/displacement render) using only the
// Apache-2.0 oblikovati.org/api client. The cgo c-shared shell (../export.go) owns
// the C ABI; this package owns the FEA pipeline and stays cgo-free so it unit-tests
// on every platform.
package ccx

import (
	"encoding/json"
	"fmt"
	"sync"

	"oblikovati.org/api/client"
	"oblikovati.org/api/wire"
	"oblikovati.org/calculix/ccx/femmodel"
)

// HostCaller is the transport the engine talks to the host through — exactly the
// api/client Caller contract, supplied by the cgo shell at Activate (or a fake in
// tests). Keeping it an interface here keeps this package cgo-free and testable.
type HostCaller interface {
	Call(method string, req []byte) ([]byte, error)
}

// Engine runs CalculiX FEA studies against a live host.
type Engine struct {
	host HostCaller
	api  *client.Client

	mu          sync.Mutex         // guards analysis, extras, builderKind and running
	analysis    *femmodel.Analysis // tree-owned source of truth (Solver/Mesh/Material/Result/Constraints)
	extras      StudySettings      // not-yet-modeled flat params; overlaid by projectAnalysis
	builderKind ConstraintKind     // the constraint type the panel builder adds next
	running     bool               // a study is in flight (coalesces overlapping command triggers)
}

// NewEngine binds the engine to the host transport with the default study parameters.
func NewEngine(host HostCaller) *Engine {
	return &Engine{host: host, api: client.New(host),
		analysis: femmodel.NewDefaultAnalysis(), extras: defaultSettings()}
}

// study snapshots the study model under lock and projects it to the flat StudySettings the
// pipeline consumes — the ONE seam the mesh/deck/solve/render path reads.
func (e *Engine) study() (StudySettings, []ConstraintSpec) {
	e.mu.Lock()
	defer e.mu.Unlock()
	return projectAnalysis(e.analysis, e.extras)
}

// Notify receives host event bytes. A command.started carrying RunStudyCommandID runs the
// FEA study on a SEPARATE goroutine — never inline, because Notify is invoked on the host's
// session goroutine and a host call from there blocks until the frame loop drains the
// dispatcher (which cannot happen while we're inside it), deadlocking every host call. A
// guard coalesces overlapping triggers so one study is in flight at a time.
func (e *Engine) Notify(ev []byte) {
	var hdr struct {
		Type string `json:"type"`
	}
	if json.Unmarshal(ev, &hdr) != nil {
		return
	}
	switch hdr.Type {
	case wire.EventCommandStarted:
		e.onCommandStarted(ev)
	case wire.EventPanelValueChanged:
		e.onPanelValueChanged(ev)
	case wire.EventBrowserNode:
		e.onBrowserNode(ev)
	}
}

// onCommandStarted dispatches our registered commands. The study runs through launchStudy's
// coalescing guard; the constraint-builder commands make host calls (read selection, redraw the
// panel), so — like the study — they must run OFF the session goroutine to avoid the dispatcher
// deadlock, hence the goroutines.
func (e *Engine) onCommandStarted(ev []byte) {
	var c struct {
		Command string `json:"command"`
	}
	if json.Unmarshal(ev, &c) != nil {
		return
	}
	switch c.Command {
	case RunStudyCommandID:
		e.launchStudy()
	case AddConstraintCommandID:
		go e.addConstraintFromSelection()
	case ClearConstraintsCommandID:
		go e.clearConstraints()
	case ShowPanelCommandID:
		go func() { _, _ = e.ShowPanel() }()
	case ShowTreeCommandID:
		go func() { _, _ = e.ShowAnalysisTree() }()
	}
}

// onPanelValueChanged applies a panel edit. Editing a parameter only mutates engine
// state (no host call) — safe to run inline on the session goroutine.
func (e *Engine) onPanelValueChanged(ev []byte) {
	var p struct {
		WindowId  string `json:"windowId"`
		ControlId string `json:"controlId"`
		Value     string `json:"value"`
	}
	if json.Unmarshal(ev, &p) == nil && p.WindowId == PanelID {
		e.applyPanelEdit(p.ControlId, p.Value)
	}
}

// launchStudy starts one study goroutine, coalescing overlapping triggers, and reports the
// outcome to the host status bar so a failed solve is visible rather than silently empty.
func (e *Engine) launchStudy() {
	e.mu.Lock()
	if e.running {
		e.mu.Unlock()
		return
	}
	e.running = true
	e.mu.Unlock()

	go e.runAndReport()
}

// runAndReport runs one study and reports its outcome, recovering from any panic in the
// pipeline so a bug cannot take down the in-process host — the failure is surfaced on the
// status bar instead.
func (e *Engine) runAndReport() {
	defer func() {
		e.mu.Lock()
		e.running = false
		e.mu.Unlock()
		if r := recover(); r != nil {
			e.reportStatus(fmt.Sprintf("CalculiX study crashed: %v", r))
		}
	}()
	res, err := e.RunStudyOnHost()
	if err != nil {
		e.reportStatus("CalculiX study failed: " + err.Error())
		return
	}
	e.reportStatus(res.Summary())
}

// onBrowserNode dispatches a gesture on our Analysis pane (ignoring events for other panes).
func (e *Engine) onBrowserNode(ev []byte) {
	var b struct {
		Pane     string `json:"pane"`
		Node     string `json:"node"`
		Gesture  string `json:"gesture"`
		MenuItem string `json:"menuItem"`
	}
	if json.Unmarshal(ev, &b) == nil && b.Pane == AnalysisBrowserPaneID {
		e.handleAnalysisNode(b.Node, b.Gesture, b.MenuItem)
	}
}

// reportStatus surfaces a study's outcome on the host status bar (best-effort: a status
// failure must not mask the study result).
func (e *Engine) reportStatus(msg string) { _, _ = e.api.Status().SetText(msg) }
