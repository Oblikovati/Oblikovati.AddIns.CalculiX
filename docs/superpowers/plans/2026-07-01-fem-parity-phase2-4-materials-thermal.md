<!-- SPDX-License-Identifier: GPL-2.0-only -->
# FEM Parity — Phase 2.4 (materials-thermal migration) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Migrate the **thermal material properties** — `ThermalAlpha` (expansion), `Conductivity`, `SpecificHeat` — from the flat `extras StudySettings` into the typed `femmodel.MaterialObject`, so the default material owns them and the panel controls edit the aggregate. First `2.4+` field-group migration; shrinks the aggregate-vs-extras boundary toward retiring `extras`.

**Architecture:** The incremental-migration pattern (Phase-2 ADR-1), extending exactly what slice 2.1 did for the 4 mechanical material fields: add the 3 thermal fields to `MaterialObject`; seed their defaults in `NewDefaultAnalysis`; `projectAnalysis` **overlays** them from the default material ("analysis wins"); re-route the `alpha`/`conductivity`/`specific_heat` panel controls from `e.extras` to the aggregate via the existing `applyAggCoreMatEdit` path. Editing stays in the panel (ADR-3 non-modal) — the *storage* moves, not the controls. Behavior-preserving; the `reflect.DeepEqual` equivalence test is the guard.

**Tech Stack:** Go; `oblikovati.org/calculix` add-in (pure `ccx/femmodel` + `ccx`); links only `oblikovati.org/api`.

## Global Constraints

- Every new `.go` file carries `// SPDX-License-Identifier: GPL-2.0-only`.
- `ccx/femmodel` stays PURE (imports nothing but stdlib). `ccx → femmodel` only.
- Style: functions 4–20 lines, files <500 lines, explicit types, early returns.
- **Behavior-preserving:** the equivalence test `TestProjectDefaultAnalysisEqualsDefaultSettings` and the full suite MUST stay green. The overlay rule is FIXED: analysis-owned fields (now incl. the 3 thermal) always win over `extras`.
- Do NOT remove `ThermalAlpha`/`Conductivity`/`SpecificHeat` from `StudySettings` — the deck/solve pipeline reads them; the migration moves their SOURCE OF TRUTH into the aggregate (overlay), leaving the `extras` copies dead (like the mechanical fields after 2.1).
- Run `go test ./...` + `golangci-lint run ./ccx/...` (watch `unused`) + `gofmt -l` before each commit.
- Branch: `feature/fem-parity-phase2-4-materials-thermal` (already off the merged `main`).

## Anchors (verified)

- `femmodel.MaterialObject{ id, name string; YoungGPa, Poisson, DensityGCm3, YieldMPa float64; ScopeAll bool }`; `newMaterialObject(id,name,young,poisson,density,yield,scopeAll)`; `AddMaterial(name,young,poisson,density,yield,scopeAll)`; `SetDefaultMaterial(m)` replaces the whole ScopeAll material (preserving id/name/scope).
- `NewDefaultAnalysis()` seeds `AddMaterial("Steel", 210, 0.3, 7.85, 0, true)`.
- `ccx/project.go` `projectAnalysis` material overlay: `if mat, ok := a.DefaultMaterial(); ok { s.YoungGPa=…; s.Poisson=…; s.DensityGCm3=…; s.YieldMPa=… }`.
- `ccx/panel.go`: display reads `s.ThermalAlpha`/`s.Conductivity`/`s.SpecificHeat` (via `s,_ := e.study()`); `applyMaterialEdit` → `applyAggCoreMatEdit` routes the 4 mechanical fields to the aggregate, and "hyperelastic and thermal/hot properties write to e.extras" — the `alpha`/`conductivity`/`specific_heat` cases currently write `e.extras`.
- Defaults (`ccx/analysis.go`): `ThermalAlpha = 1.2e-5`, `Conductivity = 50`, `SpecificHeat = 5e8`.

---

### Task M1: extend `MaterialObject` with thermal fields + seed defaults

**Files:**
- Modify: `ccx/femmodel/material_object.go` (add 3 exported fields + doc)
- Modify: `ccx/femmodel/analysis.go` (`NewDefaultAnalysis` seeds the thermal defaults on the default material)
- Test: `ccx/femmodel/material_thermal_test.go` (create)

**Interfaces:**
- Produces: `MaterialObject.ThermalAlpha`, `.Conductivity`, `.SpecificHeat` (exported `float64`); `NewDefaultAnalysis()`'s default material carries `ThermalAlpha=1.2e-5, Conductivity=50, SpecificHeat=5e8`; `SetDefaultMaterial` already preserves them (it replaces the whole object).

- [ ] **Step 1: Write the failing test**

