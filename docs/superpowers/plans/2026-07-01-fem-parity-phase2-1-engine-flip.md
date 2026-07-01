<!-- SPDX-License-Identifier: GPL-2.0-only -->
# FEM Parity — Phase 2.1 (engine ownership flip) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Flip the CalculiX add-in's `Engine` to hold a `*femmodel.Analysis` (the tree-owned source of truth) plus a flat `extras StudySettings` remainder, so panel edits and the solve pipeline read a projected `StudySettings` — activating the Phase-1 femmodel foundation with **no visible UI change**.

**Architecture:** Per the Phase-2 architecture brief (ADR-1: incremental migration, ccx-side remainder). `Engine.settings StudySettings` becomes `Engine.analysis *femmodel.Analysis` + `Engine.extras StudySettings`. A new `study()` method returns `projectAnalysis(analysis, extras)` under lock; `projectAnalysis` starts from `extras` and **overlays** the analysis-owned fields ("analysis wins"). The existing `reflect.DeepEqual` equivalence test is the guard rail. Editing stays in the existing dockable panel (ADR-3: non-modal; no dependency on the still-pending host modal task panel).

**Tech Stack:** Go; `oblikovati.org/calculix` add-in (cgo-free `ccx/` + pure `ccx/femmodel`); links only `oblikovati.org/api`.

## Global Constraints

- Every new `.go` file carries `// SPDX-License-Identifier: GPL-2.0-only`.
- `ccx/femmodel` stays PURE: imports neither the host nor `oblikovati.org/api` nor `ccx`. New mutators live there; the `StudyExtras`/`StudySettings` remainder lives in `ccx` (ADR-2).
- Dependency direction: `ccx → femmodel`, never the reverse.
- Style: functions 4–20 lines, files <500 lines, explicit types (no `any`), early returns, error messages include the offending value.
- TDD: failing test first, watched fail. This whole slice is behavior-preserving — the `reflect.DeepEqual` equivalence test + a "panel edit lands in the aggregate" test are the guards; the full existing suite (127 tests) MUST stay green.
- The projection overlay rule is FIXED: **analysis-owned fields always win** over `extras`. Do not add new fields to `extras`; it only shrinks in later slices.
- Run `go test ./...` + `golangci-lint run ./ccx/...` + `gofmt -l` before each commit. Coverage >80%, duplication <3%.
- Branch: `feature/fem-parity-phase2-engine-flip` (already created off `main`).

## Tree-owned control → femmodel object map (the split this slice implements)

| panel control id | routes to | femmodel field |
|---|---|---|
| `analysis` | Solver | `AnalysisType` (string) |
| `eigenmodes` | Solver | `Eigenmodes` |
| `transient_time` | Solver | `TransientTimeS` |
| `mesh_size` | Mesh | `MaxSizeMM` |
| `element_order` | Mesh | `Quadratic` (bool; QuadraticTet↔true) |
| `young` | default Material | `YoungGPa` |
| `poisson` | default Material | `Poisson` |
| `yield` | default Material | `YieldMPa` |
| `density` | default Material | `DensityGCm3` |
| `result_field` | primary Result | `Field` (string) |
| `deform_scale` | primary Result | `DeformScale` |

**Everything else** (`young_hot`, `hot_temp`, `material_model`, `neo_c10`, `neo_d1`, `load_type`, `load`, `pressure`, `gravity`, `rotation`, `displacement`, `alpha`, `delta_t`, `conductivity`, `cold_temp`, `heat_flux`, `heat_drive`, `film_coeff`, `sink_temp`, `body_heat`, `emissivity`, `rad_ambient`, `voltage`, `elec_sigma`, `em_drive`, `current_density`, `specific_heat`, `contact_mode`, `friction`, `support_type`, `spring_stiffness`, `hydro_gradient`, `hydro_surface`, `body_scope`, `builder_kind`, constraints) stays written to `e.extras`.

---

### Task P1: femmodel mutators for the singleton/default objects

**Files:**
- Modify: `ccx/femmodel/analysis.go` (add `SetDefaultMaterial`, `SetPrimaryResult`; `SetSolver`/`SetMesh` already exist)
- Test: `ccx/femmodel/analysis_mutators_test.go` (create)

