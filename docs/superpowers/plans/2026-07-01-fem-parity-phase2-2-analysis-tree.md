<!-- SPDX-License-Identifier: GPL-2.0-only -->
# FEM Parity — Phase 2.2 (Analysis browser tree) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a read-only **Analysis browser tree** (Analysis ▸ Solver / Mesh / Materials / Constraints / Results) that reflects the add-in's `femmodel.Analysis` (source of truth after slice 2.1), with context menus and double-click routing to the **existing dockable panel** and the existing commands — the first *visible* step of FreeCAD-FEM parity.

**Architecture:** Per the Phase-2 brief (ADR-3: non-modal — no dependency on the still-pending host modal task panel). Mirror the CAM add-in's proven tree pattern (`browser_tree.go`/`browser_tree_events.go`): a `BrowserPaneSpec` built from the aggregate, a stable `kind:index` node-id scheme, `EventBrowserNode` routing with `double`/`menu` gestures, and a `runAndRefresh` re-render helper. The tree is a **synchronous projection** of the aggregate — no new state, no engine-behavior change beyond adding the pane + routing.

**Tech Stack:** Go; `oblikovati.org/calculix` add-in; links only `oblikovati.org/api`.

## Global Constraints

- Every new `.go` file carries `// SPDX-License-Identifier: GPL-2.0-only`.
- `ccx → femmodel` only; the tree builder reads `femmodel` accessors, never mutates the domain outside its mutators.
- Style: functions 4–20 lines, files <500 lines, explicit types (no `any`), early returns.
- **Goroutine discipline:** `Notify` runs on the host session goroutine; any handler that makes host calls (open panel, run study, re-render tree) must run OFF it — reuse the existing `go e.…()` / `launchStudy` coalescing pattern (`engine.go`). A pure aggregate read (building nodes) may run inline under `e.mu`, but the `Browser().SetPane` host call must be OUTSIDE the lock.
- Nodes omit `IconSVG` (there is no `iconSVG` helper in `ccx`; the field is optional — a later slice can add glyphs).
- TDD for the pure builders + routing (recordingHost fake); the pane rendering is asserted via `recordingHost.saw(wire.MethodBrowserSetPane)`.
- Run `go test ./...` + `golangci-lint run ./ccx/...` + `gofmt -l` before each commit.
- Branch: `feature/fem-parity-phase2-2-analysis-tree` (already created off the merged `main`).

## Node-id scheme (stable handles; parsed in events)

```
analysis                 root — menu: Run Study; double-click: open panel
├─ solver                menu: Edit (→ panel)
├─ mesh                  menu: Edit (→ panel)
├─ materials             category — menu: (none v2.2); children mat:N
│   └─ mat:N             menu: Edit (→ panel)
├─ constraints           category — menu: Add From Selection · Clear; children con:N
│   └─ con:N             menu: Delete-not-yet (v2.2: none)
└─ results               category; children result:N
    └─ result:N          menu: Edit (→ panel)
```
Format with `fmt.Sprintf("mat:%d", i)`; parse with `fmt.Sscanf(node, "mat:%d", &i)` (mirror CAM `opIndexOf`). Singletons/categories (`analysis`/`solver`/`mesh`/`materials`/`constraints`/`results`) are string-equality matches. Every non-category node double-clicks to the existing panel (`ShowPanel`); the only menu *actions* in 2.2 are Run Study (analysis root) and Add/Clear constraints (existing commands) — everything else opens the panel. This honors ADR-3 (no modal editors yet).

---

### Task T1: `analysis_tree.go` — build + show the pane

**Files:**
- Create: `ccx/analysis_tree.go`
- Test: `ccx/analysis_tree_test.go`

**Interfaces:**
- Consumes: `e.api.Browser().SetPane`, `e.analysis` (`femmodel` accessors `Solver()/Mesh()/Materials()/Results()`), `e.extras.Constraints`, `e.mu`.
- Produces: `const AnalysisBrowserPaneID = "com.oblikovati.calculix.tree"`; `(*Engine).ShowAnalysisTree() (wire.OKResult, error)` (snapshot under lock, build, `SetPane` outside lock); pure builder `analysisNodes(a *femmodel.Analysis, cons []ConstraintSpec) []wire.BrowserNodeSpec`.

