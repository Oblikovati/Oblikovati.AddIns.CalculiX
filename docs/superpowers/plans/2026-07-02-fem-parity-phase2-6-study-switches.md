<!-- SPDX-License-Identifier: GPL-2.0-only -->
# FEM Parity — Phase 2.6 (study-wide switches → SolverObject) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Migrate the three study-wide switches — `BodyScope`, `ContactMode`, `FrictionMu` — from the flat `extras StudySettings` onto `femmodel.SolverObject`, re-routing their panel controls to the aggregate. The smallest safe first slice of the non-material remainder (no builder coupling — a pure overlay).

## Design context (from the architect's remainder-decomposition brief)

The remaining `extras` migration is a strangler: field groups move into the pure `femmodel.Analysis` one green slice at a time; `projectAnalysis` keeps re-flattening to `StudySettings` so the deck/solve pipeline never changes. Load-bearing ADRs:

- **ADR-1 — `ConstraintObject.Kind` is a neutral `string`; ccx owns `ConstraintKind` + a `constraintSpecFor` mapper.** Exactly the `MaterialModel`-as-string precedent. `femmodel` never learns the CalculiX taxonomy; dependency stays `ccx → femmodel`.
- **ADR-2 — study-wide switches (`BodyScope`, `ContactMode`, `FrictionMu`) live on `SolverObject`, NOT as constraints.** They are global policy (scope, interface treatment), peers of analysis-type/eigenmodes. **This slice (2.6).**
- **ADR-3 — the implicit convention stays a solve-time synthesis**, later parameterized by aggregate `defaultLoad`/`defaultSupport` templates; editing stays non-modal.
- **ADR-4 — `StudySettings` survives as the pipeline read-model DTO; `projectAnalysis` is the sole ACL ("analysis wins"); `panel.go` retires last.**

**Ordered sub-slices:** **2.6 study switches → SolverObject (THIS)** · 2.7 `ConstraintObject` spine + explicit-list migration (the crux; `femmodel/constraint_object.go` + `ccx/constraintmap.go`) · 2.8 default-load params → `Analysis.defaultLoad` · 2.9 default-support params → `Analysis.defaultSupport` · 2.10 thermal-BC params → `femmodel.ThermalBCObject` · 2.11 field-drive params → `FieldDriveObject` · 2.12 retire `extras` (drop the `extras` arg from `projectAnalysis`) + retire `panel.go`.

## Architecture (this slice)

The proven material pattern, on `SolverObject`: add the 3 fields (`BodyScope` as a `string` — `femmodel` stays pure, ccx casts `BodyScope(sv.BodyScope)` at the seam); seed defaults in `NewDefaultAnalysis`; `projectAnalysis` overlays them after the existing solver block; re-route the `body_scope`/`contact_mode`/`friction` panel controls from `e.extras` (currently in `applyEMEdit`) to a new aggregate helper. Behavior-preserving; the `reflect.DeepEqual` equivalence test is the guard. These 3 fields have **no builder coupling** (they feed only `scopeBodies`/`applyInterfaces`, never `newConstraintSpec`), so it is a pure, zero-hazard overlay.

## Global Constraints

- Every new `.go` file carries `// SPDX-License-Identifier: GPL-2.0-only`.
- `ccx/femmodel` stays PURE (stdlib only) — `SolverObject.BodyScope` is a plain `string`, NOT the ccx `BodyScope` enum.
- Style: functions 4–20 lines, files <500 lines, explicit types, early returns.
- **Behavior-preserving:** `TestProjectDefaultAnalysisEqualsDefaultSettings` and the full suite MUST stay green; the overlay is total (all 3 fields overlaid).
- Do NOT remove the 3 fields from `StudySettings` (the deck/solve pipeline reads them via `s.material()`/`scopeBodies`/`applyInterfaces`); the migration moves their SOURCE OF TRUTH.
- Run `go test ./...` + `golangci-lint run ./ccx/...` (watch `unused`) + `gofmt -l` before each commit.
- Branch: `feature/fem-parity-phase2-6-study-switches` (already off the merged `main`).