**Interfaces:**
- Consumes: existing `SolverObject`/`MeshObject`/`MaterialObject`/`ResultObject`, `DefaultMaterial()`, `PrimaryResult()`, `SetSolver`/`SetMesh`.
- Produces: `(*Analysis).SetDefaultMaterial(m MaterialObject)` — replaces the `ScopeAll` fallback material's mechanical fields, **preserving its id and its `ScopeAll` flag** (so the ≥1-ScopeAll invariant holds); `(*Analysis).SetPrimaryResult(r ResultObject)` — replaces the first result's fields, **preserving its id** (keeps ≥1 Result).

- [ ] **Step 1: Write the failing test**

Create `ccx/femmodel/analysis_mutators_test.go`:
```go
// SPDX-License-Identifier: GPL-2.0-only

package femmodel

import "testing"

func TestSetDefaultMaterialPreservesIDAndScope(t *testing.T) {
	a := NewDefaultAnalysis()
	orig, _ := a.DefaultMaterial()
	repl := MaterialObject{YoungGPa: 69, Poisson: 0.33, DensityGCm3: 2.70, YieldMPa: 40}
	a.SetDefaultMaterial(repl)
	got, ok := a.DefaultMaterial()
	if !ok || got.ObjectID() != orig.ObjectID() {
		t.Fatalf("id not preserved: got %q want %q", got.ObjectID(), orig.ObjectID())
	}
	if !got.ScopeAll {
		t.Fatalf("ScopeAll not preserved: %+v", got)
	}
	if got.YoungGPa != 69 || got.Poisson != 0.33 || got.DensityGCm3 != 2.70 || got.YieldMPa != 40 {
		t.Fatalf("fields not updated: %+v", got)
	}
}

func TestSetPrimaryResultPreservesID(t *testing.T) {
	a := NewDefaultAnalysis()
	orig, _ := a.PrimaryResult()
	a.SetPrimaryResult(ResultObject{Field: "displacement magnitude", DeformScale: 5})
	got, ok := a.PrimaryResult()
	if !ok || got.ObjectID() != orig.ObjectID() {
		t.Fatalf("id not preserved: got %q want %q", got.ObjectID(), orig.ObjectID())
	}
	if got.Field != "displacement magnitude" || got.DeformScale != 5 {
		t.Fatalf("fields not updated: %+v", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /home/vmiguel/git/oblikovati-workspace/Oblikovati.AddIns.CalculiX && go test ./ccx/femmodel/ -run 'TestSetDefaultMaterial|TestSetPrimaryResult' -v`
Expected: FAIL — `undefined: (*Analysis).SetDefaultMaterial`.

- [ ] **Step 3: Write minimal implementation**

In `ccx/femmodel/analysis.go`, after `SetMesh` (~line 48):
```go
// SetDefaultMaterial replaces the ScopeAll fallback material's mechanical fields, preserving its
// id and ScopeAll flag (upholding the ≥1-ScopeAll-material invariant). If no ScopeAll material
// exists yet, it updates the first material.
func (a *Analysis) SetDefaultMaterial(m MaterialObject) {
	for i := range a.materials {
		if a.materials[i].ScopeAll {
			m.id, m.name, m.ScopeAll = a.materials[i].id, a.materials[i].name, true
			a.materials[i] = m
			return
		}
	}
	if len(a.materials) > 0 {
		m.id, m.name = a.materials[0].id, a.materials[0].name
		m.ScopeAll = a.materials[0].ScopeAll
		a.materials[0] = m
	}
}

// SetPrimaryResult replaces the first result object's fields, preserving its id (keeps ≥1 Result).
func (a *Analysis) SetPrimaryResult(r ResultObject) {
	if len(a.results) == 0 {
		return
	}
	r.id = a.results[0].id
	a.results[0] = r
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./ccx/femmodel/ -run 'TestSetDefaultMaterial|TestSetPrimaryResult' -v` → PASS. Then `go test ./ccx/femmodel/` → PASS.