- [ ] **Step 1: Write the failing test**

Create `ccx/analysis_tree_test.go`:
```go
// SPDX-License-Identifier: GPL-2.0-only

package ccx

import (
	"testing"

	"oblikovati.org/calculix/ccx/femmodel"
)

func TestAnalysisNodesReflectAggregate(t *testing.T) {
	a := femmodel.NewDefaultAnalysis()
	nodes := analysisNodes(a, nil)
	if len(nodes) != 1 || nodes[0].ID != "analysis" {
		t.Fatalf("want single 'analysis' root, got %+v", nodes)
	}
	kids := childIDs(nodes[0].Children)
	for _, want := range []string{"solver", "mesh", "materials", "constraints", "results"} {
		if !contains(kids, want) {
			t.Fatalf("root missing %q child; got %v", want, kids)
		}
	}
	// One default material + one default result appear as leaves.
	mats := findChild(nodes[0].Children, "materials")
	if len(mats.Children) != 1 || mats.Children[0].ID != "mat:0" {
		t.Fatalf("want one mat:0 leaf, got %+v", mats.Children)
	}
}

func TestAnalysisNodesListConstraints(t *testing.T) {
	a := femmodel.NewDefaultAnalysis()
	cons := []ConstraintSpec{fixedSpecForTest(), fixedSpecForTest()}
	nodes := analysisNodes(a, cons)
	cn := findChild(nodes[0].Children, "constraints")
	if len(cn.Children) != 2 || cn.Children[1].ID != "con:1" {
		t.Fatalf("want two constraint leaves con:0/con:1, got %+v", cn.Children)
	}
}

// --- tiny test helpers (keep in this file) ---
func childIDs(ns []wire.BrowserNodeSpec) []string {
	out := make([]string, len(ns))
	for i, n := range ns {
		out[i] = n.ID
	}
	return out
}
func contains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}
func findChild(ns []wire.BrowserNodeSpec, id string) wire.BrowserNodeSpec {
	for _, n := range ns {
		if n.ID == id {
			return n
		}
	}
	return wire.BrowserNodeSpec{}
}
```
NOTE: `fixedSpecForTest()` — find how existing tests build a `ConstraintSpec` (grep `ccx/*_test.go` for a `FixedSpec`/`newConstraintSpec` construction, e.g. in `constraintbuilder_test.go`) and reuse that constructor. If none is trivially reusable, build the simplest valid `ConstraintSpec` the package already exposes. The test only needs `len(cons)` reflected, so any two specs work.

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /home/vmiguel/git/oblikovati-workspace/Oblikovati.AddIns.CalculiX && go test ./ccx/ -run TestAnalysisNodes`
Expected: FAIL — `undefined: analysisNodes`.

- [ ] **Step 3: Write minimal implementation**

Create `ccx/analysis_tree.go`:
```go
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
	nodes := analysisNodes(e.analysis, e.extras.Constraints)
	e.mu.Unlock()
	return e.api.Browser().SetPane(wire.BrowserPaneSpec{
		ID: AnalysisBrowserPaneID, Title: "Analysis", Nodes: nodes,
	})
}

// analysisNodes projects the aggregate (+ current constraint list) into the tree — pure and
// directly testable.
func analysisNodes(a *femmodel.Analysis, cons []ConstraintSpec) []wire.BrowserNodeSpec {
	return []wire.BrowserNodeSpec{{
		ID: "analysis", Label: "Analysis", Expanded: true, Menu: analysisRootMenu(),
		Children: []wire.BrowserNodeSpec{
			{ID: "solver", Label: "Solver: " + a.Solver().AnalysisType, Menu: editMenu()},
			{ID: "mesh", Label: "Mesh", Menu: editMenu()},
			materialsNode(a.Materials()),
			constraintsNode(cons),
			resultsNode(a.Results()),
		},
	}}
}

