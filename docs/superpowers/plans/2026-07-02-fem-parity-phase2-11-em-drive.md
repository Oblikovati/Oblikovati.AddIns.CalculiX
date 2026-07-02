<!-- SPDX-License-Identifier: GPL-2.0-only -->
# FEM Parity Phase 2.11 — field-drive (EM) group → aggregate

> **For agentic workers:** REQUIRED SUB-SKILL: superpowers:subagent-driven-development.
> Steps use checkbox (`- [ ]`) syntax.

**Goal:** Move the 3 electromagnetic field-drive fields (`VoltageV`, `EMDriveMode`,
`CurrentDensity`) out of `StudySettings.extras` into the `femmodel.Analysis` aggregate as an
`EMDefaults` template, and delete the now-empty legacy helper `applyEMEdit`. This is the LAST
field migration — after it, `ccx/panel.go` performs zero `e.extras.*` writes, which unblocks
2.12 (dropping the `extras` argument and retiring `panel.go`).

**Architecture:** Strangler migration — same aggregate-template pattern as 2.8/2.9/2.10.
`Analysis` gains `EMDefaults` (`EM()`/`SetEM()`), seeded in `NewDefaultAnalysis`.
`projectAnalysis` re-flattens via `overlayEM`. The 3 panel controls route to `applyAggEMEdit`,
which becomes the terminal branch of `applyLoadEdit`; the old `applyEMEdit` is deleted.

**Tech Stack:** Go; `ccx/femmodel` pure (stdlib only — the EM drive mode is a plain `string`
there); `ccx` links only `oblikovati.org/api`.

## Global Constraints

- `ccx/femmodel` imports ONLY stdlib. In femmodel, the EM drive mode is a plain `string`.
- Equivalence guard `TestProjectDefaultAnalysisEqualsDefaultSettings`
  (`reflect.DeepEqual(projectAnalysis(NewDefaultAnalysis(), defaultSettings()), defaultSettings())`)
  MUST stay green at every commit.
- Seed values MUST equal `defaultSettings()`: `VoltageV=5`, `EMDriveMode="voltage"` (`EMVoltage`),
  `CurrentDensity=1`.
- `applyPanelEdit` already holds `e.mu.Lock(); defer Unlock()` before dispatching — the new
  helper runs under that lock; do NOT add a second lock.
- Style: golangci funlen 30 lines / 20 statements; no `any`; explicit types; functions ≤20
  lines; every new `.go` carries `// SPDX-License-Identifier: GPL-2.0-only`.
- After migration: NO `e.extras.{VoltageV|EMDriveMode|CurrentDensity}` write remains in
  `ccx/panel.go`; `applyEMEdit` deleted with no dangling caller; `grep 'e.extras.' ccx/panel.go`
  returns ONLY reads (if any) — no assignments.

---

### Task E1: `femmodel.EMDefaults` template on `Analysis`

**Files:**
- Create: `ccx/femmodel/em_defaults.go`
- Modify: `ccx/femmodel/analysis.go` (add `em EMDefaults`; `EM()`/`SetEM()`; seed)
- Test: `ccx/femmodel/em_defaults_test.go`

**Interfaces:**
- Produces: `type EMDefaults struct { EMDriveMode string; VoltageV, CurrentDensity float64 }`;
  `func (a *Analysis) EM() EMDefaults`; `func (a *Analysis) SetEM(EMDefaults)`.

- [ ] **Step 1: Write the failing test** (`ccx/femmodel/em_defaults_test.go`)

```go
// SPDX-License-Identifier: GPL-2.0-only
package femmodel

import "testing"

func TestDefaultEM(t *testing.T) {
	em := NewDefaultAnalysis().EM()
	if em.EMDriveMode != "voltage" {
		t.Fatalf("EMDriveMode = %q, want \"voltage\"", em.EMDriveMode)
	}
	if em.VoltageV != 5 || em.CurrentDensity != 1 {
		t.Fatalf("EM magnitudes = {%v %v}, want {5 1}", em.VoltageV, em.CurrentDensity)
	}
}

func TestSetEM(t *testing.T) {
	a := NewDefaultAnalysis()
	a.SetEM(EMDefaults{EMDriveMode: "current", VoltageV: 12, CurrentDensity: 7})
	got := a.EM()
	if got.EMDriveMode != "current" || got.VoltageV != 12 || got.CurrentDensity != 7 {
		t.Fatalf("EM() = %+v, want {current 12 7}", got)
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./ccx/femmodel/ -run 'TestDefaultEM|TestSetEM' -v`
Expected: FAIL — `EMDefaults`/`EM`/`SetEM` undefined.