## Anchors (verified)

- `femmodel.SolverObject{ id, AnalysisType string; Eigenmodes int; TransientTimeS float64 }`; `newSolverObject(id, analysisType string, eigenmodes int, transientS float64)`; `SetSolver(s SolverObject)` replaces the whole solver preserving id.
- `NewDefaultAnalysis()` builds `solver: newSolverObject("solver", "static", 6, 0)` in a struct literal, then seeds materials/result.
- `ccx.BodyScope string` enum: `BodyScopeAll = "all solid bodies"`, `BodyScopeSelected = "bodies with a selected face"`. Defaults (`ccx/analysis.go`): `ContactMode = false`, `FrictionMu = 0.3`, `BodyScope = BodyScopeAll`.
- `ccx/panel.go` `applyEMEdit` (line ~459) currently handles `voltage`/`em_drive`/`current_density` (EM, → extras, stay for 2.11) AND `contact_mode`/`friction`/`body_scope` (→ extras): `e.extras.ContactMode = strings.TrimSpace(value) == "contact"`; `e.extras.FrictionMu = panelNum(value, e.extras.FrictionMu)`; `e.extras.BodyScope = BodyScope(strings.TrimSpace(value))`.
- `ccx/project.go` solver block: `sv := a.Solver(); s.Analysis = AnalysisType(sv.AnalysisType); s.Eigenmodes = …; s.TransientTimeS = …`.

---

### Task S1: `SolverObject` gains the 3 switch fields + seed

**Files:**
- Modify: `ccx/femmodel/solver_object.go` (add 3 fields)
- Modify: `ccx/femmodel/analysis.go` (`NewDefaultAnalysis` seeds them on the solver)
- Test: `ccx/femmodel/solver_switches_test.go` (create)

**Interfaces:**
- Produces: `SolverObject.BodyScope string`, `.ContactMode bool`, `.FrictionMu float64`; the default analysis's solver carries `BodyScope="all solid bodies"`, `ContactMode=false`, `FrictionMu=0.3`.

- [ ] **Step 1: Write the failing test**

Create `ccx/femmodel/solver_switches_test.go`:
```go
// SPDX-License-Identifier: GPL-2.0-only

package femmodel

import "testing"

func TestDefaultSolverHasStudySwitches(t *testing.T) {
	sv := NewDefaultAnalysis().Solver()
	if sv.BodyScope != "all solid bodies" || sv.ContactMode || sv.FrictionMu != 0.3 {
		t.Fatalf("study-switch defaults wrong: scope=%q contact=%v mu=%g", sv.BodyScope, sv.ContactMode, sv.FrictionMu)
	}
}

func TestSetSolverCarriesStudySwitches(t *testing.T) {
	a := NewDefaultAnalysis()
	sv := a.Solver()
	sv.BodyScope, sv.ContactMode, sv.FrictionMu = "bodies with a selected face", true, 0.15
	a.SetSolver(sv)
	got := a.Solver()
	if got.BodyScope != "bodies with a selected face" || !got.ContactMode || got.FrictionMu != 0.15 {
		t.Fatalf("switches not persisted: %+v", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /home/vmiguel/git/oblikovati-workspace/Oblikovati.AddIns.CalculiX && go test ./ccx/femmodel/ -run 'TestDefaultSolverHasStudySwitches|TestSetSolverCarriesStudySwitches'`
Expected: FAIL — `sv.BodyScope undefined`.

- [ ] **Step 3: Write minimal implementation**