func materialsNode(mats []femmodel.MaterialObject) wire.BrowserNodeSpec {
	kids := make([]wire.BrowserNodeSpec, len(mats))
	for i, m := range mats {
		kids[i] = wire.BrowserNodeSpec{ID: fmt.Sprintf("mat:%d", i), Label: m.Name(), Menu: editMenu()}
	}
	return wire.BrowserNodeSpec{ID: "materials", Label: "Materials", Expanded: true, Children: kids}
}

func constraintsNode(cons []ConstraintSpec) wire.BrowserNodeSpec {
	kids := make([]wire.BrowserNodeSpec, len(cons))
	for i := range cons {
		kids[i] = wire.BrowserNodeSpec{ID: fmt.Sprintf("con:%d", i), Label: fmt.Sprintf("Constraint %d", i+1)}
	}
	return wire.BrowserNodeSpec{ID: "constraints", Label: "Constraints & Loads", Expanded: true,
		Menu: constraintsMenu(), Children: kids}
}

func resultsNode(results []femmodel.ResultObject) wire.BrowserNodeSpec {
	kids := make([]wire.BrowserNodeSpec, len(results))
	for i, r := range results {
		kids[i] = wire.BrowserNodeSpec{ID: fmt.Sprintf("result:%d", i), Label: r.Field, Menu: editMenu()}
	}
	return wire.BrowserNodeSpec{ID: "results", Label: "Results", Expanded: true, Children: kids}
}

func analysisRootMenu() []wire.BrowserMenuItem {
	return []wire.BrowserMenuItem{{ID: "run", Label: "Run Study"}}
}
func constraintsMenu() []wire.BrowserMenuItem {
	return []wire.BrowserMenuItem{{ID: "add", Label: "Add From Selection"}, {ID: "clear", Label: "Clear"}}
}
func editMenu() []wire.BrowserMenuItem {
	return []wire.BrowserMenuItem{{ID: "edit", Label: "Edit…"}}
}
```
(`MaterialObject.Name()`, `ResultObject.Field`, `SolverObject.AnalysisType` are exported per femmodel — confirm the accessor names as you go. If `Materials()`/`Results()` return values without an exported label, use what exists.)

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./ccx/ -run TestAnalysisNodes -v` → PASS.

- [ ] **Step 5: Commit**
```bash
git add ccx/analysis_tree.go ccx/analysis_tree_test.go
git commit -m "feat(ccx): build the Analysis browser tree from the aggregate (read-only)"
```

---

### Task T2: `analysis_tree_events.go` — route double-click + menu

**Files:**
- Create: `ccx/analysis_tree_events.go`
- Test: `ccx/analysis_tree_events_test.go`

**Interfaces:**
- Consumes: `ShowPanel`, `launchStudy`, `addConstraintFromSelection`, `clearConstraints`, `ShowAnalysisTree`, the existing `go e.…()` off-goroutine pattern.
- Produces: `(*Engine).handleAnalysisNode(node, gesture, menuItem string)` (dispatch `double`/`menu`); `matIndexOf`/`conIndexOf`/`resultIndexOf(node string) (int, bool)` parsers; `(*Engine).runAndRefreshAnalysisTree(action func())` (run off-goroutine, then re-declare the pane).

- [ ] **Step 1: Write the failing test**

Create `ccx/analysis_tree_events_test.go`:
```go
// SPDX-License-Identifier: GPL-2.0-only

package ccx

import "testing"

func TestConIndexOf(t *testing.T) {
	if i, ok := conIndexOf("con:3"); !ok || i != 3 {
		t.Fatalf("conIndexOf(con:3) = %d,%v", i, ok)
	}
	if _, ok := conIndexOf("constraints"); ok {
		t.Fatal("category node must not parse as con:N")
	}
}

func TestAnalysisMenuRunRoutesToStudy(t *testing.T) {
	h := &recordingHost{}
	e := NewEngine(h)
	// A menu "run" on the analysis root must trigger a study attempt (host calls happen
	// off-goroutine; wait for the coalescing guard to have run).
	e.handleAnalysisNode("analysis", "menu", "run")
	waitFor(t, func() bool { return e.busy() || h.saw(wire.MethodStatusSetText) })
}
```
NOTE: replace `e.busy()`/`waitFor`/the exact "study ran" signal with whatever the existing async tests use to observe a launched study (grep `ccx/*_test.go` for how a `launchStudy`/`go e.…` result is awaited — e.g. polling `e.running` under lock, or asserting a recorded host method like `MethodStatusSetText` that `reportStatus` calls). Use the REAL observation the suite already uses; do not invent `busy()` if a different accessor exists. The point: assert the "run" menu actually launches the study path, not just that it returns.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./ccx/ -run 'TestConIndexOf|TestAnalysisMenuRun'`
Expected: FAIL — `undefined: conIndexOf` / `handleAnalysisNode`.

- [ ] **Step 3: Write minimal implementation**

Create `ccx/analysis_tree_events.go`:
```go
// SPDX-License-Identifier: GPL-2.0-only