- [ ] **Step 3: Create `ccx/femmodel/em_defaults.go`**

```go
// SPDX-License-Identifier: GPL-2.0-only
package femmodel

// EMDefaults holds the electromagnetic (electric-conduction) field-drive parameters synthesized
// at solve time. It is a study-wide template (not a browser-tree node), mirroring LoadDefaults,
// SupportDefaults, and ThermalDefaults. EMDriveMode is a neutral string here — the ccx layer maps
// it to its EMDrive display enum (voltage vs current).
type EMDefaults struct {
	EMDriveMode    string  // how the study is driven: applied "voltage" vs injected "current"
	VoltageV       float64 // prescribed potential on the first face (V) for the voltage mode
	CurrentDensity float64 // injected current density on the loaded faces for the current mode
}
```

- [ ] **Step 4: Modify `ccx/femmodel/analysis.go`** — add field, accessors, seed.

Add to the `Analysis` struct (beside `thermal ThermalDefaults`):

```go
	em EMDefaults
```

Add accessors (beside `Thermal`/`SetThermal`):

```go
// EM returns the electromagnetic field-drive parameters.
func (a *Analysis) EM() EMDefaults { return a.em }

// SetEM replaces the electromagnetic field-drive parameters.
func (a *Analysis) SetEM(e EMDefaults) { a.em = e }
```

Seed in `NewDefaultAnalysis` (right after the `a.SetThermal(...)` call):

```go
	a.SetEM(EMDefaults{EMDriveMode: "voltage", VoltageV: 5, CurrentDensity: 1})
```

- [ ] **Step 5: Run to verify pass**

Run: `go test ./ccx/femmodel/ -run 'TestDefaultEM|TestSetEM' -v` → PASS.
Then `go test ./ccx/femmodel/` → PASS.

- [ ] **Step 6: Commit**

```bash
git add ccx/femmodel/em_defaults.go ccx/femmodel/em_defaults_test.go ccx/femmodel/analysis.go
git commit -m "feat(femmodel): EMDefaults template on Analysis (EM/SetEM + seed)"
```

---

### Task E2: overlay + re-route the 3 EM controls; delete `applyEMEdit`

**Files:**
- Modify: `ccx/project.go` (add `overlayEM(a, s) StudySettings`; call in `projectAnalysis`)
- Modify: `ccx/panel.go` (add `applyAggEMEdit`; delete `applyEMEdit`; make it the terminal branch of `applyLoadEdit`)
- Test: `ccx/panel_routing_test.go` (beside `TestThermalEditsRouteToAggregate`)

**Interfaces:**
- Consumes: `femmodel.Analysis.EM()` (E1); the `ccx.EMDrive` display enum.
- Produces: `func overlayEM(a *femmodel.Analysis, s StudySettings) StudySettings`;
  `func (e *Engine) applyAggEMEdit(controlID, value string) bool`.

- [ ] **Step 1: Write the failing test** — add to `ccx/panel_routing_test.go` (uses `NewEngine(nil)` + `e.study()`):

```go
func TestEMEditsRouteToAggregate(t *testing.T) {
	e := NewEngine(nil)
	e.applyPanelEdit("voltage", "24")
	e.applyPanelEdit("em_drive", "current")
	e.applyPanelEdit("current_density", "3")
	em := e.analysis.EM()
	if em.EMDriveMode != "current" || em.VoltageV != 24 || em.CurrentDensity != 3 {
		t.Fatalf("EM edits did not land in the aggregate: %+v", em)
	}
	s, _ := e.study()
	if s.EMDriveMode != EMCurrent || s.VoltageV != 24 || s.CurrentDensity != 3 {
		t.Fatalf("study() did not reflect EM edits: %+v", s)
	}
}
```