Create `ccx/femmodel/material_thermal_test.go`:
```go
// SPDX-License-Identifier: GPL-2.0-only

package femmodel

import "testing"

func TestDefaultMaterialHasThermalDefaults(t *testing.T) {
	a := NewDefaultAnalysis()
	mat, ok := a.DefaultMaterial()
	if !ok {
		t.Fatal("no default material")
	}
	if mat.ThermalAlpha != 1.2e-5 || mat.Conductivity != 50 || mat.SpecificHeat != 5e8 {
		t.Fatalf("thermal defaults wrong: alpha=%g cond=%g cp=%g", mat.ThermalAlpha, mat.Conductivity, mat.SpecificHeat)
	}
}

func TestSetDefaultMaterialCarriesThermal(t *testing.T) {
	a := NewDefaultAnalysis()
	mat, _ := a.DefaultMaterial()
	mat.ThermalAlpha, mat.Conductivity, mat.SpecificHeat = 2e-5, 80, 4e8
	a.SetDefaultMaterial(mat)
	got, _ := a.DefaultMaterial()
	if got.ThermalAlpha != 2e-5 || got.Conductivity != 80 || got.SpecificHeat != 4e8 {
		t.Fatalf("thermal not persisted: %+v", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /home/vmiguel/git/oblikovati-workspace/Oblikovati.AddIns.CalculiX && go test ./ccx/femmodel/ -run 'TestDefaultMaterialHasThermal|TestSetDefaultMaterialCarriesThermal'`
Expected: FAIL — `mat.ThermalAlpha undefined`.

- [ ] **Step 3: Write minimal implementation**

In `ccx/femmodel/material_object.go`, add the 3 fields to the struct (after `YieldMPa`) and update the doc line:
```go
	YieldMPa    float64
	ThermalAlpha float64 // thermal expansion coefficient (1/K)
	Conductivity float64 // thermal conductivity (consistent units)
	SpecificHeat float64 // specific heat capacity (consistent units; transient)
	ScopeAll    bool
```
(Change the type doc "Phase 1 carries the core mechanical properties; thermal … migrate here in a later phase." → "carries mechanical + thermal properties; electromagnetic/hyperelastic migrate later." `newMaterialObject` keeps its mechanical-only signature — the thermal fields default to zero and are set explicitly where needed.)
In `ccx/femmodel/analysis.go` `NewDefaultAnalysis`, after `a.AddMaterial("Steel", 210, 0.3, 7.85, 0, true)`, seed the thermal defaults:
```go
	steel, _ := a.DefaultMaterial()
	steel.ThermalAlpha, steel.Conductivity, steel.SpecificHeat = 1.2e-5, 50, 5e8
	a.SetDefaultMaterial(steel)
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./ccx/femmodel/ -run 'TestDefaultMaterialHasThermal|TestSetDefaultMaterialCarriesThermal' -v` → PASS. Then `go test ./ccx/femmodel/` → PASS.

- [ ] **Step 5: Commit**
```bash
git add ccx/femmodel/material_object.go ccx/femmodel/analysis.go ccx/femmodel/material_thermal_test.go
git commit -m "feat(femmodel): MaterialObject carries thermal props (alpha/conductivity/specific heat)"
```

---

### Task M2: overlay + re-route the thermal panel controls

**Files:**
- Modify: `ccx/project.go` (`projectAnalysis` overlays the 3 thermal fields from the default material)
- Modify: `ccx/panel.go` (`applyAggCoreMatEdit` also routes `alpha`/`conductivity`/`specific_heat` to the aggregate; remove them from the `e.extras` path in `applyMaterialEdit`)
- Test: `ccx/project_test.go` (equivalence still green — no change needed unless it drifts) + `ccx/panel_routing_test.go` (add thermal-routing assertions)

**Interfaces:**
- Consumes: `MaterialObject` thermal fields (M1); `SetDefaultMaterial`/`DefaultMaterial`.
- Produces: `projectAnalysis` sets `s.ThermalAlpha`/`s.Conductivity`/`s.SpecificHeat` from the default material; `applyPanelEdit("alpha"/"conductivity"/"specific_heat", …)` mutates `e.analysis`'s default material (not `e.extras`).

- [ ] **Step 1: Write the failing test**

In `ccx/panel_routing_test.go`, add:
```go
func TestThermalMaterialEditRoutesToAggregate(t *testing.T) {
	e := NewEngine(nil)
	e.applyPanelEdit("alpha", "2.5e-5")
	e.applyPanelEdit("conductivity", "77")
	e.applyPanelEdit("specific_heat", "4.2e8")
	mat, _ := e.analysis.DefaultMaterial()
	if mat.ThermalAlpha != 2.5e-5 || mat.Conductivity != 77 || mat.SpecificHeat != 4.2e8 {
		t.Fatalf("thermal edits did not land in the aggregate material: %+v", mat)
	}
	// And the projection reflects them.
	s, _ := e.study()
	if s.ThermalAlpha != 2.5e-5 || s.Conductivity != 77 || s.SpecificHeat != 4.2e8 {
		t.Fatalf("study() did not reflect thermal edits: %+v", s)
	}
}
```
The existing `TestProjectDefaultAnalysisEqualsDefaultSettings` must STILL pass (the default material now carries the same thermal defaults defaultSettings() has) — do not change it; if it drifts red, the M1 seeding is wrong.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./ccx/ -run 'TestThermalMaterialEditRoutesToAggregate|TestProjectDefaultAnalysisEqualsDefaultSettings'`
Expected: the new test FAILS (edits still land in `e.extras`, not the aggregate); the equivalence test PASSES (M1 kept defaults aligned).

- [ ] **Step 3: Write minimal implementation**

In `ccx/project.go`, extend the material overlay block:
```go
	if mat, ok := a.DefaultMaterial(); ok {
		s.YoungGPa = mat.YoungGPa
		s.Poisson = mat.Poisson
		s.DensityGCm3 = mat.DensityGCm3
		s.YieldMPa = mat.YieldMPa
		s.ThermalAlpha = mat.ThermalAlpha
		s.Conductivity = mat.Conductivity
		s.SpecificHeat = mat.SpecificHeat
	}