package ccx

import "fmt"

// handleAnalysisNode routes a browser gesture on the Analysis pane: double-click opens the
// existing study panel; a context-menu item runs its action. (ADR-3: editing stays in the
// existing dockable panel until the host modal task panel lands.)
func (e *Engine) handleAnalysisNode(node, gesture, menuItem string) {
	switch gesture {
	case "double":
		go func() { _, _ = e.ShowPanel() }()
	case "menu":
		e.analysisMenu(node, menuItem)
	}
}

func (e *Engine) analysisMenu(node, item string) {
	switch {
	case node == "analysis" && item == "run":
		e.launchStudy()
	case node == "constraints" && item == "add":
		e.runAndRefreshAnalysisTree(func() { e.addConstraintFromSelection() })
	case node == "constraints" && item == "clear":
		e.runAndRefreshAnalysisTree(func() { e.clearConstraints() })
	default: // solver/mesh/mat/result "edit" → open the panel
		go func() { _, _ = e.ShowPanel() }()
	}
}

// runAndRefreshAnalysisTree runs a mutating action off the session goroutine, then re-declares
// the pane so the tree reflects the new state.
func (e *Engine) runAndRefreshAnalysisTree(action func()) {
	go func() {
		action()
		_, _ = e.ShowAnalysisTree()
	}()
}

func matIndexOf(node string) (int, bool)    { return indexOf(node, "mat:%d") }
func conIndexOf(node string) (int, bool)    { return indexOf(node, "con:%d") }
func resultIndexOf(node string) (int, bool) { return indexOf(node, "result:%d") }

func indexOf(node, format string) (int, bool) {
	var i int
	if _, err := fmt.Sscanf(node, format, &i); err == nil {
		return i, true
	}
	return 0, false
}
```
(If `addConstraintFromSelection`/`clearConstraints` already run their own `ShowPanel`/refresh or already spawn a goroutine, do NOT double-wrap — call them consistently with how `onCommandStarted` invokes them today. Adjust the wrapping to match the existing signatures: they may return values or take no args — read `constraintbuilder.go`.)

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./ccx/ -run 'TestConIndexOf|TestAnalysisMenuRun' -v` → PASS.

- [ ] **Step 5: Commit**
```bash
git add ccx/analysis_tree_events.go ccx/analysis_tree_events_test.go
git commit -m "feat(ccx): route Analysis-tree double-click/menu to panel + existing commands"
```

---

### Task T3: wire into `Notify` + `Setup`

**Files:**
- Modify: `ccx/engine.go` (`Notify` switch: add `case wire.EventBrowserNode`; decode + gate on pane id + dispatch)
- Modify: `ccx/commands.go` (`Setup` also calls `ShowAnalysisTree`)
- Test: extend `ccx/engine_test.go` (`TestSetupRegistersCommandAndPanel` now also expects `wire.MethodBrowserSetPane`)

**Interfaces:**
- Consumes: `handleAnalysisNode` (T2), `ShowAnalysisTree` (T1), `AnalysisBrowserPaneID`.
- Produces: `EventBrowserNode` routed to `handleAnalysisNode` (gated on `pane == AnalysisBrowserPaneID`); the tree declared at `Setup`.

- [ ] **Step 1: Write the failing test**