- [ ] **Step 5: Commit**
```bash
git add ccx/femmodel/analysis.go ccx/femmodel/analysis_mutators_test.go
git commit -m "feat(femmodel): SetDefaultMaterial + SetPrimaryResult mutators (preserve id/scope)"
```

---

### Task P2: `projectAnalysis` overlay + Engine ownership flip

**Files:**
- Modify: `ccx/project.go` (`projectAnalysis` takes `extras StudySettings`, starts from it, overlays analysis-owned fields)
- Modify: `ccx/project_test.go` (both tests pass `defaultSettings()` as extras)
- Modify: `ccx/engine.go` (struct: `settings` → `analysis`+`extras`; `NewEngine`; add `study()`)
- Modify: `ccx/study.go` (the `settings := e.settings` read site → `settings, _ := e.study()`)
- Test: `ccx/engine_study_test.go` (create — `study()` projects defaults)

**Interfaces:**
- Consumes: `femmodel.NewDefaultAnalysis()`, `defaultSettings()`, the P1 mutators, existing `elementOrder(bool) ElementOrder`.
- Produces: `projectAnalysis(a *femmodel.Analysis, extras StudySettings) (StudySettings, []ConstraintSpec)`; `Engine{analysis *femmodel.Analysis; extras StudySettings; ...}`; `(*Engine).study() (StudySettings, []ConstraintSpec)` (locks, projects). The pipeline reads `StudySettings` ONLY through `study()`.

- [ ] **Step 1: Write the failing test**

Update `ccx/project_test.go` — change both call sites to pass extras:
```go
	got, specs := projectAnalysis(femmodel.NewDefaultAnalysis(), defaultSettings())
```
```go
	got, _ := projectAnalysis(a, defaultSettings())
```
Create `ccx/engine_study_test.go`:
```go
// SPDX-License-Identifier: GPL-2.0-only

package ccx

import (
	"reflect"
	"testing"
)

// study() must project the default Engine (default analysis + default extras) to exactly
// defaultSettings() — the behavior-preserving guard for the ownership flip.
func TestEngineStudyProjectsDefaults(t *testing.T) {
	e := NewEngine(nil)
	got, specs := e.study()
	if !reflect.DeepEqual(got, defaultSettings()) {
		t.Fatalf("study() drifted from defaults:\n got=%+v\nwant=%+v", got, defaultSettings())
	}
	if len(specs) != 0 {
		t.Fatalf("expected no constraints, got %d", len(specs))
	}
}
```
(`NewEngine(nil)` is safe here: `study()` touches only `analysis`/`extras`, no host call. If `NewEngine` dereferences the host eagerly, pass a trivial fake `HostCaller` instead — check `NewEngine`.)

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./ccx/ -run 'TestProject|TestEngineStudy'`
Expected: FAIL — `projectAnalysis` arity mismatch / `(*Engine).study` undefined.

- [ ] **Step 3: Write minimal implementation**

In `ccx/project.go`, replace `projectAnalysis` so it starts from `extras` and overlays:
```go
// projectAnalysis flattens the Analysis tree ONTO the flat extras remainder: it starts from extras
// (which supplies every not-yet-modeled field) and overlays the fields the tree owns — "analysis
// wins". This single seam keeps the mesh/deck/solve/render pipeline reading a plain StudySettings
// while the edit model is a tree. projectAnalysis(NewDefaultAnalysis(), defaultSettings()) reproduces
// defaultSettings() exactly (the equivalence guard).
func projectAnalysis(a *femmodel.Analysis, extras StudySettings) (StudySettings, []ConstraintSpec) {
	s := extras

	sv := a.Solver()
	s.Analysis = AnalysisType(sv.AnalysisType)
	s.Eigenmodes = sv.Eigenmodes
	s.TransientTimeS = sv.TransientTimeS

	m := a.Mesh()
	s.MeshSizeMM = m.MaxSizeMM
	s.ElementOrder = elementOrder(m.Quadratic)

	if mat, ok := a.DefaultMaterial(); ok {
		s.YoungGPa, s.Poisson, s.DensityGCm3, s.YieldMPa = mat.YoungGPa, mat.Poisson, mat.DensityGCm3, mat.YieldMPa
	}
	if r, ok := a.PrimaryResult(); ok {
		s.ResultField = ResultFieldKind(r.Field)
		s.DeformScale = r.DeformScale
	}
	return s, s.Constraints
}
```
In `ccx/engine.go`, replace the struct fields + `NewEngine` (the `settings StudySettings` field ~line 33) with:
```go
	mu       sync.Mutex
	analysis *femmodel.Analysis // tree-owned source of truth (Solver/Mesh/Material/Result)
	extras   StudySettings      // not-yet-modeled flat params; overlaid by projectAnalysis
	running  bool
