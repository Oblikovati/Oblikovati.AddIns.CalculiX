<!-- SPDX-License-Identifier: GPL-2.0-only -->
# FEM Parity — Phase 2.5 (complete the materials migration) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Migrate the **last 6 material fields** out of `extras StudySettings` into `femmodel.MaterialObject` — electrical (`ElectricalSigma`), hyperelastic (`MaterialModel`, `NeoHookeC10`, `NeoHookeD1`), and temperature-dependent elasticity (`YoungHotGPa`, `HotTempK`) — so the **entire material is aggregate-owned** and every material panel control edits the aggregate. Completes the materials migration begun in 2.1 (mechanical) + 2.4 (thermal).

**Architecture:** The proven 2.4 pattern, ×6: add the fields to `MaterialObject`; seed defaults in `NewDefaultAnalysis`; `projectAnalysis` **overlays** them ("analysis wins"); consolidate the material panel controls that still write `e.extras` (`applyElecMatEdit` + `applyHyperelasticEdit` + the `young_hot`/`hot_temp` cases) into a single **aggregate** helper. `MaterialModel` is a ccx enum, so `femmodel` holds it as a plain `string` (like `SolverObject.AnalysisType` already does) and `projectAnalysis` converts it back. Behavior-preserving; the `reflect.DeepEqual` equivalence test is the guard. Panel stays editable (ADR-3).

**Tech Stack:** Go; `oblikovati.org/calculix` add-in (pure `ccx/femmodel` + `ccx`); links only `oblikovati.org/api`.

## Global Constraints

- Every new `.go` file carries `// SPDX-License-Identifier: GPL-2.0-only`.
- `ccx/femmodel` stays PURE (stdlib only) — it holds `MaterialModel` as a `string`, NOT the ccx `MaterialModel` enum (that would invert the dependency).
- Style: functions 4–20 lines, files <500 lines, explicit types, early returns.
- **Behavior-preserving:** the equivalence test `TestProjectDefaultAnalysisEqualsDefaultSettings` and the full suite MUST stay green; the overlay is total (all 6 fields overlaid so their dead `extras` copies never leak into the solve).
- Do NOT remove the fields from `StudySettings` (the deck/solve pipeline reads them); the migration moves their SOURCE OF TRUTH into the aggregate.
- Run `go test ./...` + `golangci-lint run ./ccx/...` (watch `unused` — deleting the extras helpers must not orphan anything) + `gofmt -l` before each commit.
- Branch: `feature/fem-parity-phase2-5-materials-complete` (already off the merged `main`).

## Anchors (verified)

- `femmodel.MaterialObject` currently: `id, name string; YoungGPa, Poisson, DensityGCm3, YieldMPa, ThermalAlpha, Conductivity, SpecificHeat float64; ScopeAll bool`. `SetDefaultMaterial`/`DefaultMaterial` replace/return the whole object.
- `ccx.MaterialModel string` enum: `MaterialLinear = "linear elastic"`, `MaterialNeoHooke = "neo-hookean (rubber)"`.
- Defaults (`ccx/analysis.go`): `ElectricalSigma = 1`, `MaterialModel = MaterialLinear`, `NeoHookeC10 = 1.0`, `NeoHookeD1 = 0.1`, `YoungHotGPa = 0`, `HotTempK = 100`.
- `ccx/panel.go` post-2.4 material dispatch (`applyMaterialEdit`): `applyHyperelasticEdit` (`material_model`/`neo_c10`/`neo_d1` → `e.extras`); a switch with `young_hot`/`hot_temp` (→ `e.extras`); `applyElecMatEdit` (`elec_sigma` → `e.extras`). Mechanical → `applyAggMechanicalMatEdit`; thermal → `applyAggThermalMatEdit` (both aggregate). Aggregate helpers use the read-once-`DefaultMaterial` / switch / `SetDefaultMaterial`-once pattern.
- `ccx/project.go` material overlay block sets Young/Poisson/Density/Yield + Thermal/Conductivity/SpecificHeat from the default material.

---

### Task N1: extend `MaterialObject` with the EM/hyper/temp-dep fields + seed

**Files:**
- Modify: `ccx/femmodel/material_object.go` (add 6 fields)
- Modify: `ccx/femmodel/analysis.go` (`NewDefaultAnalysis` seeds their defaults on the default material)
- Test: `ccx/femmodel/material_complete_test.go` (create)

