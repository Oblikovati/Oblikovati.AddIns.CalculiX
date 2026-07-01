<!-- SPDX-License-Identifier: GPL-2.0-only -->
# FEM Parity — Phase 2.3 (FEA ribbon) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Give the CalculiX commands a proper home — a dedicated **"FEA" ribbon tab** with grouped panels (Solve / Constraints / Windows) — mirroring the CAM add-in's `ribbon_layout.go` pattern, and add two **window commands** (re-open the study Panel / the Analysis Tree) so a closed panel/tree can be brought back.

**Architecture:** A `ccxRibbonSpots` map places each command on a panel of the FEA tab; a `commandArgs(id,name,tip)` merge builds the `wire.CreateCommandArgs` (Ribbon=Part, Tab="FEA", Category=panel, ButtonStyle); `RegisterCommands` loops a command list through it. Button **styles are set** (Run Study large, the rest small) but **`IconSVG` is left empty** — the add-in ships no embedded SVG glyph assets yet (unlike CAM), so the host renders each button by its caption until a later icon pass adds an `iconSVG` helper + assets. No engine-behavior change beyond the two new window commands routing to `ShowPanel`/`ShowAnalysisTree`.

**Tech Stack:** Go; `oblikovati.org/calculix` add-in; links only `oblikovati.org/api`.

## Global Constraints

- Every new `.go` file carries `// SPDX-License-Identifier: GPL-2.0-only`.
- Never re-declare a wire DTO/method string — import from `oblikovati.org/api/wire`/`types`.
- Style: functions 4–20 lines, files <500 lines, explicit types, early returns.
- **Goroutine discipline:** the new window commands make host calls — route them OFF the session goroutine (`go func(){ … }()`), like the existing `AddConstraint`/`Clear` commands in `onCommandStarted`.
- TDD; the pure `commandArgs`/`ccxRibbonSpots` are directly testable; full suite stays green.
- Run `go test ./...` + `golangci-lint run ./ccx/...` (watch for `unused`) + `gofmt -l` before each commit.
- Branch: `feature/fem-parity-phase2-3-fea-ribbon` (already off the merged `main`).

## Anchors (verified)

- `ccx/commands.go`: `RunStudyCommandID="CCX.RunStudy"`, `AddConstraintCommandID="CCX.AddConstraint"`, `ClearConstraintsCommandID="CCX.ClearConstraints"`; `RegisterCommands` does 3 inline `e.api.Commands().Create(wire.CreateCommandArgs{ID,DisplayName,Category,Tooltip})`.
- `ccx/engine.go` `onCommandStarted` switch routes `RunStudyCommandID`→`launchStudy`; `AddConstraintCommandID`→`go e.addConstraintFromSelection()`; `ClearConstraintsCommandID`→`go e.clearConstraints()`.
- `wire.CreateCommandArgs{ID, DisplayName, Tooltip string; Ribbon types.RibbonKey; Tab, Category string; IconSVG string; ButtonStyle types.ButtonStyle}`.
- `types.PartRibbon RibbonKey = "Part"`; `types.TextOnlyButton ButtonStyle = 0`, `SmallIconButton = 1`, `LargeIconButton = 2`.
- `ShowPanel()`/`ShowAnalysisTree()` exist (both host-calling → run off-goroutine).

## FEA tab layout

| Panel | Commands |
|---|---|
| **Solve** | Run Study (`CCX.RunStudy`) |
| **Constraints** | Add From Selection (`CCX.AddConstraint`) · Clear (`CCX.ClearConstraints`) |
| **Windows** | Study Panel (`CCX.ShowPanel`) · Analysis Tree (`CCX.ShowTree`) |

---

### Task R1: `ribbon_layout.go` — spot map + `commandArgs`

**Files:**
- Create: `ccx/ribbon_layout.go`
- Test: `ccx/ribbon_layout_test.go`

**Interfaces:**
- Produces: `const ccxRibbonTab = "FEA"`; `type ccxRibbonSpot struct{ panel string; style types.ButtonStyle }`; `var ccxRibbonSpots map[string]ccxRibbonSpot`; `commandArgs(id, name, tip string) wire.CreateCommandArgs` (merges Ribbon=Part, Tab=FEA, Category=spot.panel, ButtonStyle=spot.style; no IconSVG).

- [ ] **Step 1: Write the failing test**