```
```go
func NewEngine(host HostCaller) *Engine {
	return &Engine{host: host, api: client.New(host),
		analysis: femmodel.NewDefaultAnalysis(), extras: defaultSettings()}
}
```
Add (import `oblikovati.org/calculix/ccx/femmodel`):
```go
// study snapshots the study model under lock and projects it to the flat StudySettings the
// pipeline consumes — the ONE seam the mesh/deck/solve path reads.
func (e *Engine) study() (StudySettings, []ConstraintSpec) {
	e.mu.Lock()
	defer e.mu.Unlock()
	return projectAnalysis(e.analysis, e.extras)
}
```
In `ccx/study.go` `RunStudyOnHost`, replace the `e.mu.Lock(); settings := e.settings; e.mu.Unlock()` block (~line 88-90) with:
```go
	settings, _ := e.study()
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./ccx/ -run 'TestProject|TestEngineStudy'` → PASS. (Do NOT run the whole `ccx` suite yet — `applyPanelEdit`/`constraintbuilder` still reference `e.settings` and won't compile until P3. If the package fails to COMPILE here, that is expected; the focused test won't run until P3 removes the last `e.settings` references. To keep this task independently green, complete the `e.settings`→`e.extras`/`e.analysis` migration of applyPanelEdit + constraintbuilder + panel display **within P3 before committing P2's dependents** — SEE NOTE.)

**NOTE on task ordering:** P2 changes the `Engine` struct, which breaks every remaining `e.settings` reference (`panel.go`, `constraintbuilder.go`). Those are migrated in P3. To keep the repo compiling at each commit, **commit P2 and P3 together as one reviewable unit** if the package cannot compile between them — i.e. do P2's struct/projection change and P3's reference migration, then run the full suite, then make TWO commits (P2 message, then P3 message) only if the tree compiles after each; otherwise make a single squashed commit with both messages. Prefer two commits; fall back to one if compilation forces it.

- [ ] **Step 5: Commit** (see NOTE — may be combined with P3)
```bash
git add ccx/project.go ccx/project_test.go ccx/engine.go ccx/study.go ccx/engine_study_test.go
git commit -m "feat(ccx): Engine holds femmodel.Analysis + extras; projectAnalysis overlay + study()"
```

---

### Task P3: re-route panel edits + display through the aggregate

**Files:**
- Modify: `ccx/panel.go` (`ShowPanel`/`panelControls` read via `e.study()`; `applyPanelEdit` routes tree-owned controls to `e.analysis` via the P1 mutators, the rest to `e.extras`)
- Modify: `ccx/constraintbuilder.go` (`e.settings.Constraints`/`e.settings.BuilderKind` → `e.extras.*`)
- Modify: any remaining `e.settings` references (grep to be exhaustive)
- Test: `ccx/panel_routing_test.go` (create — a tree-owned edit lands in `e.analysis`; a remainder edit lands in `e.extras`)

**Interfaces:**
- Consumes: `e.analysis` mutators (P1: `SetSolver`/`SetMesh`/`SetDefaultMaterial`/`SetPrimaryResult`), `e.extras` fields, `e.study()`.
- Produces: `applyPanelEdit` mutates the aggregate for tree-owned controls and `extras` for the rest; the panel renders from the projected settings; `constraintbuilder` reads/writes `e.extras`.

- [ ] **Step 1: Write the failing test**

Create `ccx/panel_routing_test.go`:
```go
// SPDX-License-Identifier: GPL-2.0-only

package ccx

import "testing"