**Interfaces:**
- Produces: `MaterialObject.ElectricalSigma, .NeoHookeC10, .NeoHookeD1, .YoungHotGPa, .HotTempK float64` and `.MaterialModel string`; the default material carries `ElectricalSigma=1, MaterialModel="linear elastic", NeoHookeC10=1.0, NeoHookeD1=0.1, YoungHotGPa=0, HotTempK=100`.

- [ ] **Step 1: Write the failing test**

Create `ccx/femmodel/material_complete_test.go`:
```go
// SPDX-License-Identifier: GPL-2.0-only

package femmodel

import "testing"

func TestDefaultMaterialHasEMHyperTempDefaults(t *testing.T) {
	mat, ok := NewDefaultAnalysis().DefaultMaterial()
	if !ok {
		t.Fatal("no default material")
	}
	if mat.ElectricalSigma != 1 || mat.MaterialModel != "linear elastic" {
		t.Fatalf("EM/model defaults wrong: sigma=%g model=%q", mat.ElectricalSigma, mat.MaterialModel)
	}
	if mat.NeoHookeC10 != 1.0 || mat.NeoHookeD1 != 0.1 {
		t.Fatalf("neo-hooke defaults wrong: c10=%g d1=%g", mat.NeoHookeC10, mat.NeoHookeD1)
	}
	if mat.YoungHotGPa != 0 || mat.HotTempK != 100 {
		t.Fatalf("temp-dep defaults wrong: hot=%g tk=%g", mat.YoungHotGPa, mat.HotTempK)
	}
}

func TestSetDefaultMaterialCarriesEMHyperTemp(t *testing.T) {
	a := NewDefaultAnalysis()
	mat, _ := a.DefaultMaterial()
	mat.ElectricalSigma, mat.MaterialModel = 2, "neo-hookean (rubber)"
	mat.NeoHookeC10, mat.NeoHookeD1, mat.YoungHotGPa, mat.HotTempK = 3, 0.2, 150, 400
	a.SetDefaultMaterial(mat)
	got, _ := a.DefaultMaterial()
	if got.ElectricalSigma != 2 || got.MaterialModel != "neo-hookean (rubber)" ||
		got.NeoHookeC10 != 3 || got.NeoHookeD1 != 0.2 || got.YoungHotGPa != 150 || got.HotTempK != 400 {
		t.Fatalf("fields not persisted: %+v", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /home/vmiguel/git/oblikovati-workspace/Oblikovati.AddIns.CalculiX && go test ./ccx/femmodel/ -run 'TestDefaultMaterialHasEMHyperTemp|TestSetDefaultMaterialCarriesEMHyperTemp'`
Expected: FAIL — `mat.ElectricalSigma undefined`.

- [ ] **Step 3: Write minimal implementation**