Create `ccx/ribbon_layout_test.go`:
```go
// SPDX-License-Identifier: GPL-2.0-only

package ccx

import (
	"testing"

	"oblikovati.org/api/types"
)

func TestCommandArgsPlacesOnFEATab(t *testing.T) {
	a := commandArgs(RunStudyCommandID, "Run Study", "tip")
	if a.ID != RunStudyCommandID || a.DisplayName != "Run Study" || a.Tooltip != "tip" {
		t.Fatalf("identity fields wrong: %+v", a)
	}
	if a.Ribbon != types.PartRibbon || a.Tab != "FEA" || a.Category != "Solve" {
		t.Fatalf("placement wrong: ribbon=%q tab=%q cat=%q", a.Ribbon, a.Tab, a.Category)
	}
	if a.ButtonStyle != types.LargeIconButton {
		t.Fatalf("Run Study should be a large button, got %v", a.ButtonStyle)
	}
}

func TestEveryCommandHasARibbonSpot(t *testing.T) {
	for _, id := range []string{
		RunStudyCommandID, AddConstraintCommandID, ClearConstraintsCommandID,
		ShowPanelCommandID, ShowTreeCommandID,
	} {
		if _, ok := ccxRibbonSpots[id]; !ok {
			t.Errorf("command %q has no ribbon spot", id)
		}
	}
}
```
(This references `ShowPanelCommandID`/`ShowTreeCommandID` — declared in R2. To keep R1 self-contained, either declare those two consts in R1's `ribbon_layout.go` alongside the spots, or split `TestEveryCommandHasARibbonSpot` into R2. RECOMMENDED: declare all five command-id consts that the spot map keys on in `commands.go` as part of R1's prerequisite — add `ShowPanelCommandID = "CCX.ShowPanel"` and `ShowTreeCommandID = "CCX.ShowTree"` to `commands.go` in R1 so the map + test compile. Their *routing* is added in R2.)

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /home/vmiguel/git/oblikovati-workspace/Oblikovati.AddIns.CalculiX && go test ./ccx/ -run 'TestCommandArgs|TestEveryCommandHasARibbonSpot'`
Expected: FAIL — `undefined: commandArgs` / `ccxRibbonSpots`.

- [ ] **Step 3: Write minimal implementation**

In `ccx/commands.go`, add the two window command-id consts (routing comes in R2):
```go
// ShowPanelCommandID / ShowTreeCommandID re-open the study panel / Analysis tree from the ribbon.
const (
	ShowPanelCommandID = "CCX.ShowPanel"
	ShowTreeCommandID  = "CCX.ShowTree"
)
```
Create `ccx/ribbon_layout.go`:
```go
// SPDX-License-Identifier: GPL-2.0-only

package ccx

import (
	"oblikovati.org/api/types"
	"oblikovati.org/api/wire"
)

// ccxRibbonTab is the CalculiX add-in's dedicated document ribbon tab.
const ccxRibbonTab = "FEA"

// ccxRibbonSpot places one command on a panel of the FEA tab, with its button style. Buttons are
// text-only for now (the add-in ships no glyph assets yet); an icon pass is a later follow-up.
type ccxRibbonSpot struct {
	panel string
	style types.ButtonStyle
}

// ccxRibbonSpots places every CalculiX command on a panel of the FEA tab. Kept exhaustive so a
// command can never land on an unnamed panel — guarded by TestEveryCommandHasARibbonSpot.
var ccxRibbonSpots = map[string]ccxRibbonSpot{
	RunStudyCommandID:         {"Solve", types.LargeIconButton},
	AddConstraintCommandID:    {"Constraints", types.LargeIconButton},
	ClearConstraintsCommandID: {"Constraints", types.SmallIconButton},
	ShowPanelCommandID:        {"Windows", types.SmallIconButton},
	ShowTreeCommandID:         {"Windows", types.SmallIconButton},
}