func TestPanelEditRoutesToAggregate(t *testing.T) {
	e := NewEngine(nil)
	e.applyPanelEdit("young", "123")
	if mat, _ := e.analysis.DefaultMaterial(); mat.YoungGPa != 123 {
		t.Fatalf("young edit did not land in the aggregate material: %+v", mat)
	}
	e.applyPanelEdit("analysis", "frequency")
	if e.analysis.Solver().AnalysisType != "frequency" {
		t.Fatalf("analysis edit did not land in the solver: %+v", e.analysis.Solver())
	}
	e.applyPanelEdit("element_order", "linear")
	if e.analysis.Mesh().Quadratic {
		t.Fatalf("element_order edit did not land in the mesh: %+v", e.analysis.Mesh())
	}
}

func TestPanelEditRoutesRemainderToExtras(t *testing.T) {
	e := NewEngine(nil)
	e.applyPanelEdit("gravity", "2.5")
	if e.extras.GravityG != 2.5 {
		t.Fatalf("gravity edit did not land in extras: %+v", e.extras.GravityG)
	}
	// And the projection reflects both homes.
	got, _ := e.study()
	if got.YoungGPa == 0 || got.GravityG != 2.5 {
		t.Fatalf("study() did not reflect aggregate+extras: young=%v gravity=%v", got.YoungGPa, got.GravityG)
	}
}
```
(If `e.analysis`/`e.extras` are unexported and this test is in package `ccx` — it is — direct field access is fine. `applyPanelEdit` must accept the control id + value as today; confirm its signature.)

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./ccx/ -run TestPanelEditRoutes`
Expected: FAIL to COMPILE (applyPanelEdit still writes `e.settings`) — fix by implementing Step 3.

- [ ] **Step 3: Write minimal implementation**

