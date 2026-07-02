<!-- SPDX-License-Identifier: GPL-2.0-only -->
# FEM Parity — Phase 2.8 (default-load params → aggregate) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans. Steps use checkbox (`- [ ]`) syntax.

**Goal:** Migrate the load parameters — `LoadType`, `LoadN`, `PressureMPa`, `GravityG`, `RotationRadS`, `DisplacementMM`, `HydroGradientMPaMM`, `HydroSurfaceZ` — from `extras` into a `femmodel.LoadDefaults` aggregate template; re-route their 8 panel controls to the aggregate; and **close the TOCTOU** the 2.7 review flagged in `addConstraintFromSelection`.

**Architecture:** ADR-3: the load magnitudes parameterize the implicit convention (`loadSpec`) and the explicit builder (`objectForKind`), both of which read PROJECTED `StudySettings` — so moving the source of truth into the aggregate + overlaying it leaves them unchanged. `LoadDefaults` is a plain pure value struct on `Analysis` (not a tree node — ADR-3). Behavior-preserving; equivalence-guarded.

## Global Constraints

- New `.go` files carry `// SPDX-License-Identifier: GPL-2.0-only`. `ccx/femmodel` PURE (stdlib only; `LoadType` held as `string`).
- Style: functions 4–20 lines, explicit types, early returns. `ccx → femmodel` only.
- **Behavior-preserving:** `TestProjectDefaultAnalysisEqualsDefaultSettings` + full suite stay green; overlay total (all 8 fields).
- Do NOT remove the 8 fields from `StudySettings` (pipeline reads them); move their SOURCE OF TRUTH only.
- Run `go test ./...` + `go test -race ./ccx/...` + `golangci-lint run ./ccx/...` + `gofmt -l` before each commit.
- Branch: `feature/fem-parity-phase2-8-default-load` (off merged `main`).

## Anchors (verified)

- `ccx.LoadType string`: `LoadForce="force"`, `LoadPressure="pressure"`, `LoadGravity="gravity"`, `LoadCentrifugal="centrifugal"`, `LoadDisplacement="displacement"`, `LoadHydrostatic="hydrostatic"`.
- Defaults (`ccx/analysis.go`): `LoadType=LoadForce, LoadN=100, PressureMPa=1, GravityG=1, RotationRadS=100, DisplacementMM=0.1, HydroGradientMPaMM=1e-5, HydroSurfaceZ=0`.
- Panel controls (panel.go): `load_type, load, pressure, gravity, rotation, displacement, hydro_gradient, hydro_surface` — handled in a load-edit helper (grep `case "load_type"` ~panel.go:391; the cases write `e.extras.X`).
- `femmodel.Analysis` mutator pattern (`SetSolver`/`SetDefaultMaterial`). `projectAnalysis` overlay blocks (solver/mesh/material/switches).
- `addConstraintFromSelection` (post-2.7): `settings, _ := e.study(); e.mu.Lock(); … objectForKind(e.builderKind, faces, settings) …; e.mu.Unlock()` — the two-lock TOCTOU to close. `e.study()` = `e.mu.Lock(); defer Unlock(); return projectAnalysis(e.analysis, e.extras)`.

---

### Task L1: `femmodel.LoadDefaults` + `Analysis.Load`/`SetLoad` + seed

**Files:**
- Create: `ccx/femmodel/load_defaults.go`
- Modify: `ccx/femmodel/analysis.go` (add `load LoadDefaults` + `Load()`/`SetLoad()`; seed in `NewDefaultAnalysis`)
- Test: `ccx/femmodel/load_defaults_test.go`

**Interfaces:**
- Produces: `femmodel.LoadDefaults{ LoadType string; LoadN, PressureMPa, GravityG, RotationRadS, DisplacementMM, HydroGradientMPaMM, HydroSurfaceZ float64 }`; `(*Analysis).Load() LoadDefaults`; `(*Analysis).SetLoad(LoadDefaults)`; default `Load()` carries `LoadType="force", LoadN=100, PressureMPa=1, GravityG=1, RotationRadS=100, DisplacementMM=0.1, HydroGradientMPaMM=1e-5, HydroSurfaceZ=0`.

