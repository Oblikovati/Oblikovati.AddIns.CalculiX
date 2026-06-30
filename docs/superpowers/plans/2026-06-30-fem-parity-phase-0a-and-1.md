<!-- SPDX-License-Identifier: Apache-2.0 -->
# FEM Parity — Phase 0a (panel/task API) + Phase 1 (femmodel seam) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add the public-API panel/task surface FreeCAD-style FEM editing needs (a `referenceList` control and a modal `TaskPanelSpec`), and establish the CalculiX add-in's `femmodel` Analysis aggregate plus the `projectAnalysis` seam, with no behavior change.

**Architecture:** Two **independent** parts that can be built in parallel. Part A extends the Apache-2.0 `oblikovati.org/api` module (types → wire → client), following ADR-0018 ordering; the GPL host implementation of these methods is a **separate follow-on plan** (it needs its own read of `Oblikovati/head/ui` + `addin/router`). Part B adds a pure `ccx/femmodel` package and a `ccx/project.go` projection function to the CalculiX add-in, proven by an equivalence test, **without wiring it into the engine yet** (the engine flip is folded into Phase 2 alongside the tree editor, to avoid rewriting the soon-retired `panel.go` twice).

**Tech Stack:** Go. `oblikovati.org/api` (Apache-2.0 contract module). `oblikovati.org/calculix` (the add-in, GPL-2.0-only, cgo-free `ccx/` package, links only `api/client`). Tests: standard `go test`.

## Global Constraints

- API additions are **additive only** → semver **minor** bump (`0.100.1` → `0.101.0`). Never re-declare a DTO/method string outside `api/wire`.
- `oblikovati.org/api` must NEVER import the GPL module; dependency flows one way only.
- Every new exported `.go` file carries an SPDX header: **`Apache-2.0`** in `Oblikovati.API`, **`GPL-2.0-only`** in the CalculiX add-in. (`scripts/add-spdx-headers.py` can backfill; the code blocks below already include the header.)
- `ccx/femmodel` imports **nothing** from `api` or the host — pure domain, leaf of the graph. `ccx` imports `femmodel`, never the reverse.
- Code style (repo CLAUDE.md): functions 4–20 lines, files <500 lines, explicit types (no `any`/untyped), early returns, exception/error messages include the offending value and expected shape.
- TDD: every new function gets a test written first and watched fail; commit after each green step.
- Backward-compat rule (host renderer, preserved): an unknown `PanelControlKind` degrades to its `Text`; a new method an old host lacks returns method-not-found and the add-in feature-detects + falls back.
- Repo paths: API repo = `/home/vmiguel/git/oblikovati-workspace/Oblikovati.API`; add-in repo = `/home/vmiguel/git/oblikovati-workspace/Oblikovati.AddIns.CalculiX` (module `oblikovati.org/calculix`, package `ccx`, sub-package `oblikovati.org/calculix/ccx/femmodel`).

---

# PART A — Phase 0a: public panel/task API surface (`Oblikovati.API`)

Work on a branch in the API repo: `git -C /home/vmiguel/git/oblikovati-workspace/Oblikovati.API checkout -b feature/fem-panel-task-api`.

### Task A1: `referenceList` control kind (types)

**Files:**
- Modify: `Oblikovati.API/types/panel_control_kind.go` (add kind 12 after `PanelTabs = 11`; add to `panelControlKindNames`)
- Test: `Oblikovati.API/types/panel_control_kind_test.go` (create)

**Interfaces:**
- Produces: `types.PanelReferenceList PanelControlKind = 12`; `PanelReferenceList.String() == "referenceList"`.

- [ ] **Step 1: Write the failing test**

Create `Oblikovati.API/types/panel_control_kind_test.go`:
```go
// SPDX-License-Identifier: Apache-2.0

package types

import "testing"

func TestPanelReferenceListKindName(t *testing.T) {
	if PanelReferenceList != 12 {
		t.Fatalf("PanelReferenceList = %d, want 12 (next free kind after PanelTabs=11)", PanelReferenceList)
	}
	if got := PanelReferenceList.String(); got != "referenceList" {
		t.Fatalf("PanelReferenceList.String() = %q, want %q", got, "referenceList")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /home/vmiguel/git/oblikovati-workspace/Oblikovati.API && go test ./types/ -run TestPanelReferenceListKindName -v`
Expected: FAIL — `undefined: PanelReferenceList`.

- [ ] **Step 3: Write minimal implementation**

In `types/panel_control_kind.go`, add this constant immediately after `PanelTabs PanelControlKind = 11`:
```go
	// PanelReferenceList is a list of picked host geometry references (faces/edges/
	// vertices) with host-driven Add-from-selection and per-row Remove. Rows holds the
	// current refs; Accepts limits which selection kinds may be added (empty = any).
	// Editing the rows pushes a [PanelReferencesChangedEvent] — NOT the scalar
	// PanelValueChangedEvent — because the value is a set, not one string.
	PanelReferenceList PanelControlKind = 12
```
And add `PanelReferenceList: "referenceList",` to the `panelControlKindNames` map literal.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./types/ -run TestPanelReferenceListKindName -v`
Expected: PASS. Then `go test ./types/` (whole package) — Expected: PASS (if an exhaustiveness test exists, it now sees the new kind via the updated map).

- [ ] **Step 5: Commit**
```bash
cd /home/vmiguel/git/oblikovati-workspace/Oblikovati.API
git add types/panel_control_kind.go types/panel_control_kind_test.go
git commit -m "feat(types): add referenceList panel control kind (12)"
```

### Task A2: reference-list wire DTOs, method + event constants

**Files:**
- Modify: `Oblikovati.API/wire/docking.go` (add `PanelReferenceRow`, two `PanelControlSpec` fields, `PanelReferencesChangedEvent`, `SetDockableWindowReferencesArgs`)
- Modify: `Oblikovati.API/wire/methods.go` (one method constant in the dockable block at line ~767–771; one event constant near `EventPanelValueChanged` at line ~948)
- Test: `Oblikovati.API/wire/docking_references_test.go` (create)

**Interfaces:**
- Consumes: `types.PanelReferenceList` (Task A1).
- Produces: `wire.PanelReferenceRow{Ref, Label string}`; `wire.PanelControlSpec.Rows []PanelReferenceRow`, `.Accepts []string`; `wire.PanelReferencesChangedEvent{Type, WindowId, ControlId string; Refs []string; Action string}`; `wire.SetDockableWindowReferencesArgs{WindowId, ControlId string; Refs []string}`; `wire.MethodDockableWindowsSetReferences = "dockableWindows.setReferences"`; `wire.EventPanelReferencesChanged = "panel.referencesChanged"`.

- [ ] **Step 1: Write the failing test**

Create `Oblikovati.API/wire/docking_references_test.go`:
```go
// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"encoding/json"
	"testing"

	"oblikovati.org/api/types"
)