In `ccx/femmodel/solver_object.go`, add to the struct (after `TransientTimeS`):
```go
	BodyScope    string  // which solid bodies to analyse ("all solid bodies" | "bodies with a selected face")
	ContactMode  bool    // treat detected body interfaces as unilateral contact (vs bonded *TIE)
	FrictionMu   float64 // Coulomb friction for contact interfaces; 0 = frictionless
```
(`newSolverObject` keeps its 4-arg signature — the switch fields zero-default and are seeded explicitly.)
In `ccx/femmodel/analysis.go` `NewDefaultAnalysis`, after the `&Analysis{...}` literal (before or after the existing material/result seeding, but on the solver), seed via read-modify-`SetSolver`:
```go
	sv := a.Solver()
	sv.BodyScope, sv.ContactMode, sv.FrictionMu = "all solid bodies", false, 0.3
	a.SetSolver(sv)
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./ccx/femmodel/ -run 'TestDefaultSolverHasStudySwitches|TestSetSolverCarriesStudySwitches' -v` → PASS. Then `go test ./ccx/femmodel/` → PASS.

- [ ] **Step 5: Commit**
```bash
git add ccx/femmodel/solver_object.go ccx/femmodel/analysis.go ccx/femmodel/solver_switches_test.go
git commit -m "feat(femmodel): SolverObject carries the study-wide switches (scope/contact/friction)"
```

---

### Task S2: overlay + re-route the 3 switch controls to the aggregate

**Files:**
- Modify: `ccx/project.go` (`projectAnalysis` overlays the 3 from the solver)
- Modify: `ccx/panel.go` (route `body_scope`/`contact_mode`/`friction` to a new `applyAggStudySwitchEdit`; remove them from `applyEMEdit`)
- Test: `ccx/panel_routing_test.go` (add the routing assertions) + equivalence test stays green untouched

**Interfaces:**
- Consumes: the S1 fields; `Solver()`/`SetSolver`; `ccx.BodyScope`.
- Produces: `projectAnalysis` sets `s.BodyScope`/`s.ContactMode`/`s.FrictionMu` from the solver; `applyPanelEdit("body_scope"/"contact_mode"/"friction", …)` mutates `e.analysis`'s solver (not `e.extras`).

- [ ] **Step 1: Write the failing test**

In `ccx/panel_routing_test.go`, add:
```go
func TestStudySwitchEditsRouteToAggregate(t *testing.T) {
	e := NewEngine(nil)
	e.applyPanelEdit("contact_mode", "contact")
	e.applyPanelEdit("friction", "0.2")
	e.applyPanelEdit("body_scope", "bodies with a selected face")
	sv := e.analysis.Solver()
	if !sv.ContactMode || sv.FrictionMu != 0.2 || sv.BodyScope != "bodies with a selected face" {
		t.Fatalf("switch edits did not land in the solver: %+v", sv)
	}
	s, _ := e.study()
	if !s.ContactMode || s.FrictionMu != 0.2 || string(s.BodyScope) != "bodies with a selected face" {
		t.Fatalf("study() did not reflect switch edits: %+v", s)
	}
}
```
The existing `TestProjectDefaultAnalysisEqualsDefaultSettings` must STILL pass unchanged. If it drifts red, STOP — S1 seeding mismatch.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./ccx/ -run 'TestStudySwitchEditsRouteToAggregate|TestProjectDefaultAnalysisEqualsDefaultSettings'`
Expected: the new test FAILS (edits still land in `e.extras`); equivalence PASSES.

- [ ] **Step 3: Write minimal implementation**

In `ccx/project.go`, after `s.TransientTimeS = sv.TransientTimeS`:
```go
	s.BodyScope = BodyScope(sv.BodyScope)
	s.ContactMode = sv.ContactMode
	s.FrictionMu = sv.FrictionMu