- [ ] **Step 1: Write the failing test**

Create `ccx/femmodel/load_defaults_test.go`:
```go
// SPDX-License-Identifier: GPL-2.0-only

package femmodel

import "testing"

func TestDefaultLoad(t *testing.T) {
	ld := NewDefaultAnalysis().Load()
	if ld.LoadType != "force" || ld.LoadN != 100 || ld.PressureMPa != 1 || ld.GravityG != 1 {
		t.Fatalf("load defaults wrong (1): %+v", ld)
	}
	if ld.RotationRadS != 100 || ld.DisplacementMM != 0.1 || ld.HydroGradientMPaMM != 1e-5 || ld.HydroSurfaceZ != 0 {
		t.Fatalf("load defaults wrong (2): %+v", ld)
	}
}

func TestSetLoad(t *testing.T) {
	a := NewDefaultAnalysis()
	a.SetLoad(LoadDefaults{LoadType: "pressure", PressureMPa: 5, LoadN: 7, GravityG: 2,
		RotationRadS: 50, DisplacementMM: 0.3, HydroGradientMPaMM: 2e-5, HydroSurfaceZ: 10})
	got := a.Load()
	if got.LoadType != "pressure" || got.PressureMPa != 5 || got.HydroSurfaceZ != 10 {
		t.Fatalf("SetLoad not persisted: %+v", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /home/vmiguel/git/oblikovati-workspace/Oblikovati.AddIns.CalculiX && go test ./ccx/femmodel/ -run 'TestDefaultLoad|TestSetLoad'`
Expected: FAIL — `undefined: (*Analysis).Load`.

- [ ] **Step 3: Write minimal implementation**

Create `ccx/femmodel/load_defaults.go`:
```go
// SPDX-License-Identifier: GPL-2.0-only

package femmodel

// LoadDefaults holds the parameters of the study's default load — the numbers the implicit
// convention (and the explicit builder) apply to the loaded faces. Not a tree node: the load is
// synthesized at solve time from the selection, so this carries only its params (LoadType as a
// string; ccx casts it). One per Analysis.
type LoadDefaults struct {
	LoadType           string
	LoadN              float64
	PressureMPa        float64
	GravityG           float64
	RotationRadS       float64
	DisplacementMM     float64
	HydroGradientMPaMM float64
	HydroSurfaceZ      float64
}
```
In `ccx/femmodel/analysis.go`, add `load LoadDefaults` to the `Analysis` struct; add:
```go
// Load returns the default-load parameters.
func (a *Analysis) Load() LoadDefaults { return a.load }

// SetLoad replaces the default-load parameters.
func (a *Analysis) SetLoad(l LoadDefaults) { a.load = l }
```
In `NewDefaultAnalysis`, seed (near the solver-switch seed block):
```go
	a.SetLoad(LoadDefaults{LoadType: "force", LoadN: 100, PressureMPa: 1, GravityG: 1,
		RotationRadS: 100, DisplacementMM: 0.1, HydroGradientMPaMM: 1e-5, HydroSurfaceZ: 0})
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./ccx/femmodel/ -run 'TestDefaultLoad|TestSetLoad' -v` → PASS. Then `go test ./ccx/femmodel/` → PASS.

- [ ] **Step 5: Commit**
```bash
git add ccx/femmodel/load_defaults.go ccx/femmodel/analysis.go ccx/femmodel/load_defaults_test.go
git commit -m "feat(femmodel): LoadDefaults template on Analysis (Load/SetLoad + seed)"
```

---

### Task L2: overlay + re-route the 8 load controls + close the TOCTOU

**Files:**
- Modify: `ccx/project.go` (overlay the 8 load fields from `a.Load()`)
- Modify: `ccx/panel.go` (route the 8 load controls to a new `applyAggLoadEdit`; remove them from the `e.extras` path)
- Modify: `ccx/constraintbuilder.go` (`addConstraintFromSelection`: single lock, `projectAnalysis(e.analysis, e.extras)` under it — closes the TOCTOU)
- Test: `ccx/panel_routing_test.go` (add routing assertions) + equivalence stays green