```
In `ccx/panel.go` `applyAggCoreMatEdit`, add the 3 thermal cases (read-modify-`SetDefaultMaterial`, mirroring the mechanical cases):
```go
	case "alpha":
		mat, _ := e.analysis.DefaultMaterial()
		mat.ThermalAlpha = panelNum(value, mat.ThermalAlpha)
		e.analysis.SetDefaultMaterial(mat)
	case "conductivity":
		mat, _ := e.analysis.DefaultMaterial()
		mat.Conductivity = panelNum(value, mat.Conductivity)
		e.analysis.SetDefaultMaterial(mat)
	case "specific_heat":
		mat, _ := e.analysis.DefaultMaterial()
		mat.SpecificHeat = panelNum(value, mat.SpecificHeat)
		e.analysis.SetDefaultMaterial(mat)
```
Then REMOVE the now-dead `alpha`/`conductivity`/`specific_heat` cases from the `e.extras`-writing path in `applyMaterialEdit` (grep the file — they currently do `e.extras.ThermalAlpha = …` etc.). If `applyAggCoreMatEdit` now covers 7 fields (4 mechanical + 3 thermal) and exceeds the 20-line/statement budget, split it into `applyAggMechanicalMatEdit` + `applyAggThermalMatEdit`, both dispatched from `applyMaterialEdit`. Rename `applyAggCoreMatEdit`→`applyAggMaterialEdit` if that reads better; keep it ≤20 lines/helper.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./ccx/ -run 'TestThermalMaterialEditRoutesToAggregate|TestProjectDefaultAnalysisEqualsDefaultSettings' -v` → PASS. Then `go test ./...` → all green (behavior preserved). `grep -n 'e\.extras\.\(ThermalAlpha\|Conductivity\|SpecificHeat\)' ccx/panel.go` → NO writes (the thermal fields are aggregate-owned now; reads via `study()` are fine).

- [ ] **Step 5: Commit**
```bash
git add ccx/project.go ccx/panel.go ccx/panel_routing_test.go
git commit -m "feat(ccx): route thermal material edits to the aggregate + overlay in projection"
```

---

### Task M3: verification gate

- [ ] **Step 1:** `go test ./...` — all green (femmodel + ccx incl. the equivalence guard).
- [ ] **Step 2:** `golangci-lint run ./ccx/...` clean (watch `unused` + funlen on the split material helpers); `gofmt -l ccx/` empty.
- [ ] **Step 3:** Confirm the migration: `grep -n 'e\.extras\.\(ThermalAlpha\|Conductivity\|SpecificHeat\)' ccx/*.go` shows NO write sites (reads are gone too — the panel displays via `study()`); `grep -n 'ThermalAlpha\|Conductivity\|SpecificHeat' ccx/project.go` shows the overlay.
- [ ] **Step 4:** No commit (verification). Fix gaps via a focused TDD cycle.

---

## Self-Review (completed by plan author)

- **Spec coverage:** implements the materials-thermal migration per Phase-2 ADR-1: the 3 thermal fields move into `MaterialObject` (M1), the projection overlays them and the panel controls re-route to the aggregate (M2). Behavior-preserving (equivalence guard); panel stays editable (ADR-3). The `extras` copies of the 3 fields become dead (overlaid), consistent with the 2.1 mechanical fields.
- **Placeholder scan:** none — the fields, defaults, overlay, and panel cases are fully coded; the only judgment call ("split `applyAggCoreMatEdit` if over budget / rename") is an explicit funlen-driven instruction, not a TODO.
- **Type consistency:** `MaterialObject.ThermalAlpha/Conductivity/SpecificHeat` used identically in M1/M2; `SetDefaultMaterial`/`DefaultMaterial` reused; overlay mirrors the mechanical block exactly.
- **Equivalence guarded:** M1 seeds the same defaults `defaultSettings()` holds, so `projectAnalysis(NewDefaultAnalysis(), defaultSettings()) == defaultSettings()` stays true.

## Next slices
- **2.5** — materials EM / hyperelastic / temperature-dependent-E (`YoungHotGPa`/`HotTempK`, `MaterialModel`/`NeoHookeC10`/`NeoHookeD1`, `ElectricalSigma`) into `MaterialObject`/a material-model field.
- **2.6+** — contact params → constraints-into-`femmodel` → results, each shrinking `extras`, until it is empty and `panel.go` retires. Plus the icon pass.