func TestReferenceListSpecMarshals(t *testing.T) {
	spec := PanelControlSpec{
		Kind:    types.PanelReferenceList,
		ID:      "faces",
		Text:    "Faces",
		Accepts: []string{"face"},
		Rows:    []PanelReferenceRow{{Ref: "face/abc", Label: "Face3"}},
	}
	b, err := json.Marshal(spec)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var back PanelControlSpec
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(back.Rows) != 1 || back.Rows[0].Ref != "face/abc" || back.Accepts[0] != "face" {
		t.Fatalf("round-trip lost data: %+v", back)
	}
}

func TestReferenceMethodAndEventConstants(t *testing.T) {
	if MethodDockableWindowsSetReferences != "dockableWindows.setReferences" {
		t.Fatalf("method = %q", MethodDockableWindowsSetReferences)
	}
	if EventPanelReferencesChanged != "panel.referencesChanged" {
		t.Fatalf("event = %q", EventPanelReferencesChanged)
	}
	var a SetDockableWindowReferencesArgs = SetDockableWindowReferencesArgs{
		WindowId: "w", ControlId: "faces", Refs: []string{"face/abc"},
	}
	if a.Refs[0] != "face/abc" {
		t.Fatalf("args lost refs: %+v", a)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./wire/ -run 'TestReferenceListSpecMarshals|TestReferenceMethodAndEventConstants' -v`
Expected: FAIL — `unknown field Rows`, `undefined: MethodDockableWindowsSetReferences`.

- [ ] **Step 3: Write minimal implementation**

In `wire/docking.go`, add the two fields to `PanelControlSpec` (after the existing `Cell` field, keeping `omitempty`):
```go
	Rows    []PanelReferenceRow `json:"rows,omitempty"`    // referenceList: current picked refs
	Accepts []string            `json:"accepts,omitempty"` // referenceList: allowed kinds ("face"/"edge"/"vertex"); empty = any
```
Add these declarations at the end of `wire/docking.go`:
```go
// PanelReferenceRow is one row of a referenceList control: a host geometry selection
// reference plus an optional display label (the host derives one, e.g. "Face3", when empty).
type PanelReferenceRow struct {
	Ref   string `json:"ref"`
	Label string `json:"label,omitempty"`
}

// SetDockableWindowReferencesArgs is the request of [MethodDockableWindowsSetReferences]: it
// replaces a referenceList control's rows exactly as an Add-from-selection would, and notifies
// the owning add-in with a [PanelReferencesChangedEvent]. Refs is the full new set.
type SetDockableWindowReferencesArgs struct {
	WindowId  string   `json:"windowId"`
	ControlId string   `json:"controlId"`
	Refs      []string `json:"refs"`
}

// PanelReferencesChangedEvent is the push event (type [EventPanelReferencesChanged]) fired when a
// referenceList control's rows change — by the user's Add-from-selection / per-row Remove or by
// [MethodDockableWindowsSetReferences]. Refs is the FULL new set (bulk-state, matching the rest of
// the panel model); Action is "add"/"remove" for diagnostics only.
type PanelReferencesChangedEvent struct {
	Type      string   `json:"type"` // always EventPanelReferencesChanged
	WindowId  string   `json:"windowId"`
	ControlId string   `json:"controlId"`
	Refs      []string `json:"refs"`
	Action    string   `json:"action,omitempty"`
}
```
In `wire/methods.go`, add to the dockable-windows block (after `MethodDockableWindowsList` at line ~771):
```go
	MethodDockableWindowsSetReferences = "dockableWindows.setReferences"
```
And add near `EventPanelValueChanged` (line ~948):
```go
	// EventPanelReferencesChanged reports a referenceList control's row set changing
	// (see [PanelReferencesChangedEvent]): the add-in receives the window id, control id, and
	// the full new ref set.
	EventPanelReferencesChanged = "panel.referencesChanged"
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./wire/ -run 'TestReferenceListSpecMarshals|TestReferenceMethodAndEventConstants' -v`
Expected: PASS. Then `go test ./wire/` — Expected: PASS.

- [ ] **Step 5: Commit**
```bash
git add wire/docking.go wire/methods.go wire/docking_references_test.go
git commit -m "feat(wire): referenceList DTOs + setReferences method + referencesChanged event"
```

### Task A3: reference-list client helper + `SetReferences`

**Files:**
- Modify: `Oblikovati.API/client/panel_controls.go` (add `PanelReferenceList` builder)
- Modify: `Oblikovati.API/client/docking.go` (add `DockableWindows.SetReferences`)
- Test: `Oblikovati.API/client/docking_references_test.go` (create)

**Interfaces:**
- Consumes: `wire.PanelReferenceRow`, `wire.SetDockableWindowReferencesArgs`, `wire.MethodDockableWindowsSetReferences` (Task A2).
- Produces: `client.PanelReferenceList(id, text string, accepts []string, rows []wire.PanelReferenceRow) wire.PanelControlSpec`; `DockableWindows.SetReferences(windowID, controlID string, refs []string) (wire.OKResult, error)`.

- [ ] **Step 1: Write the failing test**

Create `Oblikovati.API/client/docking_references_test.go`:
```go
// SPDX-License-Identifier: Apache-2.0

package client

import (
	"encoding/json"
	"testing"

	"oblikovati.org/api/types"
	"oblikovati.org/api/wire"
)

func TestPanelReferenceListBuilder(t *testing.T) {
	c := PanelReferenceList("faces", "Faces", []string{"face"},
		[]wire.PanelReferenceRow{{Ref: "face/abc"}})
	if c.Kind != types.PanelReferenceList || c.ID != "faces" ||
		c.Accepts[0] != "face" || c.Rows[0].Ref != "face/abc" {
		t.Fatalf("builder produced %+v", c)
	}
}

func TestDockableWindowsSetReferences(t *testing.T) {
	ft := &fakeTransport{reply: []byte(`{"ok":true}`)}
	cl := New(ft)
	if _, err := cl.DockableWindows().SetReferences("w", "faces", []string{"face/abc"}); err != nil {
		t.Fatalf("SetReferences: %v", err)
	}
	if ft.gotMethod != wire.MethodDockableWindowsSetReferences {
		t.Fatalf("method = %q, want %q", ft.gotMethod, wire.MethodDockableWindowsSetReferences)
	}
	var sent wire.SetDockableWindowReferencesArgs
	if err := json.Unmarshal(ft.gotReq, &sent); err != nil ||
		sent.WindowId != "w" || sent.ControlId != "faces" || sent.Refs[0] != "face/abc" {
		t.Fatalf("SetReferences sent %s", ft.gotReq)
	}
}
```
(`fakeTransport` with `reply`, `gotMethod`, `gotReq` already exists in `client` — it is the fake used by `client/browser_test.go`.)

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./client/ -run 'TestPanelReferenceListBuilder|TestDockableWindowsSetReferences' -v`
Expected: FAIL — `undefined: PanelReferenceList`, `SetReferences`.

- [ ] **Step 3: Write minimal implementation**

Append to `client/panel_controls.go`:
```go
// PanelReferenceList builds a geometry reference-list control: rows are the current picked
// refs; accepts limits which host selection kinds Add may append ("face"/"edge"/"vertex";
// empty = any). Row edits arrive as a wire.PanelReferencesChangedEvent, not the scalar value event.
func PanelReferenceList(id, text string, accepts []string, rows []wire.PanelReferenceRow) wire.PanelControlSpec {
	return wire.PanelControlSpec{Kind: types.PanelReferenceList, ID: id, Text: text, Accepts: accepts, Rows: rows}
}
```
Append to `client/docking.go` (inside the file, after `SetValue`):
```go
// SetReferences replaces a referenceList control's rows exactly as an Add-from-selection would:
// the host updates the stored rows and notifies the owning add-in with a
// wire.PanelReferencesChangedEvent. Refs is the full new set.
//
// mcp:tool set_panel_references
// mcp:summary Replace a reference-list control's rows (as Add-from-selection would), notifying the add-in.
func (d DockableWindows) SetReferences(windowID, controlID string, refs []string) (wire.OKResult, error) {
	var r wire.OKResult
	args := wire.SetDockableWindowReferencesArgs{WindowId: windowID, ControlId: controlID, Refs: refs}
	return r, d.c.call(wire.MethodDockableWindowsSetReferences, args, &r)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./client/ -run 'TestPanelReferenceListBuilder|TestDockableWindowsSetReferences' -v`
Expected: PASS. Then `go test ./client/` — Expected: PASS.

- [ ] **Step 5: Commit**
```bash
git add client/panel_controls.go client/docking.go client/docking_references_test.go
git commit -m "feat(client): PanelReferenceList builder + DockableWindows.SetReferences"
```

### Task A4: modal `TaskPanelSpec` wire DTOs + method/event constants

**Files:**
- Create: `Oblikovati.API/wire/task_panel.go`
- Modify: `Oblikovati.API/wire/methods.go` (two method constants in the modal-dialog block at line ~814; one event constant near the other UI events)
- Test: `Oblikovati.API/wire/task_panel_test.go` (create)

**Interfaces:**
- Produces: `wire.TaskPanelSpec{ID, Title string; Controls []PanelControlSpec; OKLabel, CancelLabel string}`; `wire.ShowTaskPanelArgs{Panel TaskPanelSpec}`; `wire.CloseTaskPanelArgs{ID string}`; `wire.TaskPanelClosedEvent{Type, ID string; Accepted bool}`; `wire.MethodTaskPanelShow = "taskPanel.show"`; `wire.MethodTaskPanelClose = "taskPanel.close"`; `wire.EventTaskPanelClosed = "taskPanel.closed"`.

- [ ] **Step 1: Write the failing test**

Create `Oblikovati.API/wire/task_panel_test.go`:
```go
// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"encoding/json"
	"testing"
)

func TestTaskPanelSpecRoundTrip(t *testing.T) {
	spec := TaskPanelSpec{
		ID: "fix", Title: "Fixed Constraint",
		Controls: []PanelControlSpec{{ID: "faces"}},
		OKLabel:  "OK", CancelLabel: "Cancel",
	}
	b, _ := json.Marshal(ShowTaskPanelArgs{Panel: spec})
	var back ShowTaskPanelArgs
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if back.Panel.ID != "fix" || back.Panel.Title != "Fixed Constraint" || len(back.Panel.Controls) != 1 {
		t.Fatalf("round-trip lost data: %+v", back.Panel)
	}
}

func TestTaskPanelConstants(t *testing.T) {
	if MethodTaskPanelShow != "taskPanel.show" || MethodTaskPanelClose != "taskPanel.close" {
		t.Fatalf("methods: %q %q", MethodTaskPanelShow, MethodTaskPanelClose)
	}
	if EventTaskPanelClosed != "taskPanel.closed" {
		t.Fatalf("event: %q", EventTaskPanelClosed)
	}
	ev := TaskPanelClosedEvent{Type: EventTaskPanelClosed, ID: "fix", Accepted: true}
	if !ev.Accepted {
		t.Fatalf("event lost Accepted: %+v", ev)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./wire/ -run 'TestTaskPanel' -v`
Expected: FAIL — `undefined: TaskPanelSpec`.

- [ ] **Step 3: Write minimal implementation**

Create `wire/task_panel.go`:
```go
// SPDX-License-Identifier: Apache-2.0

package wire

// TaskPanelSpec is a modal task panel that takes over the host task area (FreeCAD Task-dialog
// semantics): declarative PanelControlSpec content with OK/Cancel. Showing is asynchronous — like
// file dialogs it must never block the session goroutine — so the user's accept/cancel arrives as a
// [TaskPanelClosedEvent]. While open, edits to its controls push the same PanelValueChangedEvent /
// PanelReferencesChangedEvent keyed on this panel's ID. Unlike WebDialogSpec it is built from the
// shared control vocabulary, so a referenceList composes inside it.
type TaskPanelSpec struct {
	ID          string             `json:"id"`
	Title       string             `json:"title"`
	Controls    []PanelControlSpec `json:"controls,omitempty"`
	OKLabel     string             `json:"okLabel,omitempty"`     // default "OK"
	CancelLabel string             `json:"cancelLabel,omitempty"` // default "Cancel"
}

// ShowTaskPanelArgs is the request of [MethodTaskPanelShow].
type ShowTaskPanelArgs struct {
	Panel TaskPanelSpec `json:"panel"`
}

// CloseTaskPanelArgs is the request of [MethodTaskPanelClose] (programmatic dismissal).
type CloseTaskPanelArgs struct {
	ID string `json:"id"`
}

// TaskPanelClosedEvent is the push event (type [EventTaskPanelClosed]) delivering the user's
// accept/cancel. Accepted is true for OK, false for Cancel/close. Control values are NOT echoed —
// they already arrived incrementally via the value/references events while the panel was open.
type TaskPanelClosedEvent struct {
	Type     string `json:"type"` // always EventTaskPanelClosed
	ID       string `json:"id"`
	Accepted bool   `json:"accepted"`
}
```
In `wire/methods.go`, add to the modal-dialog block (after `MethodDialogsListWebViews` at line ~816):
```go
	// Modal task panels built from declarative controls (FEM/parametric editing).
	MethodTaskPanelShow  = "taskPanel.show"
	MethodTaskPanelClose = "taskPanel.close"
```
And add near `EventWebDialogChanged` (line ~940):
```go
	// EventTaskPanelClosed delivers a modal task panel's accept/cancel
	// (see [TaskPanelClosedEvent]).
	EventTaskPanelClosed = "taskPanel.closed"
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./wire/ -run 'TestTaskPanel' -v` → PASS. Then `go test ./wire/` → PASS.

- [ ] **Step 5: Commit**
```bash
git add wire/task_panel.go wire/methods.go wire/task_panel_test.go
git commit -m "feat(wire): modal TaskPanelSpec DTOs + show/close methods + closed event"
```

### Task A5: `TaskPanels` client group

**Files:**
- Create: `Oblikovati.API/client/task_panels.go`
- Test: `Oblikovati.API/client/task_panels_test.go`

**Interfaces:**
- Consumes: `wire.TaskPanelSpec`, `wire.ShowTaskPanelArgs`, `wire.CloseTaskPanelArgs`, method constants (Task A4).
- Produces: `Client.TaskPanels() TaskPanels`; `TaskPanels.Show(p wire.TaskPanelSpec) (wire.OKResult, error)`; `TaskPanels.Close(id string) (wire.OKResult, error)`.

- [ ] **Step 1: Write the failing test**

Create `Oblikovati.API/client/task_panels_test.go`:
```go
// SPDX-License-Identifier: Apache-2.0

package client

import (
	"encoding/json"
	"testing"

	"oblikovati.org/api/wire"
)

func TestTaskPanelsShowAndClose(t *testing.T) {
	ft := &fakeTransport{reply: []byte(`{"ok":true}`)}
	cl := New(ft)
	if _, err := cl.TaskPanels().Show(wire.TaskPanelSpec{ID: "fix", Title: "Fixed"}); err != nil {
		t.Fatalf("Show: %v", err)
	}
	if ft.gotMethod != wire.MethodTaskPanelShow {
		t.Fatalf("method = %q, want %q", ft.gotMethod, wire.MethodTaskPanelShow)
	}
	var shown wire.ShowTaskPanelArgs
	if err := json.Unmarshal(ft.gotReq, &shown); err != nil || shown.Panel.ID != "fix" {
		t.Fatalf("Show sent %s", ft.gotReq)
	}
	if _, err := cl.TaskPanels().Close("fix"); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if ft.gotMethod != wire.MethodTaskPanelClose {
		t.Fatalf("method = %q, want %q", ft.gotMethod, wire.MethodTaskPanelClose)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./client/ -run TestTaskPanelsShowAndClose -v`
Expected: FAIL — `undefined: (*Client).TaskPanels`.

- [ ] **Step 3: Write minimal implementation**

Create `client/task_panels.go`:
```go
// SPDX-License-Identifier: Apache-2.0

package client

import "oblikovati.org/api/wire"

// TaskPanels is the modal task-panel operation group: show a FreeCAD-Task-style modal panel built
// from declarative controls, with OK/Cancel. The result arrives as a wire.TaskPanelClosedEvent;
// control edits arrive as the ordinary value/references events keyed on the panel id.
type TaskPanels struct{ c *Client }

// TaskPanels returns the modal task-panel operation group.
func (c *Client) TaskPanels() TaskPanels { return TaskPanels{c} }

// Show displays the modal task panel (asynchronous — never blocks; the accept/cancel arrives as a
// wire.TaskPanelClosedEvent).
//
// mcp:tool task_panel_show
// mcp:summary Show a modal task panel (OK/Cancel) built from declarative controls.
func (t TaskPanels) Show(p wire.TaskPanelSpec) (wire.OKResult, error) {
	var r wire.OKResult
	return r, t.c.call(wire.MethodTaskPanelShow, wire.ShowTaskPanelArgs{Panel: p}, &r)
}

// Close dismisses an open task panel programmatically.
//
// mcp:tool task_panel_close
// mcp:summary Dismiss an open task panel programmatically.
func (t TaskPanels) Close(id string) (wire.OKResult, error) {
	var r wire.OKResult
	return r, t.c.call(wire.MethodTaskPanelClose, wire.CloseTaskPanelArgs{ID: id}, &r)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./client/ -run TestTaskPanelsShowAndClose -v` → PASS. Then `go test ./...` (whole API module) → PASS.

- [ ] **Step 5: Commit**
```bash
git add client/task_panels.go client/task_panels_test.go
git commit -m "feat(client): TaskPanels group (Show/Close) for modal task panels"
```

### Task A6: version bump (cuts the API release)

**Files:**
- Modify: `Oblikovati.API/version.go` (`Version` const)
- Test: `Oblikovati.API/version_test.go` (existing `TestVersionIsSemver` must stay green; no new test needed)

**Interfaces:**
- Produces: `api.Version == "0.101.0"` (release tag `v0.101.0`, auto-cut by the version-bump bot on merge).

- [ ] **Step 1: Bump the version**

In `version.go`, change:
```go
const Version = "0.100.1"
```
to:
```go
const Version = "0.101.0"
```

- [ ] **Step 2: Run the version + full module tests**

Run: `cd /home/vmiguel/git/oblikovati-workspace/Oblikovati.API && go test ./...`
Expected: PASS (including `TestVersionIsSemver`).

- [ ] **Step 3: Lint**

Run: `golangci-lint run` (in the API repo). Expected: clean.

- [ ] **Step 4: Commit**
```bash
git add version.go
git commit -m "release: v0.101.0 — referenceList control + modal TaskPanelSpec"
```

**After Part A:** open the API PR. The host implementation of these three methods + the
two events (router handlers + `head/ui` rendering + the session store) is a **separate
follow-on plan** (`Phase 0a-host`), written after reading `Oblikovati/addin/router/ui_surfaces.go`
and `Oblikovati/head/ui/addin_panels.go`. The CalculiX add-in does not consume these methods
until Phase 3, so Part B below does not depend on Part A.

---

# PART B — Phase 1: `femmodel` aggregate + `projectAnalysis` seam (CalculiX add-in)

Pure, additive foundation. **No existing file is modified** — `ccx/femmodel/*` is new, and
`ccx/project.go` is new. The engine still uses `e.settings`; the projection is proven by an
equivalence test and wired into the engine in Phase 2 (when `panel.go` is retired). Work on the
already-created branch `feature/freecad-fem-parity` in the add-in repo.

### Task B1: `femmodel` scaffold — `Category` + `FEMObject`

**Files:**
- Create: `ccx/femmodel/category.go`, `ccx/femmodel/object.go`
- Test: `ccx/femmodel/object_test.go`

**Interfaces:**
- Produces: `femmodel.Category` (`CategorySolver`/`CategoryMesh`/`CategoryMaterial`/`CategoryConstraint`/`CategoryResult`); `femmodel.FEMObject` interface (`ObjectID() string`, `Category() Category`, `Name() string`).

- [ ] **Step 1: Write the failing test**

Create `ccx/femmodel/object_test.go`:
```go
// SPDX-License-Identifier: GPL-2.0-only

package femmodel

import "testing"

func TestCategoryString(t *testing.T) {
	cases := map[Category]string{
		CategorySolver: "Solver", CategoryMesh: "Mesh", CategoryMaterial: "Material",
		CategoryConstraint: "Constraint", CategoryResult: "Result",
	}
	for c, want := range cases {
		if got := c.String(); got != want {
			t.Fatalf("Category(%d).String() = %q, want %q", c, got, want)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /home/vmiguel/git/oblikovati-workspace/Oblikovati.AddIns.CalculiX && go test ./ccx/femmodel/ -run TestCategoryString -v`
Expected: FAIL — `undefined: Category` / package does not exist.

- [ ] **Step 3: Write minimal implementation**

Create `ccx/femmodel/category.go`:
```go
// SPDX-License-Identifier: GPL-2.0-only

// Package femmodel is the pure Analysis domain of the CalculiX add-in: a tree of first-class FEM
// objects (Solver, Mesh, Material, Constraint, Result) under an Analysis aggregate. It imports
// neither the host nor oblikovati.org/api — the add-in's ccx package projects it onto the solver
// pipeline (see ccx/project.go). Keeping it pure makes the model unit-testable on every platform.
package femmodel

// Category is the kind of a FEM object within an Analysis.
type Category int

const (
	CategorySolver Category = iota
	CategoryMesh
	CategoryMaterial
	CategoryConstraint
	CategoryResult
)

var categoryNames = map[Category]string{
	CategorySolver: "Solver", CategoryMesh: "Mesh", CategoryMaterial: "Material",
	CategoryConstraint: "Constraint", CategoryResult: "Result",
}

// String returns the category's stable display name.
func (c Category) String() string {
	if n, ok := categoryNames[c]; ok {
		return n
	}
	return "Category(?)"
}
```
Create `ccx/femmodel/object.go`:
```go
// SPDX-License-Identifier: GPL-2.0-only

package femmodel

// FEMObject is one first-class node of an Analysis tree. Every object has a stable id (unique
// within its Analysis), a category, and a display name shown in the browser tree.
type FEMObject interface {
	ObjectID() string
	Category() Category
	Name() string
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./ccx/femmodel/ -run TestCategoryString -v` → PASS.

- [ ] **Step 5: Commit**
```bash
git add ccx/femmodel/category.go ccx/femmodel/object.go ccx/femmodel/object_test.go
git commit -m "feat(femmodel): Category enum + FEMObject interface scaffold"
```

### Task B2: `SolverObject` + `MeshObject`

**Files:**
- Create: `ccx/femmodel/solver_object.go`, `ccx/femmodel/mesh_object.go`
- Test: `ccx/femmodel/solver_mesh_test.go`

**Interfaces:**
- Consumes: `Category`, `FEMObject` (B1).
- Produces: `femmodel.SolverObject{AnalysisType string; Eigenmodes int; TransientTimeS float64}` and `femmodel.MeshObject{MaxSizeMM float64; Quadratic bool}`, each with `ObjectID()/Category()/Name()` and a constructor `newSolverObject(id, analysisType string, eigenmodes int, transientS float64)` / `newMeshObject(id string, maxSizeMM float64, quadratic bool)`. (`AnalysisType` is the canonical analysis name string, e.g. `"static"`, matching `ccx.AnalysisType`'s underlying value.)

- [ ] **Step 1: Write the failing test**

Create `ccx/femmodel/solver_mesh_test.go`:
```go
// SPDX-License-Identifier: GPL-2.0-only

package femmodel

import "testing"

func TestSolverObject(t *testing.T) {
	s := newSolverObject("solver", "static", 6, 0)
	if s.ObjectID() != "solver" || s.Category() != CategorySolver || s.Name() != "Solver" {
		t.Fatalf("solver identity wrong: %+v", s)
	}
	if s.AnalysisType != "static" || s.Eigenmodes != 6 || s.TransientTimeS != 0 {
		t.Fatalf("solver fields wrong: %+v", s)
	}
}

func TestMeshObject(t *testing.T) {
	m := newMeshObject("mesh", 0, true)
	if m.ObjectID() != "mesh" || m.Category() != CategoryMesh || m.Name() != "Mesh" {
		t.Fatalf("mesh identity wrong: %+v", m)
	}
	if m.MaxSizeMM != 0 || !m.Quadratic {
		t.Fatalf("mesh fields wrong: %+v", m)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./ccx/femmodel/ -run 'TestSolverObject|TestMeshObject' -v`
Expected: FAIL — `undefined: newSolverObject`.

- [ ] **Step 3: Write minimal implementation**

Create `ccx/femmodel/solver_object.go`:
```go
// SPDX-License-Identifier: GPL-2.0-only

package femmodel

// SolverObject is the analysis-procedure object: which CalculiX *STEP to run, how many eigenmodes
// (frequency/buckling), and the transient total time (0 = steady). AnalysisType is the canonical
// analysis name string (e.g. "static") the add-in maps to its ccx.AnalysisType.
type SolverObject struct {
	id             string
	AnalysisType   string
	Eigenmodes     int
	TransientTimeS float64
}

func newSolverObject(id, analysisType string, eigenmodes int, transientS float64) SolverObject {
	return SolverObject{id: id, AnalysisType: analysisType, Eigenmodes: eigenmodes, TransientTimeS: transientS}
}

func (o SolverObject) ObjectID() string   { return o.id }
func (o SolverObject) Category() Category  { return CategorySolver }
func (o SolverObject) Name() string        { return "Solver" }
```
Create `ccx/femmodel/mesh_object.go`:
```go
// SPDX-License-Identifier: GPL-2.0-only

package femmodel

// MeshObject is the volume-mesh object: gmsh characteristic length (mm; 0 = auto) and element
// order (Quadratic = C3D10, the default; false = linear C3D4).
type MeshObject struct {
	id        string
	MaxSizeMM float64
	Quadratic bool
}

func newMeshObject(id string, maxSizeMM float64, quadratic bool) MeshObject {
	return MeshObject{id: id, MaxSizeMM: maxSizeMM, Quadratic: quadratic}
}

func (o MeshObject) ObjectID() string  { return o.id }
func (o MeshObject) Category() Category { return CategoryMesh }
func (o MeshObject) Name() string       { return "Mesh" }
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./ccx/femmodel/ -run 'TestSolverObject|TestMeshObject' -v` → PASS.

- [ ] **Step 5: Commit**
```bash
git add ccx/femmodel/solver_object.go ccx/femmodel/mesh_object.go ccx/femmodel/solver_mesh_test.go
git commit -m "feat(femmodel): SolverObject + MeshObject"
```

### Task B3: `MaterialObject`

**Files:**
- Create: `ccx/femmodel/material_object.go`
- Test: `ccx/femmodel/material_object_test.go`

**Interfaces:**
- Produces: `femmodel.MaterialObject{YoungGPa, Poisson, DensityGCm3, YieldMPa float64; ScopeAll bool}` with `ObjectID()/Category()/Name()` (Name returns the material's display name) and `newMaterialObject(id, name string, young, poisson, density, yield float64, scopeAll bool)`.

- [ ] **Step 1: Write the failing test**

Create `ccx/femmodel/material_object_test.go`:
```go
// SPDX-License-Identifier: GPL-2.0-only

package femmodel

import "testing"

func TestMaterialObject(t *testing.T) {
	m := newMaterialObject("mat1", "Steel", 210, 0.3, 7.85, 0, true)
	if m.ObjectID() != "mat1" || m.Category() != CategoryMaterial || m.Name() != "Steel" {
		t.Fatalf("material identity wrong: %+v", m)
	}
	if m.YoungGPa != 210 || m.Poisson != 0.3 || m.DensityGCm3 != 7.85 || m.YieldMPa != 0 || !m.ScopeAll {
		t.Fatalf("material fields wrong: %+v", m)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./ccx/femmodel/ -run TestMaterialObject -v`
Expected: FAIL — `undefined: newMaterialObject`.

- [ ] **Step 3: Write minimal implementation**

Create `ccx/femmodel/material_object.go`:
```go
// SPDX-License-Identifier: GPL-2.0-only

package femmodel

// MaterialObject is one material assignment. ScopeAll marks the fallback material applied to every
// body that has no more-specific assignment. Phase 1 carries the core mechanical properties; thermal
// and electromagnetic properties migrate here in a later phase.
type MaterialObject struct {
	id          string
	name        string
	YoungGPa    float64
	Poisson     float64
	DensityGCm3 float64
	YieldMPa    float64
	ScopeAll    bool
}

func newMaterialObject(id, name string, young, poisson, density, yield float64, scopeAll bool) MaterialObject {
	return MaterialObject{
		id: id, name: name, YoungGPa: young, Poisson: poisson,
		DensityGCm3: density, YieldMPa: yield, ScopeAll: scopeAll,
	}
}

func (o MaterialObject) ObjectID() string  { return o.id }
func (o MaterialObject) Category() Category { return CategoryMaterial }
func (o MaterialObject) Name() string       { return o.name }
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./ccx/femmodel/ -run TestMaterialObject -v` → PASS.

- [ ] **Step 5: Commit**
```bash
git add ccx/femmodel/material_object.go ccx/femmodel/material_object_test.go
git commit -m "feat(femmodel): MaterialObject"
```

### Task B4: `ResultObject`

**Files:**
- Create: `ccx/femmodel/result_object.go`
- Test: `ccx/femmodel/result_object_test.go`

**Interfaces:**
- Produces: `femmodel.ResultObject{Field string; DeformScale float64}` with `ObjectID()/Category()/Name()` (Name returns `"Results"`) and `newResultObject(id, field string, deformScale float64)`. (`Field` is the `ccx.ResultFieldKind` underlying string, e.g. `"von Mises stress"`.)

- [ ] **Step 1: Write the failing test**

Create `ccx/femmodel/result_object_test.go`:
```go
// SPDX-License-Identifier: GPL-2.0-only

package femmodel

import "testing"

func TestResultObject(t *testing.T) {
	r := newResultObject("result1", "von Mises stress", 0)
	if r.ObjectID() != "result1" || r.Category() != CategoryResult || r.Name() != "Results" {
		t.Fatalf("result identity wrong: %+v", r)
	}
	if r.Field != "von Mises stress" || r.DeformScale != 0 {
		t.Fatalf("result fields wrong: %+v", r)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./ccx/femmodel/ -run TestResultObject -v`
Expected: FAIL — `undefined: newResultObject`.

- [ ] **Step 3: Write minimal implementation**

Create `ccx/femmodel/result_object.go`:
```go
// SPDX-License-Identifier: GPL-2.0-only

package femmodel

// ResultObject is the result-display object: which scalar field colours the result and the
// deformed-shape magnification (0 = auto). Post-processing filters attach here in Phase 5.
type ResultObject struct {
	id          string
	Field       string
	DeformScale float64
}

func newResultObject(id, field string, deformScale float64) ResultObject {
	return ResultObject{id: id, Field: field, DeformScale: deformScale}
}

func (o ResultObject) ObjectID() string  { return o.id }
func (o ResultObject) Category() Category { return CategoryResult }
func (o ResultObject) Name() string       { return "Results" }
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./ccx/femmodel/ -run TestResultObject -v` → PASS.

- [ ] **Step 5: Commit**
```bash
git add ccx/femmodel/result_object.go ccx/femmodel/result_object_test.go
git commit -m "feat(femmodel): ResultObject"
```

### Task B5: `Analysis` aggregate + invariants + `NewDefaultAnalysis`

**Files:**
- Create: `ccx/femmodel/analysis.go`
- Test: `ccx/femmodel/analysis_test.go`

**Interfaces:**
- Consumes: all object types (B2–B4).
- Produces: `femmodel.NewDefaultAnalysis() *Analysis`; accessors `(*Analysis).Solver() SolverObject`, `.Mesh() MeshObject`, `.Materials() []MaterialObject`, `.Results() []ResultObject`, `.DefaultMaterial() (MaterialObject, bool)`, `.PrimaryResult() (ResultObject, bool)`; mutators `.SetSolver(SolverObject)`, `.SetMesh(MeshObject)`, `.AddMaterial(name string, young, poisson, density, yield float64, scopeAll bool) MaterialObject`, `.AddResult(field string, deformScale float64) ResultObject`. Invariants: exactly one Solver + one Mesh; ≥1 Material; unique ids.

- [ ] **Step 1: Write the failing test**

Create `ccx/femmodel/analysis_test.go`:
```go
// SPDX-License-Identifier: GPL-2.0-only

package femmodel

import "testing"

func TestNewDefaultAnalysisCoreValues(t *testing.T) {
	a := NewDefaultAnalysis()
	if a.Solver().AnalysisType != "static" || a.Solver().Eigenmodes != 6 || a.Solver().TransientTimeS != 0 {
		t.Fatalf("default solver wrong: %+v", a.Solver())
	}
	if a.Mesh().MaxSizeMM != 0 || !a.Mesh().Quadratic {
		t.Fatalf("default mesh wrong: %+v", a.Mesh())
	}
	mat, ok := a.DefaultMaterial()
	if !ok || mat.YoungGPa != 210 || mat.Poisson != 0.3 || mat.DensityGCm3 != 7.85 || mat.YieldMPa != 0 {
		t.Fatalf("default material wrong: %+v ok=%v", mat, ok)
	}
	r, ok := a.PrimaryResult()
	if !ok || r.Field != "von Mises stress" || r.DeformScale != 0 {
		t.Fatalf("default result wrong: %+v ok=%v", r, ok)
	}
}

func TestAddMaterialAssignsUniqueIDs(t *testing.T) {
	a := NewDefaultAnalysis()
	first := len(a.Materials())
	m2 := a.AddMaterial("Aluminium", 69, 0.33, 2.70, 0, false)
	if m2.ObjectID() == a.Materials()[0].ObjectID() {
		t.Fatalf("AddMaterial reused id %q", m2.ObjectID())
	}
	if len(a.Materials()) != first+1 {
		t.Fatalf("AddMaterial did not append: %d", len(a.Materials()))
	}
	if _, ok := a.DefaultMaterial(); !ok {
		t.Fatalf("DefaultMaterial lost after adding a scoped material")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./ccx/femmodel/ -run 'TestNewDefaultAnalysis|TestAddMaterial' -v`
Expected: FAIL — `undefined: NewDefaultAnalysis`.

- [ ] **Step 3: Write minimal implementation**

Create `ccx/femmodel/analysis.go`:
```go
// SPDX-License-Identifier: GPL-2.0-only

package femmodel

import "strconv"

// Analysis is the root aggregate: exactly one Solver and one Mesh, at least one Material (the
// ScopeAll one is the fallback), and the result-display objects. It is the source of truth the
// add-in projects onto its solver pipeline. Mutators preserve the invariants; ids are unique
// within the aggregate.
type Analysis struct {
	solver    SolverObject
	mesh      MeshObject
	materials []MaterialObject
	results   []ResultObject
	nextMat   int
	nextResult int
}

// NewDefaultAnalysis returns the v1 defaults, matching the add-in's defaultSettings(): linear-static,
// quadratic tets, auto mesh size, a mild-steel fallback material, and a von-Mises result.
func NewDefaultAnalysis() *Analysis {
	a := &Analysis{
		solver: newSolverObject("solver", "static", 6, 0),
		mesh:   newMeshObject("mesh", 0, true),
	}
	a.AddMaterial("Steel", 210, 0.3, 7.85, 0, true)
	a.AddResult("von Mises stress", 0)
	return a
}

// Solver returns the single solver object.
func (a *Analysis) Solver() SolverObject { return a.solver }

// Mesh returns the single mesh object.
func (a *Analysis) Mesh() MeshObject { return a.mesh }

// Materials returns the materials in insertion order.
func (a *Analysis) Materials() []MaterialObject { return a.materials }

// Results returns the result objects in insertion order.
func (a *Analysis) Results() []ResultObject { return a.results }

// SetSolver replaces the solver object (preserving its id).
func (a *Analysis) SetSolver(s SolverObject) { s.id = a.solver.id; a.solver = s }

// SetMesh replaces the mesh object (preserving its id).
func (a *Analysis) SetMesh(m MeshObject) { m.id = a.mesh.id; a.mesh = m }

// AddMaterial appends a material with a fresh unique id and returns it.
func (a *Analysis) AddMaterial(name string, young, poisson, density, yield float64, scopeAll bool) MaterialObject {
	a.nextMat++
	m := newMaterialObject("mat"+strconv.Itoa(a.nextMat), name, young, poisson, density, yield, scopeAll)
	a.materials = append(a.materials, m)
	return m
}

// AddResult appends a result-display object with a fresh unique id and returns it.
func (a *Analysis) AddResult(field string, deformScale float64) ResultObject {
	a.nextResult++
	r := newResultObject("result"+strconv.Itoa(a.nextResult), field, deformScale)
	a.results = append(a.results, r)
	return r
}

// DefaultMaterial returns the ScopeAll fallback material (the first one if none is explicitly
// ScopeAll), and false only when there is no material at all.
func (a *Analysis) DefaultMaterial() (MaterialObject, bool) {
	if len(a.materials) == 0 {
		return MaterialObject{}, false
	}
	for _, m := range a.materials {
		if m.ScopeAll {
			return m, true
		}
	}
	return a.materials[0], true
}

// PrimaryResult returns the first result-display object, false when there is none.
func (a *Analysis) PrimaryResult() (ResultObject, bool) {
	if len(a.results) == 0 {
		return ResultObject{}, false
	}
	return a.results[0], true
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./ccx/femmodel/ -v` → PASS (all femmodel tests).

- [ ] **Step 5: Commit**
```bash
git add ccx/femmodel/analysis.go ccx/femmodel/analysis_test.go
git commit -m "feat(femmodel): Analysis aggregate, mutators, invariants, NewDefaultAnalysis"
```

### Task B6: `projectAnalysis` seam + equivalence test

**Files:**
- Create: `ccx/project.go`
- Test: `ccx/project_test.go`

**Interfaces:**
- Consumes: `femmodel.Analysis` + accessors (B5); `ccx.StudySettings`, `ccx.defaultSettings()`, `ccx.AnalysisType`, `ccx.ElementOrder` (`LinearTet`/`QuadraticTet`), `ccx.ResultFieldKind`, `ccx.ConstraintSpec` (existing in package `ccx`).
- Produces: `ccx.projectAnalysis(a *femmodel.Analysis) (StudySettings, []ConstraintSpec)` — starts from `defaultSettings()` and overrides the Solver/Mesh/Material/Result fields the Phase-1 tree owns; the remaining flat fields keep their defaults until later phases migrate them. Returned specs are `s.Constraints` (empty in Phase 1).

- [ ] **Step 1: Write the failing test**

Create `ccx/project_test.go`:
```go
// SPDX-License-Identifier: GPL-2.0-only

package ccx

import (
	"reflect"
	"testing"

	"oblikovati.org/calculix/ccx/femmodel"
)

// The seam must preserve the v1 defaults exactly: projecting the default Analysis must reproduce
// defaultSettings() field-for-field. This is the safety net that lets later phases migrate fields
// into the tree one at a time without drifting the solve inputs.
func TestProjectDefaultAnalysisEqualsDefaultSettings(t *testing.T) {
	got, specs := projectAnalysis(femmodel.NewDefaultAnalysis())
	want := defaultSettings()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("projection drifted from defaults:\n got=%+v\nwant=%+v", got, want)
	}
	if len(specs) != 0 {
		t.Fatalf("expected no constraints from a default analysis, got %d", len(specs))
	}
}

// Overrides on the tree must flow through the projection.
func TestProjectAppliesTreeOverrides(t *testing.T) {
	a := femmodel.NewDefaultAnalysis()
	a.SetSolver(femmodel.SolverObject{AnalysisType: "frequency", Eigenmodes: 12, TransientTimeS: 0})
	a.SetMesh(femmodel.MeshObject{MaxSizeMM: 2.5, Quadratic: false})
	got, _ := projectAnalysis(a)
	if got.Analysis != AnalysisFrequency || got.Eigenmodes != 12 {
		t.Fatalf("solver override lost: %+v", got)
	}
	if got.MeshSizeMM != 2.5 || got.ElementOrder != LinearTet {
		t.Fatalf("mesh override lost: %+v", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./ccx/ -run 'TestProject' -v`
Expected: FAIL — `undefined: projectAnalysis`.

- [ ] **Step 3: Write minimal implementation**

Create `ccx/project.go`:
```go
// SPDX-License-Identifier: GPL-2.0-only

package ccx

import "oblikovati.org/calculix/ccx/femmodel"

// projectAnalysis flattens the femmodel.Analysis tree onto the engine's StudySettings + explicit
// constraint list — the seam that keeps the mature mesh/deck/solve/render pipeline unchanged while
// the edit model becomes a tree. It starts from the v1 defaults and overrides only the fields the
// Phase-1 tree owns (Solver/Mesh/Material/Result); fields not yet migrated keep their defaults, so
// projecting the default Analysis reproduces defaultSettings() exactly. Constraints are carried on
// StudySettings and returned alongside for callers that want them directly.
func projectAnalysis(a *femmodel.Analysis) (StudySettings, []ConstraintSpec) {
	s := defaultSettings()

	sv := a.Solver()
	s.Analysis = AnalysisType(sv.AnalysisType)
	s.Eigenmodes = sv.Eigenmodes
	s.TransientTimeS = sv.TransientTimeS

	m := a.Mesh()
	s.MeshSizeMM = m.MaxSizeMM
	s.ElementOrder = elementOrder(m.Quadratic)

	if mat, ok := a.DefaultMaterial(); ok {
		s.YoungGPa = mat.YoungGPa
		s.Poisson = mat.Poisson
		s.DensityGCm3 = mat.DensityGCm3
		s.YieldMPa = mat.YieldMPa
	}
	if r, ok := a.PrimaryResult(); ok {
		s.ResultField = ResultFieldKind(r.Field)
		s.DeformScale = r.DeformScale
	}
	return s, s.Constraints
}

// elementOrder maps the mesh object's Quadratic flag to the deck element order.
func elementOrder(quadratic bool) ElementOrder {
	if quadratic {
		return QuadraticTet
	}
	return LinearTet
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./ccx/ -run 'TestProject' -v` → PASS. Then `go test ./...` (whole add-in) → PASS (no existing test touched; nothing else changed).

- [ ] **Step 5: Commit**
```bash
git add ccx/project.go ccx/project_test.go
git commit -m "feat(ccx): projectAnalysis seam — femmodel.Analysis -> StudySettings + specs

Pure-additive: the engine still uses e.settings; the projection is proven by an
equivalence test (default Analysis -> defaultSettings) and wired into the engine in
Phase 2, when panel.go is retired."
```

### Task B7: lint + coverage gate (whole add-in)

**Files:** none (verification task).

- [ ] **Step 1: Run the full test suite**

Run: `cd /home/vmiguel/git/oblikovati-workspace/Oblikovati.AddIns.CalculiX && go test ./...`
Expected: PASS (existing 110 tests + the new femmodel/project tests).

- [ ] **Step 2: Run lint**

Run: `golangci-lint run` (funlen budget 20). Expected: clean (all new functions are ≤20 lines).

- [ ] **Step 3: Coverage on the new package**

Run: `go test ./ccx/femmodel/ -cover` and `go test ./ccx/ -run 'TestProject' -cover`
Expected: `femmodel` coverage > 80% (every function has a test).

- [ ] **Step 4: No commit** (verification only). If lint/coverage flags anything, fix inline with a follow-up TDD cycle and commit that.

---

## Self-Review (completed by plan author)

- **Spec coverage (Phase 0a + Phase 1):** A1–A5 implement the spec's A1 (`referenceList`) and A2 (`TaskPanelSpec`); A6 cuts the release. B1–B6 implement the spec's Pillar-B `femmodel` aggregate + `projectAnalysis` seam (ADR-B1). Spec items **deferred with rationale** (not gaps): the GPL-host rendering/router for the new API (spec's remainder of Phase 0a) → separate `Phase 0a-host` plan; the engine-ownership flip + `ConstraintObject` (spec put a flip in Phase 1) → moved to Phase 2 to avoid rewriting the soon-retired `panel.go` twice. The graphics API (A3–A5 of the spec) is Phase 0b, out of this plan's scope.
- **Placeholder scan:** none — every step has concrete code and exact commands/paths.
- **Type consistency:** `projectAnalysis` returns `(StudySettings, []ConstraintSpec)` matching the spec; `elementOrder(bool) ElementOrder` used consistently; `ResultObject.Field`/`SolverObject.AnalysisType` are the underlying strings of `ccx.ResultFieldKind`/`ccx.AnalysisType`, converted at the seam; ids (`solver`/`mesh`/`mat1`/`result1`) are unique and stable.

## Part B — execution outcome (2026-06-30)

Part B (Tasks B1–B7) executed subagent-driven, all reviews clean. Final whole-branch
review (opus): **READY WITH MINOR FOLLOW-UPS** — no Critical/Important; seam purity,
additive constraint, and full-`StudySettings` default round-trip verified field-by-field;
femmodel coverage 89.6%, 127 tests green, lint clean. Commits `1af7597..4d01d9f`.

**Deferred to Phase 2 (none merge-blocking, carried from per-task + final review):**

1. **Analysis construction contract / projection symmetry** — `projectAnalysis` reads
   `Solver()`/`Mesh()` unconditionally but guards `DefaultMaterial()`/`PrimaryResult()`
   with `ok`. A zero-value `&femmodel.Analysis{}` (constructable since fields are
   unexported) projects `AnalysisType("")` and `LinearTet` (flips the `QuadraticTet`
   default). Latent in Phase 1 (only `NewDefaultAnalysis` constructs). Fix: make the zero
   value obviously-uninitialized (sentinel) or guard solver/mesh symmetrically, when the
   engine flip lands.
2. **Compile-time interface assertions** — add `var _ FEMObject = (*SolverObject)(nil)`
   (etc.) for all four objects when wiring them into the engine/browser tree.
3. **Slice aliasing** — `Analysis.Materials()`/`Results()` return the backing slice, and
   `projectAnalysis` returns `s, s.Constraints` aliasing the struct's array. Harmless now
   (Phase-1 constraints are nil); add defensive copies when `ConstraintObject` +
   `Analysis.Constraints()` arrive and the tree populates constraints.
3. **Multi-material/result collapse** — projection takes only `DefaultMaterial()`/
   `PrimaryResult()`; grow the material seam for per-body + thermal/EM/hyperelastic before
   multi-material editing ships.
4. **Coverage gap** — directed tests for `DefaultMaterial()`'s ScopeAll-not-first and
   empty-materials branches (the 89.6%→100% gap).

The **engine ownership flip** (`Engine.analysis *femmodel.Analysis`, `RunStudyOnHost`
calling `projectAnalysis`, retiring `panel.go`) remains folded into **Phase 2** alongside
the browser tree, as planned.