In `ccx/femmodel/material_object.go`, add after `SpecificHeat` (keep `ScopeAll` last):
```go
	ElectricalSigma float64 // electrical conductivity (consistent units; electrostatic study)
	MaterialModel   string  // constitutive law name: "linear elastic" | "neo-hookean (rubber)"
	NeoHookeC10     float64 // Neo-Hookean C10 (MPa), for the hyperelastic model
	NeoHookeD1      float64 // Neo-Hookean D1 (1/MPa) compressibility, for the hyperelastic model
	YoungHotGPa     float64 // Young's modulus (GPa) at HotTempK; >0 builds a temperature-dependent E(T) table
	HotTempK        float64 // upper table temperature (K) at which YoungHotGPa applies
```
In `ccx/femmodel/analysis.go` `NewDefaultAnalysis`, extend the existing thermal-seed block (after `SetDefaultMaterial(steel)` or by adding to the same `steel` before storing) so the default material also carries the EM/hyper/temp-dep defaults. RECOMMENDED — set all non-mechanical defaults on `steel` before the single `SetDefaultMaterial`:
```go
	steel, _ := a.DefaultMaterial()
	steel.ThermalAlpha, steel.Conductivity, steel.SpecificHeat = 1.2e-5, 50, 5e8
	steel.ElectricalSigma, steel.MaterialModel = 1, "linear elastic"
	steel.NeoHookeC10, steel.NeoHookeD1 = 1.0, 0.1
	steel.YoungHotGPa, steel.HotTempK = 0, 100
	a.SetDefaultMaterial(steel)
```
(Merge with the 2.4 thermal-seed lines that already exist — do not double-call `DefaultMaterial`/`SetDefaultMaterial`.)

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./ccx/femmodel/ -run 'TestDefaultMaterialHasEMHyperTemp|TestSetDefaultMaterialCarriesEMHyperTemp' -v` → PASS. Then `go test ./ccx/femmodel/` → PASS.

- [ ] **Step 5: Commit**
```bash
git add ccx/femmodel/material_object.go ccx/femmodel/analysis.go ccx/femmodel/material_complete_test.go
git commit -m "feat(femmodel): MaterialObject carries EM/hyperelastic/temp-dependent props"
```

---

### Task N2: overlay + consolidate all material controls onto the aggregate

**Files:**
- Modify: `ccx/project.go` (`projectAnalysis` overlays the 6 fields; `MaterialModel` via `MaterialModel(mat.MaterialModel)`)
- Modify: `ccx/panel.go` (route the 6 controls to a new aggregate helper; delete/empty the extras helpers)
- Test: `ccx/panel_routing_test.go` (add the routing assertions) + equivalence test stays green untouched

**Interfaces:**
- Consumes: the N1 fields; `SetDefaultMaterial`/`DefaultMaterial`; `ccx.MaterialModel`.
- Produces: `projectAnalysis` sets `s.ElectricalSigma`/`s.MaterialModel`/`s.NeoHookeC10`/`s.NeoHookeD1`/`s.YoungHotGPa`/`s.HotTempK` from the default material; `applyPanelEdit` routes `elec_sigma`/`material_model`/`neo_c10`/`neo_d1`/`young_hot`/`hot_temp` to `e.analysis` (no `e.extras` writes for any material control).

- [ ] **Step 1: Write the failing test**

In `ccx/panel_routing_test.go`, add:
```go
func TestEMHyperTempMaterialEditsRouteToAggregate(t *testing.T) {
	e := NewEngine(nil)
	e.applyPanelEdit("elec_sigma", "2")
	e.applyPanelEdit("material_model", "neo-hookean (rubber)")
	e.applyPanelEdit("neo_c10", "3")
	e.applyPanelEdit("neo_d1", "0.2")
	e.applyPanelEdit("young_hot", "150")
	e.applyPanelEdit("hot_temp", "400")
	mat, _ := e.analysis.DefaultMaterial()
	if mat.ElectricalSigma != 2 || mat.MaterialModel != "neo-hookean (rubber)" ||
		mat.NeoHookeC10 != 3 || mat.NeoHookeD1 != 0.2 || mat.YoungHotGPa != 150 || mat.HotTempK != 400 {
		t.Fatalf("edits did not land in the aggregate material: %+v", mat)
	}
	s, _ := e.study()
	if s.ElectricalSigma != 2 || string(s.MaterialModel) != "neo-hookean (rubber)" ||
		s.NeoHookeC10 != 3 || s.YoungHotGPa != 150 {
		t.Fatalf("study() did not reflect the edits: %+v", s)
	}
}
```
The existing `TestProjectDefaultAnalysisEqualsDefaultSettings` must STILL pass unchanged (N1 seeded the exact defaults). If it goes red, STOP — N1 seeding is wrong.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./ccx/ -run 'TestEMHyperTempMaterialEditsRouteToAggregate|TestProjectDefaultAnalysisEqualsDefaultSettings'`
Expected: the new test FAILS (edits still land in `e.extras`); equivalence PASSES.

- [ ] **Step 3: Write minimal implementation**