In `ccx/engine_test.go`, extend the Setup assertion to include the pane:
```go
	for _, m := range []string{wire.MethodCommandsCreate, wire.MethodDockableWindowsSet, wire.MethodBrowserSetPane} {
```
Add a routing test:
```go
func TestNotifyRoutesBrowserNode(t *testing.T) {
	h := &recordingHost{}
	e := NewEngine(h)
	ev := []byte(`{"type":"browser.node","pane":"com.oblikovati.calculix.tree","node":"analysis","gesture":"menu","menuItem":"run"}`)
	e.Notify(ev)
	waitFor(t, func() bool { return h.saw(wire.MethodStatusSetText) }) // study path reports status
}
```
(Use the value of `wire.EventBrowserNode`/`AnalysisBrowserPaneID` — the literal above must match them; and the same async-observation helper as T2.)

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./ccx/ -run 'TestSetupRegisters|TestNotifyRoutesBrowserNode'`
Expected: FAIL — Setup never called `browser.setPane`; browser event not routed.

- [ ] **Step 3: Write minimal implementation**

In `ccx/engine.go` `Notify` switch, add after the `EventPanelValueChanged` case:
```go
	case wire.EventBrowserNode:
		e.onBrowserNode(ev)
```
Add:
```go
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
```
In `ccx/commands.go` `Setup`, after `ShowPanel()`:
```go
	if _, err := e.ShowPanel(); err != nil {
		return err
	}
	_, err := e.ShowAnalysisTree()
	return err
```
(Adjust to the existing `Setup` control flow — it currently returns `e.ShowPanel()`'s error directly; restructure so BOTH panel and tree are shown and the first error is returned.)

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./ccx/ -run 'TestSetupRegisters|TestNotifyRoutesBrowserNode' -v` → PASS. Then `go test ./...` → all green.

- [ ] **Step 5: Commit**
```bash
git add ccx/engine.go ccx/commands.go ccx/engine_test.go
git commit -m "feat(ccx): declare the Analysis tree at Setup + route EventBrowserNode"
```

---

### Task T4: verification gate + live check

- [ ] **Step 1:** `go test ./...` — all green (femmodel + ccx).
- [ ] **Step 2:** `golangci-lint run ./ccx/...` clean; `gofmt -l ccx/` empty.
- [ ] **Step 3:** Coverage: `go test ./ccx/ -run 'TestAnalysisNodes|TestConIndexOf|TestNotifyRoutesBrowserNode' -cover` — the new builders/events covered.
- [ ] **Step 4 (live, best-effort):** if the MCPBridge live-head recipe is available, run a study add-in session, confirm the "Analysis" pane appears in the browser with the Analysis ▸ Solver/Mesh/Materials/Constraints/Results nodes, right-click "Run Study" on the root, and `capture_window`. Otherwise note this as deferred to a combined 2.2/2.3 live test. Do NOT commit throwaway drivers.
- [ ] **Step 5:** No commit (verification). Fix gaps via a focused TDD cycle.

---

## Self-Review (completed by plan author)

- **Spec coverage:** implements Phase-2 slice 2.2 (read-only Analysis tree) per the architecture brief: pane built from the aggregate (T1), node-id scheme + `double`/`menu` routing to the existing panel + the 3 existing commands (T2), `Notify`/`Setup` wiring (T3). No modal dependency (ADR-3). No engine-behavior change (the tree is a projection).
- **Placeholder scan:** the only non-literal directives are "reuse the existing constraint-spec test constructor" and "use the suite's real async-observation helper" — both are explicit *locate-and-reuse* instructions naming the file to read, not TODOs. The node-id scheme + builders are fully coded.
- **Type consistency:** `analysisNodes(a, cons)`, `ShowAnalysisTree`, `handleAnalysisNode(node,gesture,menuItem)`, `matIndexOf/conIndexOf/resultIndexOf`, `AnalysisBrowserPaneID` used identically across T1–T3; gesture strings `"double"`/`"menu"` match the wire contract.

## Next slice
- **2.3** — FEA ribbon (`ribbon_layout.go`; `ccxRibbonSpots`; looped `RegisterCommands`) placing the commands on an "FEA" tab. Then 2.4+ field-group migrations (each shrinking `extras` + the panel, guarded by the equivalence test) until the god panel retires.