> NOTE: `EMCurrent == "current"` (analysis.go). Confirm before using.

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./ccx/ -run TestEMEditsRouteToAggregate -v`
Expected: FAIL — controls still write `e.extras`, so `e.analysis.EM()` is unchanged.

- [ ] **Step 3: Add `overlayEM` to `ccx/project.go`** (beside `overlayThermal`):

```go
// overlayEM copies the 3 electromagnetic field-drive fields from the aggregate onto s.
func overlayEM(a *femmodel.Analysis, s StudySettings) StudySettings {
	em := a.EM()
	s.EMDriveMode = EMDrive(em.EMDriveMode)
	s.VoltageV, s.CurrentDensity = em.VoltageV, em.CurrentDensity
	return s
}
```

Wire into `projectAnalysis` right after `s = overlayThermal(a, s)`:

```go
	s = overlayEM(a, s)
```

- [ ] **Step 4: Add `applyAggEMEdit` to `ccx/panel.go`; delete `applyEMEdit`; reroute `applyLoadEdit`.**

New helper (beside `applyAggThermalEdit`):

```go
// applyAggEMEdit routes the 3 electromagnetic controls (applied voltage, drive mode, injected
// current density) to the Analysis EM template. Returns whether the control was recognised.
func (e *Engine) applyAggEMEdit(controlID, value string) bool {
	em := e.analysis.EM()
	switch controlID {
	case "voltage":
		em.VoltageV = panelNum(value, em.VoltageV)
	case "em_drive":
		em.EMDriveMode = strings.TrimSpace(value)
	case "current_density":
		em.CurrentDensity = panelNum(value, em.CurrentDensity)
	default:
		return false
	}
	e.analysis.SetEM(em)
	return true
}
```

Rewrite `applyLoadEdit`'s terminal branch to call the aggregate helper, and DELETE `applyEMEdit`:

```go
func (e *Engine) applyLoadEdit(controlID, value string) {
	if e.applyAggSupportEdit(controlID, value) {
		return
	}
	if e.applyAggThermalEdit(controlID, value) {
		return
	}
	if e.applyAggStudySwitchEdit(controlID, value) {
		return
	}
	e.applyAggEMEdit(controlID, value)
}
```

Delete the `applyEMEdit` function entirely. Confirm no dangling caller:
`grep -n 'applyEMEdit' ccx/`.

- [ ] **Step 5: Run to verify pass**

Run: `go test ./ccx/ -run TestEMEditsRouteToAggregate -v` → PASS.
Then `go test ./ccx/...` (equivalence guard included) → PASS.
Then `golangci-lint run ./ccx/...` → clean (unused/funlen).
Then `gofmt -l ccx/` → empty.

- [ ] **Step 6: Commit**

```bash
git add ccx/project.go ccx/panel.go ccx/*_test.go
git commit -m "feat(ccx): route EM field-drive params to the aggregate + overlay; retire applyEMEdit"
```

---

### Task E3: verification gate (no commit)

- [ ] **Step 1:** `go test ./...` + `go test -race ./ccx/...` — all green.
- [ ] **Step 2:** `golangci-lint run ./ccx/...` clean; `gofmt -l ccx/` empty.
- [ ] **Step 3:** Migration proof:
  - `grep -nE 'e\.extras\.(VoltageV|EMDriveMode|CurrentDensity)' ccx/panel.go` → **empty**.
  - `grep -n 'applyEMEdit' ccx/` → **empty** (deleted, no dangling caller).
  - **Strangler milestone:** `grep -nE 'e\.extras\.[A-Za-z]+ *=' ccx/panel.go` → **empty**
    (panel.go now performs ZERO extras writes — the precondition for 2.12).
  - `overlayEM` sets both magnitude fields + the mode in `project.go`; `projectAnalysis` calls it.
- [ ] **Step 4:** No commit (verification only).