In `ccx/project.go`, extend the material overlay block (after `s.SpecificHeat = mat.SpecificHeat`):
```go
		s.ElectricalSigma = mat.ElectricalSigma
		s.MaterialModel = MaterialModel(mat.MaterialModel)
		s.NeoHookeC10 = mat.NeoHookeC10
		s.NeoHookeD1 = mat.NeoHookeD1
		s.YoungHotGPa = mat.YoungHotGPa
		s.HotTempK = mat.HotTempK
```
In `ccx/panel.go`, add ONE aggregate helper covering all 6 controls (read-once / switch / set-once, mirroring `applyAggThermalMatEdit`):
```go
// applyAggEMHyperMatEdit routes the electrical, hyperelastic, and temperature-dependent-elasticity
// material controls to the Analysis aggregate. Returns whether it matched.
func (e *Engine) applyAggEMHyperMatEdit(controlID, value string) bool {
	mat, ok := e.analysis.DefaultMaterial()
	if !ok {
		return false
	}
	switch controlID {
	case "elec_sigma":
		mat.ElectricalSigma = panelNum(value, mat.ElectricalSigma)
	case "material_model":
		mat.MaterialModel = strings.TrimSpace(value)
	case "neo_c10":
		mat.NeoHookeC10 = panelNum(value, mat.NeoHookeC10)
	case "neo_d1":
		mat.NeoHookeD1 = panelNum(value, mat.NeoHookeD1)
	case "young_hot":
		mat.YoungHotGPa = panelNum(value, mat.YoungHotGPa)
	case "hot_temp":
		mat.HotTempK = panelNum(value, mat.HotTempK)
	default:
		return false
	}
	e.analysis.SetDefaultMaterial(mat)
	return true
}
```
Then in `applyMaterialEdit`, replace the calls to `applyHyperelasticEdit`, `applyElecMatEdit`, and the inline `young_hot`/`hot_temp` switch cases with a single `if e.applyAggEMHyperMatEdit(controlID, value) { return true }`, and **DELETE** the now-unused `applyHyperelasticEdit` and `applyElecMatEdit` functions (they have no remaining cases). Verify `applyMaterialEdit` now dispatches ONLY to the three aggregate helpers (mechanical, thermal, EM/hyper/temp) and no material control writes `e.extras`. Keep every helper ≤20 lines.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./ccx/ -run 'TestEMHyperTempMaterialEditsRouteToAggregate|TestProjectDefaultAnalysisEqualsDefaultSettings' -v` → PASS. Then `go test ./...` → all green. Verify NO material control still writes extras:
`grep -nE 'e\.extras\.(ElectricalSigma|MaterialModel|NeoHooke|YoungHotGPa|HotTempK)' ccx/panel.go` → empty. `golangci-lint run ./ccx/...` → clean (no `unused` — the deleted helpers must be fully removed, not orphaned).

- [ ] **Step 5: Commit**
```bash
git add ccx/project.go ccx/panel.go ccx/panel_routing_test.go
git commit -m "feat(ccx): route all remaining material edits to the aggregate; retire the extras material helpers"
```

---

### Task N3: verification gate

- [ ] **Step 1:** `go test ./...` — all green (incl. the equivalence guard).
- [ ] **Step 2:** `golangci-lint run ./ccx/...` clean (watch `unused` from the deleted helpers + funlen); `gofmt -l ccx/` empty.
- [ ] **Step 3:** Migration proof — the ENTIRE material is now aggregate-owned:
  `grep -nE 'e\.extras\.(YoungGPa|Poisson|DensityGCm3|YieldMPa|ThermalAlpha|Conductivity|SpecificHeat|ElectricalSigma|MaterialModel|NeoHooke|YoungHotGPa|HotTempK)' ccx/panel.go` → **empty** (no material field written to extras anywhere).
  `grep -c 'mat\.' ccx/project.go` → shows the full 13-field material overlay.
- [ ] **Step 4:** No commit (verification). Fix gaps via a focused TDD cycle.

---

## Self-Review (completed by plan author)

- **Spec coverage:** completes the materials migration — the 6 remaining material fields (EM/hyper/temp-dep) move into `MaterialObject` (N1), are overlaid, and their panel controls consolidate onto one aggregate helper with the extras helpers deleted (N2). After 2.5 the entire material is aggregate-owned; behavior-preserving (equivalence guard); panel editable (ADR-3).
- **Placeholder scan:** none — fields, defaults, overlay, the consolidated helper, and the enum-as-string conversion are fully coded. The only judgment call ("delete the now-unused extras helpers; keep helpers ≤20 lines") is an explicit refactor+lint instruction.
- **Type consistency:** `MaterialModel` is `string` in `femmodel`, converted via `MaterialModel(mat.MaterialModel)` at the seam (mirrors `AnalysisType`); the 6 fields used identically in N1/N2; `SetDefaultMaterial`/`DefaultMaterial` reused.
- **Equivalence guarded + lint:** N1 seeds the exact `defaultSettings()` values so the guard stays green; N2 deletes the extras helpers fully (no `unused`), and the read-once/switch/set-once helper stays within funlen.

## Next slices
- **2.6** — contact params (`ContactMode`, `FrictionMu`) — these are NOT material or per-object; decide their home (a Contact/interface object, or keep in extras until a Contact object slice). **2.7** — constraints-into-`femmodel` (needs the `ConstraintKind` ownership decision). **2.8** — results. Then `extras` is empty → **retire `panel.go`** + delete the dead `StudySettings` fields. Plus the icon pass.