Re-route `applyPanelEdit` in `ccx/panel.go`. For the 11 tree-owned controls, read-modify-set the aggregate; for all others, keep writing `e.extras.<Field>` (rename `e.settings` → `e.extras` for those). Concretely, replace the tree-owned cases:
```go
	case "analysis":
		sv := e.analysis.Solver()
		sv.AnalysisType = strings.TrimSpace(value)
		e.analysis.SetSolver(sv)
	case "eigenmodes":
		sv := e.analysis.Solver()
		sv.Eigenmodes = int(panelNum(value, float64(sv.Eigenmodes)))
		e.analysis.SetSolver(sv)
	case "transient_time":
		sv := e.analysis.Solver()
		sv.TransientTimeS = panelNum(value, sv.TransientTimeS)
		e.analysis.SetSolver(sv)
	case "mesh_size":
		m := e.analysis.Mesh()
		m.MaxSizeMM = panelNum(value, m.MaxSizeMM)
		e.analysis.SetMesh(m)
	case "element_order":
		m := e.analysis.Mesh()
		m.Quadratic = parseElementOrder(value, elementOrder(m.Quadratic)) == QuadraticTet
		e.analysis.SetMesh(m)
	case "young":
		mat, _ := e.analysis.DefaultMaterial()
		mat.YoungGPa = panelNum(value, mat.YoungGPa)
		e.analysis.SetDefaultMaterial(mat)
	case "poisson":
		mat, _ := e.analysis.DefaultMaterial()
		mat.Poisson = panelNum(value, mat.Poisson)
		e.analysis.SetDefaultMaterial(mat)
	case "yield":
		mat, _ := e.analysis.DefaultMaterial()
		mat.YieldMPa = panelNum(value, mat.YieldMPa)
		e.analysis.SetDefaultMaterial(mat)
	case "density":
		mat, _ := e.analysis.DefaultMaterial()
		mat.DensityGCm3 = panelNum(value, mat.DensityGCm3)
		e.analysis.SetDefaultMaterial(mat)
	case "result_field":
		r, _ := e.analysis.PrimaryResult()
		r.Field = strings.TrimSpace(value)
		e.analysis.SetPrimaryResult(r)
	case "deform_scale":
		r, _ := e.analysis.PrimaryResult()
		r.DeformScale = panelNum(value, r.DeformScale)
		e.analysis.SetPrimaryResult(r)
```
Because each tree-owned case is now >2 lines, extract a helper per object group if `applyPanelEdit` exceeds the 20-line/func budget (e.g. `applySolverEdit`, `applyMeshEdit`, `applyMaterialEdit`, `applyResultEdit` dispatched from `applyPanelEdit`) — mirror the file's existing section-helper split. For ALL OTHER cases, change `e.settings.` → `e.extras.` (mechanical rename).
In `ccx/panel.go` `ShowPanel`, change the display read:
```go
	s, _ := e.study()
```
(remove the manual `e.mu.Lock(); s := e.settings; e.mu.Unlock()` — `study()` already locks).
In `ccx/constraintbuilder.go`, change `e.settings.Constraints`/`e.settings.BuilderKind` → `e.extras.Constraints`/`e.extras.BuilderKind` (grep the file). Run `grep -rn 'e\.settings' ccx/*.go` and migrate EVERY remaining reference to `e.extras` (remainder) or the aggregate (tree-owned) — there must be ZERO `e.settings` left.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./ccx/ -run TestPanelEditRoutes -v` → PASS. Then the FULL suite: `go test ./...` → all 127+ tests PASS (behavior preserved). Then `grep -rn 'e\.settings' ccx/*.go` → NO matches.

- [ ] **Step 5: Commit**
```bash
git add ccx/panel.go ccx/constraintbuilder.go ccx/panel_routing_test.go
git commit -m "feat(ccx): route panel edits + display through the Analysis aggregate + extras"
```

---

### Task P4: full verification gate

**Files:** none (verification).

- [ ] **Step 1:** `go test ./...` — all green (femmodel + ccx, incl. the 127 pre-existing).
- [ ] **Step 2:** `grep -rn 'e\.settings' ccx/*.go` — empty (the flat god-field is fully replaced).
- [ ] **Step 3:** `golangci-lint run ./ccx/...` — clean (funlen 30/20; split any over-long applyPanelEdit into per-object helpers). `gofmt -l ccx/` — empty.
- [ ] **Step 4:** `go test ./ccx/femmodel/ -cover` and `go test ./ccx/ -run 'TestProject|TestEngineStudy|TestPanelEditRoutes' -cover` — femmodel >80%.
- [ ] **Step 5:** No commit (verification). Fix any gap via a focused TDD cycle and commit that.

---

## Self-Review (completed by plan author)

- **Spec coverage:** implements Phase-2 ADR-1 sub-slice 2.1 (engine flip) exactly: `Engine.analysis + extras` (P2), `study()`/`projectAnalysis` overlay (P2), the femmodel mutators the panel needs (P1), and the panel/constraint re-routing (P3). No UI change (tree/ribbon are slices 2.2/2.3). Non-modal editing preserved (ADR-3).
- **Placeholder scan:** none — the tree-owned split is enumerated as a table + explicit cases; the only "locate" directive is `grep -rn 'e\.settings'` (an exhaustiveness command, not a TODO).
- **Type consistency:** `projectAnalysis(a, extras StudySettings)` and `study() (StudySettings, []ConstraintSpec)` used identically across P2/P3; `SetDefaultMaterial`/`SetPrimaryResult` (P1) consumed by P3; `elementOrder(bool)`/`parseElementOrder` reused for the `Quadratic`↔`ElementOrder` mapping.
- **Compilation ordering (called out in P2):** the struct change (P2) breaks `e.settings` refs until P3; commit P2+P3 so the tree compiles at the commit boundary (two commits if each compiles, one squashed if not).

## Next slices (separate plans, after 2.1 lands)
- **2.2** — read-only Analysis browser tree (`analysis_tree.go` + `analysis_tree_events.go`; node-id scheme; `EventBrowserNode` routing; double-click/menu → open the existing panel). Mirrors CAM `browser_tree.go`.
- **2.3** — FEA ribbon (`ribbon_layout.go`; `ccxRibbonSpots`; looped `RegisterCommands`). Mirrors CAM `ribbon_layout.go`.
- **2.4+** — field-group migrations (materials-thermal → EM → contact → constraints-into-femmodel → results-filters), each shrinking `extras` and the panel, guarded by the equivalence test, until `extras` is empty and `panel.go` retires.