// commandArgs builds the host command-registration args, placing the command on its FEA-tab panel.
func commandArgs(id, name, tip string) wire.CreateCommandArgs {
	spot := ccxRibbonSpots[id]
	return wire.CreateCommandArgs{
		ID: id, DisplayName: name, Tooltip: tip,
		Ribbon: types.PartRibbon, Tab: ccxRibbonTab, Category: spot.panel, ButtonStyle: spot.style,
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./ccx/ -run 'TestCommandArgs|TestEveryCommandHasARibbonSpot' -v` → PASS.

- [ ] **Step 5: Commit**
```bash
git add ccx/ribbon_layout.go ccx/ribbon_layout_test.go ccx/commands.go
git commit -m "feat(ccx): FEA ribbon-tab spot map + commandArgs placement"
```

---

### Task R2: loop `RegisterCommands` + wire the window commands

**Files:**
- Modify: `ccx/commands.go` (`RegisterCommands` loops a `ccxCommands` list through `commandArgs`)
- Modify: `ccx/engine.go` (`onCommandStarted` routes `ShowPanelCommandID`/`ShowTreeCommandID`)
- Test: extend `ccx/engine_test.go`

**Interfaces:**
- Consumes: `commandArgs` (R1), `ShowPanel`/`ShowAnalysisTree`.
- Produces: a `ccxCommands []struct{ id, name, tip string }` looped through `commandArgs`; `onCommandStarted` opens the panel/tree for the two window commands (off-goroutine).

- [ ] **Step 1: Write the failing test**

In `ccx/engine_test.go`, add:
```go
func TestRegisteredCommandsLandOnFEATab(t *testing.T) {
	h := &recordingHost{}
	if err := NewEngine(h).RegisterCommands(); err != nil {
		t.Fatalf("RegisterCommands: %v", err)
	}
	got := h.createdCommandTabs() // helper: decode every MethodCommandsCreate payload → map[id]tab
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
	e.onCommandStarted(commandStartedEvent(ShowTreeCommandID)) // reuse the existing test event builder
	waitFor(t, func() bool { return h.saw(wire.MethodBrowserSetPane) })
}
```
NOTE: `recordingHost.createdCommandTabs()` and `commandStartedEvent(id)` — check whether the test file already has a helper that decodes recorded `MethodCommandsCreate` payloads and one that builds a `command.started` event (grep `engine_test.go` for how `TestNotifyRoutesBrowserNode`/`TestSetupRegisters` build events + inspect recorded calls). Reuse the REAL helpers; add a small `createdCommandTabs` decoder only if none exists (decode each recorded `CreateCommandArgs` and map `ID→Tab`). Use the suite's existing `waitFor`.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./ccx/ -run 'TestRegisteredCommandsLandOnFEATab|TestShowTreeCommandReopensTree'`
Expected: FAIL — commands still use the old inline `Category:"CalculiX"` (no Tab); `ShowTree` command not routed.

- [ ] **Step 3: Write minimal implementation**

Replace `RegisterCommands` in `ccx/commands.go`:
```go
// ccxCommands is the exhaustive command list; RegisterCommands places each on the FEA tab.
var ccxCommands = []struct{ id, name, tip string }{
	{RunStudyCommandID, "Run Stress Analysis", "Mesh, solve, and visualize the stress and displacement of the active part with CalculiX."},
	{AddConstraintCommandID, "Add Constraint From Selection", "Add the selected face(s) as a study constraint of the chosen type."},
	{ClearConstraintsCommandID, "Clear Constraints", "Remove all study constraints added from selection."},
	{ShowPanelCommandID, "Study Panel", "Open the CalculiX study-parameters panel."},
	{ShowTreeCommandID, "Analysis Tree", "Open the CalculiX Analysis browser tree."},
}

// RegisterCommands registers every CalculiX command on the FEA ribbon tab (also invokable over
// the MCP bridge's execute_command). Command actions fire command.started, which Notify dispatches.
func (e *Engine) RegisterCommands() error {
	for _, c := range ccxCommands {
		if _, err := e.api.Commands().Create(commandArgs(c.id, c.name, c.tip)); err != nil {
			return err
		}
	}
	return nil
}
```
In `ccx/engine.go` `onCommandStarted` switch, add:
```go
	case ShowPanelCommandID:
		go func() { _, _ = e.ShowPanel() }()
	case ShowTreeCommandID:
		go func() { _, _ = e.ShowAnalysisTree() }()
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./ccx/ -run 'TestRegisteredCommandsLandOnFEATab|TestShowTreeCommandReopensTree' -v` → PASS. Then `go test ./...` → all green (the existing `TestSetupRegistersCommandAndPanel` still passes — it asserts the *methods*, which are unchanged).

- [ ] **Step 5: Commit**
```bash
git add ccx/commands.go ccx/engine.go ccx/engine_test.go
git commit -m "feat(ccx): register all commands on the FEA tab + wire Show Panel/Tree buttons"
```

---

### Task R3: verification gate

- [ ] **Step 1:** `go test ./...` — all green.
- [ ] **Step 2:** `golangci-lint run ./ccx/...` clean (watch `unused`); `gofmt -l ccx/` empty.
- [ ] **Step 3 (live, best-effort):** via the MCPBridge live-head recipe, confirm an **FEA** tab appears on the Part ribbon with Solve / Constraints / Windows panels; click **Run Study** and **Analysis Tree**; `capture_window`. Otherwise defer to a combined 2.2/2.3 live check. No throwaway drivers committed.
- [ ] **Step 4:** No commit (verification). Fix gaps via a focused TDD cycle.

---

## Self-Review (completed by plan author)

- **Spec coverage:** implements Phase-2 slice 2.3 (FEA ribbon) per the architecture brief: `ccxRibbonSpots` + `commandArgs` (R1), looped `RegisterCommands` + the two window commands (R2). Icons deferred (no asset pipeline yet) — buttons carry large/small styles with an empty `IconSVG`, which the host renders as captioned buttons.
- **Placeholder scan:** the only "locate" directives are "reuse the existing event-builder / recorded-call decoder test helpers" — explicit reuse instructions naming the file to grep. The spot map + `commandArgs` + `RegisterCommands` loop are fully coded.
- **Type consistency:** `commandArgs(id,name,tip)`, `ccxRibbonSpots`, `ShowPanelCommandID`/`ShowTreeCommandID` used identically across R1/R2; `types.PartRibbon`/`TextOnlyButton`/`Large`/`SmallIconButton` are the real enum values.
- **Lint watch:** R1 declares `ShowPanelCommandID`/`ShowTreeCommandID` (used by the spot map immediately) and R2 adds their routing in the same slice, so no `unused` window-command const escapes (the 2.2 lesson).

## Next slice
- **2.4+** — field-group migrations (materials-thermal → EM → contact → constraints-into-femmodel → results), each moving a group from `extras` into typed `femmodel` objects, adding its tree node menu + create command, and deleting its panel section — guarded by the equivalence test — until `extras` is empty and `panel.go` retires. Also: an icon pass (embed SVG glyphs + `iconSVG` helper) to upgrade the text buttons.