**Interfaces:**
- Consumes: `a.Load()`/`SetLoad` (L1); `LoadType`.
- Produces: `projectAnalysis` sets `s.LoadType`/`s.LoadN`/`s.PressureMPa`/`s.GravityG`/`s.RotationRadS`/`s.DisplacementMM`/`s.HydroGradientMPaMM`/`s.HydroSurfaceZ` from `a.Load()`; the 8 controls mutate `e.analysis`'s load; `addConstraintFromSelection` captures params+count+insert under one lock.

- [ ] **Step 1: Write the failing test**

In `ccx/panel_routing_test.go`, add:
```go
func TestLoadEditsRouteToAggregate(t *testing.T) {
	e := NewEngine(nil)
	e.applyPanelEdit("load_type", "pressure")
	e.applyPanelEdit("load", "250")
	e.applyPanelEdit("pressure", "3")
	e.applyPanelEdit("gravity", "2")
	e.applyPanelEdit("rotation", "60")
	e.applyPanelEdit("displacement", "0.5")
	e.applyPanelEdit("hydro_gradient", "2e-5")
	e.applyPanelEdit("hydro_surface", "8")
	ld := e.analysis.Load()
	if ld.LoadType != "pressure" || ld.LoadN != 250 || ld.PressureMPa != 3 || ld.GravityG != 2 ||
		ld.RotationRadS != 60 || ld.DisplacementMM != 0.5 || ld.HydroGradientMPaMM != 2e-5 || ld.HydroSurfaceZ != 8 {
		t.Fatalf("load edits did not land in the aggregate: %+v", ld)
	}
	s, _ := e.study()
	if string(s.LoadType) != "pressure" || s.LoadN != 250 || s.HydroSurfaceZ != 8 {
		t.Fatalf("study() did not reflect load edits: %+v", s)
	}
}
```
Equivalence test must stay green unchanged. If red, STOP (L1 seeding mismatch).

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./ccx/ -run 'TestLoadEditsRouteToAggregate|TestProjectDefaultAnalysisEqualsDefaultSettings'`
Expected: new test FAILS (edits still in extras); equivalence PASSES.

- [ ] **Step 3: Write minimal implementation**

In `ccx/project.go`, add to the overlay (a good place: after the switch/EM overlays; or its own block) — set the 8 load fields from `a.Load()`:
```go
	ld := a.Load()
	s.LoadType = LoadType(ld.LoadType)
	s.LoadN, s.PressureMPa, s.GravityG = ld.LoadN, ld.PressureMPa, ld.GravityG
	s.RotationRadS, s.DisplacementMM = ld.RotationRadS, ld.DisplacementMM
	s.HydroGradientMPaMM, s.HydroSurfaceZ = ld.HydroGradientMPaMM, ld.HydroSurfaceZ
```
(If this pushes `projectAnalysis` over funlen, extract an `overlayLoad(s, a)` helper mirroring `overlayMaterial`.)
In `ccx/panel.go`, add the aggregate helper (read-once `Load` / switch / `SetLoad`-once):
```go
// applyAggLoadEdit routes the load controls (type + magnitudes) to the Analysis load template.
func (e *Engine) applyAggLoadEdit(controlID, value string) bool {
	ld := e.analysis.Load()
	switch controlID {
	case "load_type":
		ld.LoadType = strings.TrimSpace(value)
	case "load":
		ld.LoadN = panelNum(value, ld.LoadN)
	case "pressure":
		ld.PressureMPa = panelNum(value, ld.PressureMPa)
	case "gravity":
		ld.GravityG = panelNum(value, ld.GravityG)
	case "rotation":
		ld.RotationRadS = panelNum(value, ld.RotationRadS)
	case "displacement":
		ld.DisplacementMM = panelNum(value, ld.DisplacementMM)
	case "hydro_gradient":
		ld.HydroGradientMPaMM = panelNum(value, ld.HydroGradientMPaMM)
	case "hydro_surface":
		ld.HydroSurfaceZ = panelNum(value, ld.HydroSurfaceZ)
	default:
		return false
	}
	e.analysis.SetLoad(ld)
	return true
}
```
Wire it into the panel-edit dispatch BEFORE the load controls' current `e.extras` handler (the load-edit helper), and REMOVE the 8 load cases from that extras helper. READ the current dispatch (the load-edit helper handling `load_type`/`load`/etc.) and mirror how the material aggregate helpers are dispatched — try `applyAggLoadEdit` first, `return` if it matched. If the extras helper becomes empty after removing the 8 cases, delete it (watch `unused`).
In `ccx/constraintbuilder.go` `addConstraintFromSelection`, close the TOCTOU — replace `settings, _ := e.study()` + `e.mu.Lock()` with a single critical section:
```go
	e.mu.Lock()
	settings, _ := projectAnalysis(e.analysis, e.extras)
	name := fmt.Sprintf("C%d", len(e.analysis.Constraints()))
	obj := e.analysis.AddConstraint(name, objectForKind(e.builderKind, faces, settings))
	count := len(e.analysis.Constraints())
	kind := ConstraintKind(obj.Kind)
	e.mu.Unlock()