```
In `ccx/panel.go`, add an aggregate helper (read-once `Solver` / switch / `SetSolver`-once, mirroring `applyAggThermalMatEdit`):
```go
// applyAggStudySwitchEdit routes the study-wide switches (body scope, contact mode, friction) to the
// Analysis aggregate's SolverObject. Returns whether it matched.
func (e *Engine) applyAggStudySwitchEdit(controlID, value string) bool {
	sv := e.analysis.Solver()
	switch controlID {
	case "body_scope":
		sv.BodyScope = strings.TrimSpace(value)
	case "contact_mode":
		sv.ContactMode = strings.TrimSpace(value) == "contact"
	case "friction":
		sv.FrictionMu = panelNum(value, sv.FrictionMu)
	default:
		return false
	}
	e.analysis.SetSolver(sv)
	return true
}
```
Then wire it into the dispatch BEFORE `applyEMEdit` runs (find where `applyEMEdit` is called — currently `e.applyEMEdit(controlID, value)` at panel.go ~454): change that site to
```go
	if e.applyAggStudySwitchEdit(controlID, value) {
		return
	}
	e.applyEMEdit(controlID, value)
```
(adjust to the exact surrounding control flow — `applyEMEdit` is a `void` catch-all; the study-switch helper must be tried first). Then **REMOVE** the `contact_mode`/`friction`/`body_scope` cases from `applyEMEdit` (leaving only `voltage`/`em_drive`/`current_density`, which stay in `extras` for slice 2.11). Keep each helper ≤20 lines.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./ccx/ -run 'TestStudySwitchEditsRouteToAggregate|TestProjectDefaultAnalysisEqualsDefaultSettings' -v` → PASS. Then `go test ./...` → all green. Verify: `grep -nE 'e\.extras\.(BodyScope|ContactMode|FrictionMu)' ccx/panel.go` → empty. `golangci-lint run ./ccx/...` → clean.

- [ ] **Step 5: Commit**
```bash
git add ccx/project.go ccx/panel.go ccx/panel_routing_test.go
git commit -m "feat(ccx): route the study-wide switches to the SolverObject aggregate"
```

---

### Task S3: verification gate

- [ ] **Step 1:** `go test ./...` — all green (incl. the equivalence guard).
- [ ] **Step 2:** `golangci-lint run ./ccx/...` clean (watch `unused` + funlen); `gofmt -l ccx/` empty.
- [ ] **Step 3:** Migration proof: `grep -nE 'e\.extras\.(BodyScope|ContactMode|FrictionMu)' ccx/panel.go` → **empty**; `grep -c 'sv\.\(BodyScope\|ContactMode\|FrictionMu\)' ccx/project.go` → 3 (the overlay). `applyEMEdit` now handles ONLY the 3 EM controls.
- [ ] **Step 4:** No commit (verification). Fix gaps via a focused TDD cycle.

---

## Self-Review (completed by plan author)

- **Spec coverage:** implements ADR-2 (study switches → SolverObject) — the smallest safe first slice of the remainder. The 3 fields move into `SolverObject` (S1), are overlaid, and their controls re-route to the aggregate, split out of `applyEMEdit` (S2). Behavior-preserving; equivalence-guarded; non-modal (ADR-3).
- **Placeholder scan:** none — fields, defaults, overlay, the new helper, and the dispatch rewire are fully coded. The only judgment call ("adjust to the exact surrounding control flow of the applyEMEdit call site") is an explicit locate-and-mirror instruction against a named function.
- **Type consistency:** `SolverObject.BodyScope` is a `string`, converted via `BodyScope(sv.BodyScope)` at the seam (mirrors `AnalysisType`/`MaterialModel`); the 3 fields used identically in S1/S2; `Solver()`/`SetSolver` reused; `applyAggStudySwitchEdit` mirrors `applyAggThermalMatEdit`.
- **Equivalence + lint:** S1 seeds the exact `defaultSettings()` values so the guard stays green; the EM cases remain in `applyEMEdit` (no `unused`); the new helper stays within funlen.

## Next slice
- **2.7** — the `ConstraintObject` spine (`femmodel/constraint_object.go` with a neutral `Kind string` + typed param union, per ADR-1) + `ccx/constraintmap.go` (`constraintSpecFor`), migrating `extras.Constraints`/`BuilderKind` off the projection side-channel. The crux; then 2.8–2.11 (load/support/thermal/field-drive param groups) and 2.12 (retire `extras` + `panel.go`).
