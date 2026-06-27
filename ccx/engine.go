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
	"sync"

	"oblikovati.org/api/client"
	"oblikovati.org/api/wire"
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

	mu       sync.Mutex    // guards settings + running
	settings StudySettings // editable study parameters (panel-driven)
	running  bool          // a study is in flight (coalesces overlapping command triggers)
}

// NewEngine binds the engine to the host transport with the default study settings.
func NewEngine(host HostCaller) *Engine {
	return &Engine{host: host, api: client.New(host), settings: defaultSettings()}
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
	}
}

// onCommandStarted launches a study when our run command fires.
func (e *Engine) onCommandStarted(ev []byte) {
	var c struct {
		Command string `json:"command"`
	}
	if json.Unmarshal(ev, &c) == nil && c.Command == RunStudyCommandID {
		e.launchStudy()
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

	go func() {
		defer func() {
			e.mu.Lock()
			e.running = false
			e.mu.Unlock()
		}()
		if _, err := e.RunStudyOnHost(); err != nil {
			e.reportStatus("CalculiX study failed: " + err.Error())
			return
		}
		e.reportStatus("CalculiX study complete.")
	}()
}

// reportStatus surfaces a study's outcome on the host status bar (best-effort: a status
// failure must not mask the study result).
func (e *Engine) reportStatus(msg string) { _, _ = e.api.Status().SetText(msg) }