```
(params + count + insert now atomic under one lock; `projectAnalysis` is called directly so it does NOT re-lock — confirm `projectAnalysis` takes no lock itself.)

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./ccx/ -run 'TestLoadEditsRouteToAggregate|TestProjectDefaultAnalysisEqualsDefaultSettings' -v` → PASS. Then `go test ./...` + `go test -race ./ccx/...` → all green. Verify `grep -nE 'e\.extras\.(LoadType|LoadN|PressureMPa|GravityG|RotationRadS|DisplacementMM|HydroGradientMPaMM|HydroSurfaceZ)' ccx/panel.go` empty; `golangci-lint run ./ccx/...` clean.

- [ ] **Step 5: Commit**
```bash
git add ccx/project.go ccx/panel.go ccx/constraintbuilder.go ccx/panel_routing_test.go
git commit -m "feat(ccx): route load params to the aggregate + overlay; close the addConstraint TOCTOU"
```

---

### Task L3: verification gate

- [ ] **Step 1:** `go test ./...` + `go test -race ./ccx/...` — all green.
- [ ] **Step 2:** `golangci-lint run ./ccx/...` clean; `gofmt -l ccx/` empty.
- [ ] **Step 3:** Migration proof: `grep -nE 'e\.extras\.(LoadType|LoadN|PressureMPa|GravityG|RotationRadS|DisplacementMM|HydroGradientMPaMM|HydroSurfaceZ)' ccx/panel.go` → **empty**; the overlay sets all 8 in `project.go`; `addConstraintFromSelection` has ONE `e.mu.Lock()` and calls `projectAnalysis` (not `e.study()`).
- [ ] **Step 4:** No commit (verification).

---

## Self-Review (plan author)

- **Spec coverage:** default-load params → `LoadDefaults` (L1); overlaid + 8 controls re-routed + TOCTOU closed (L2). `loadSpec`/`objectForKind` read projected settings unchanged (ADR-3). Behavior-preserving; equivalence-guarded.
- **Placeholder scan:** the only "locate" directive is "find the current load-edit extras helper + its dispatch" — an explicit locate-and-mirror against the panel controls; the fields/overlay/helper/TOCTOU-fix are fully coded.
- **Type consistency:** `LoadDefaults.LoadType` is a `string`, `LoadType(ld.LoadType)` at the seam; `Load()`/`SetLoad` reused; the aggregate helper mirrors `applyAggThermalMatEdit`.
- **TOCTOU:** closed by projecting under the single insert lock (`projectAnalysis` takes no lock, so no deadlock).

## Next slices
- **2.9** default-support (`SupportType`, `SpringStiffMM`) → `Analysis.SupportDefaults`. **2.10** thermal-BC group → `femmodel.ThermalBCObject`. **2.11** field-drive (`VoltageV`/`EMDriveMode`/`CurrentDensity`). **2.12** drop the `extras` arg from `projectAnalysis`, delete dead `StudySettings` fields, retire `panel.go`.
