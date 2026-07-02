<!-- SPDX-License-Identifier: GPL-2.0-only -->
# FEM Parity Phase 2.10 — thermal-BC group → aggregate

> **For agentic workers:** REQUIRED SUB-SKILL: superpowers:subagent-driven-development.
> Steps use checkbox (`- [ ]`) syntax.

**Goal:** Move the 9 thermal boundary-condition fields (`DeltaK`, `ColdTempK`, `HeatFluxQ`,
`HeatDriveMode`, `FilmCoeff`, `SinkTempK`, `BodyHeatRate`, `Emissivity`, `RadAmbientK`) out of
`StudySettings.extras` into the `femmodel.Analysis` aggregate as a `ThermalDefaults` template,
and delete the two legacy routing helpers (`applyFieldBCEdit`, `applyHeatModeEdit`) that the
migration empties.

**Architecture:** Strangler migration — same aggregate-template pattern as 2.8 (load) and 2.9
(support). `Analysis` gains `ThermalDefaults` (`Thermal()`/`SetThermal()`), seeded in
`NewDefaultAnalysis`. `projectAnalysis` re-flattens via `overlayThermal`. The 9 panel controls
route to `applyAggThermalEdit` (+ a split `applyAggThermalModeEdit` for the 5 mode params, to
respect funlen). Because all thermal cases leave `applyFieldBCEdit`/`applyHeatModeEdit`, those
two helpers are deleted and their surviving delegation (study-switch, EM) folds into
`applyLoadEdit`.

**Tech Stack:** Go; `ccx/femmodel` pure (stdlib only — `HeatDriveMode` is a plain `string`
there); `ccx` links only `oblikovati.org/api`.

## Global Constraints

- `ccx/femmodel` imports ONLY stdlib. In femmodel, the heat-drive mode is a plain `string`.
- Equivalence guard `TestProjectDefaultAnalysisEqualsDefaultSettings`
  (`reflect.DeepEqual(projectAnalysis(NewDefaultAnalysis(), defaultSettings()), defaultSettings())`)
  MUST stay green at every commit.
- Seed values MUST equal `defaultSettings()`: `DeltaK=100`, `ColdTempK=0`, `HeatFluxQ=50`,
  `HeatDriveMode="flux"` (`HeatDriveFlux`), `FilmCoeff=0.5`, `SinkTempK=0`, `BodyHeatRate=1`,
  `Emissivity=0.8`, `RadAmbientK=300`.
- `applyPanelEdit` already holds `e.mu.Lock(); defer Unlock()` before dispatching — the new
  aggregate helpers run under that lock; do NOT add a second lock.
- Style: golangci funlen 30 lines / 20 statements; no `any`; explicit types; functions ≤20
  lines; every new `.go` carries `// SPDX-License-Identifier: GPL-2.0-only`.
- After migration: NO `e.extras.{DeltaK|ColdTempK|HeatFluxQ|HeatDriveMode|FilmCoeff|SinkTempK|BodyHeatRate|Emissivity|RadAmbientK}`
  write remains in `ccx/panel.go`; `applyFieldBCEdit` and `applyHeatModeEdit` are deleted with
  no dangling caller.

---

### Task T1: `femmodel.ThermalDefaults` template on `Analysis`

**Files:**
- Create: `ccx/femmodel/thermal_defaults.go`
- Modify: `ccx/femmodel/analysis.go` (add `thermal ThermalDefaults`; `Thermal()`/`SetThermal()`; seed)
- Test: `ccx/femmodel/thermal_defaults_test.go`

**Interfaces:**
- Produces:
  `type ThermalDefaults struct { HeatDriveMode string; DeltaK, ColdTempK, HeatFluxQ, FilmCoeff, SinkTempK, BodyHeatRate, Emissivity, RadAmbientK float64 }`;
  `func (a *Analysis) Thermal() ThermalDefaults`; `func (a *Analysis) SetThermal(ThermalDefaults)`.

- [ ] **Step 1: Write the failing test** (`ccx/femmodel/thermal_defaults_test.go`)

```go
// SPDX-License-Identifier: GPL-2.0-only
package femmodel

import "testing"

func TestDefaultThermal(t *testing.T) {
	th := NewDefaultAnalysis().Thermal()
	if th.HeatDriveMode != "flux" {
		t.Fatalf("HeatDriveMode = %q, want \"flux\"", th.HeatDriveMode)
	}
	if th.DeltaK != 100 || th.ColdTempK != 0 || th.HeatFluxQ != 50 {
		t.Fatalf("core temps = {%v %v %v}, want {100 0 50}", th.DeltaK, th.ColdTempK, th.HeatFluxQ)
	}
	if th.FilmCoeff != 0.5 || th.SinkTempK != 0 || th.BodyHeatRate != 1 {
		t.Fatalf("conv/body = {%v %v %v}, want {0.5 0 1}", th.FilmCoeff, th.SinkTempK, th.BodyHeatRate)
	}
	if th.Emissivity != 0.8 || th.RadAmbientK != 300 {
		t.Fatalf("radiation = {%v %v}, want {0.8 300}", th.Emissivity, th.RadAmbientK)
	}
}

func TestSetThermal(t *testing.T) {
	a := NewDefaultAnalysis()
	a.SetThermal(ThermalDefaults{HeatDriveMode: "convection", DeltaK: 5, ColdTempK: 1, HeatFluxQ: 2,
		FilmCoeff: 3, SinkTempK: 4, BodyHeatRate: 6, Emissivity: 0.2, RadAmbientK: 290})
	got := a.Thermal()
	if got.HeatDriveMode != "convection" || got.DeltaK != 5 || got.FilmCoeff != 3 || got.RadAmbientK != 290 {
		t.Fatalf("Thermal() = %+v, want mode=convection DeltaK=5 FilmCoeff=3 RadAmbientK=290", got)
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./ccx/femmodel/ -run 'TestDefaultThermal|TestSetThermal' -v`
Expected: FAIL — `ThermalDefaults`/`Thermal`/`SetThermal` undefined.

- [ ] **Step 3: Create `ccx/femmodel/thermal_defaults.go`**

```go
// SPDX-License-Identifier: GPL-2.0-only
package femmodel

// ThermalDefaults holds the thermal boundary-condition parameters synthesized at solve time
// for a heat-transfer / thermomechanical study. It is a study-wide template (not a browser-tree
// node), mirroring LoadDefaults and SupportDefaults. HeatDriveMode is a neutral string here —
// the ccx layer maps it to its HeatDrive display enum (flux/convection/body source/radiation).
type ThermalDefaults struct {
	HeatDriveMode string  // how loaded faces exchange heat: flux, convection, body source, radiation
	DeltaK        float64 // temperature change (K) for a thermomechanical study
	ColdTempK     float64 // prescribed temperature on the support face (K)
	HeatFluxQ     float64 // surface heat flux on the remaining faces
	FilmCoeff     float64 // convective film coefficient h (convection mode)
	SinkTempK     float64 // ambient/sink temperature for convection (K)
	BodyHeatRate  float64 // volumetric internal heat generation (body-source mode)
	Emissivity    float64 // surface emissivity 0..1 (radiation mode)
	RadAmbientK   float64 // ambient temperature radiated to (K) (radiation mode)
}
```

- [ ] **Step 4: Modify `ccx/femmodel/analysis.go`** — add field, accessors, seed.

Add to the `Analysis` struct (beside `support SupportDefaults`):

```go
	thermal ThermalDefaults
```

Add accessors (beside `Support`/`SetSupport`):

```go
// Thermal returns the thermal boundary-condition parameters.
func (a *Analysis) Thermal() ThermalDefaults { return a.thermal }

// SetThermal replaces the thermal boundary-condition parameters.
func (a *Analysis) SetThermal(t ThermalDefaults) { a.thermal = t }
```

Seed in `NewDefaultAnalysis` (right after the `a.SetSupport(...)` call):

```go
	a.SetThermal(ThermalDefaults{HeatDriveMode: "flux", DeltaK: 100, ColdTempK: 0, HeatFluxQ: 50,
		FilmCoeff: 0.5, SinkTempK: 0, BodyHeatRate: 1, Emissivity: 0.8, RadAmbientK: 300})
```

- [ ] **Step 5: Run to verify pass**

Run: `go test ./ccx/femmodel/ -run 'TestDefaultThermal|TestSetThermal' -v` → PASS.
Then `go test ./ccx/femmodel/` → PASS.

- [ ] **Step 6: Commit**

```bash
git add ccx/femmodel/thermal_defaults.go ccx/femmodel/thermal_defaults_test.go ccx/femmodel/analysis.go
git commit -m "feat(femmodel): ThermalDefaults template on Analysis (Thermal/SetThermal + seed)"
```

---

### Task T2: overlay + re-route the 9 thermal controls; delete the 2 legacy helpers

**Files:**
- Modify: `ccx/project.go` (add `overlayThermal(a, s) StudySettings`; call in `projectAnalysis`)
- Modify: `ccx/panel.go` (add `applyAggThermalEdit` + `applyAggThermalModeEdit`; delete
  `applyFieldBCEdit` + `applyHeatModeEdit`; reroute `applyLoadEdit`)
- Test: `ccx/panel_routing_test.go` (beside `TestSupportEditsRouteToAggregate`)

**Interfaces:**
- Consumes: `femmodel.Analysis.Thermal()` (T1), the `ccx.HeatDrive` display enum,
  the existing `applyAggStudySwitchEdit` and `applyEMEdit`.
- Produces: `func overlayThermal(a *femmodel.Analysis, s StudySettings) StudySettings`;
  `func (e *Engine) applyAggThermalEdit(controlID, value string) bool`;
  `func (e *Engine) applyAggThermalModeEdit(controlID, value string) bool`.

- [ ] **Step 1: Write the failing test** — add to `ccx/panel_routing_test.go` (uses `NewEngine(nil)` + `e.study()`):

```go
func TestThermalEditsRouteToAggregate(t *testing.T) {
	e := NewEngine(nil)
	e.applyPanelEdit("delta_t", "60")
	e.applyPanelEdit("cold_temp", "10")
	e.applyPanelEdit("heat_flux", "70")
	e.applyPanelEdit("heat_drive", "convection")
	e.applyPanelEdit("film_coeff", "1.5")
	e.applyPanelEdit("sink_temp", "20")
	e.applyPanelEdit("body_heat", "9")
	e.applyPanelEdit("emissivity", "0.3")
	e.applyPanelEdit("rad_ambient", "310")
	th := e.analysis.Thermal()
	if th.HeatDriveMode != "convection" || th.DeltaK != 60 || th.ColdTempK != 10 || th.HeatFluxQ != 70 ||
		th.FilmCoeff != 1.5 || th.SinkTempK != 20 || th.BodyHeatRate != 9 || th.Emissivity != 0.3 || th.RadAmbientK != 310 {
		t.Fatalf("thermal edits did not land in the aggregate: %+v", th)
	}
	s, _ := e.study()
	if s.HeatDriveMode != HeatDriveFilm || s.DeltaK != 60 || s.RadAmbientK != 310 {
		t.Fatalf("study() did not reflect thermal edits: %+v", s)
	}
}
```

> NOTE: `HeatDriveFilm == "convection"` (analysis.go). Confirm before using.

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./ccx/ -run TestThermalEditsRouteToAggregate -v`
Expected: FAIL — controls still write `e.extras`, so `e.analysis.Thermal()` is unchanged.

- [ ] **Step 3: Add `overlayThermal` to `ccx/project.go`** (beside `overlaySupport`):

```go
// overlayThermal copies the 9 thermal boundary-condition fields from the aggregate onto s.
func overlayThermal(a *femmodel.Analysis, s StudySettings) StudySettings {
	th := a.Thermal()
	s.HeatDriveMode = HeatDrive(th.HeatDriveMode)
	s.DeltaK, s.ColdTempK, s.HeatFluxQ = th.DeltaK, th.ColdTempK, th.HeatFluxQ
	s.FilmCoeff, s.SinkTempK, s.BodyHeatRate = th.FilmCoeff, th.SinkTempK, th.BodyHeatRate
	s.Emissivity, s.RadAmbientK = th.Emissivity, th.RadAmbientK
	return s
}
```

Wire into `projectAnalysis` right after `s = overlaySupport(a, s)`:

```go
	s = overlayThermal(a, s)
```

- [ ] **Step 4: Add the two aggregate helpers to `ccx/panel.go`** (beside `applyAggSupportEdit`):

```go
// applyAggThermalEdit routes the 4 core thermal controls (temperature delta, cold-face temp,
// surface flux, heat-drive mode) to the Analysis thermal template, delegating the 5 mode
// parameters to applyAggThermalModeEdit. Returns whether the control was recognised.
func (e *Engine) applyAggThermalEdit(controlID, value string) bool {
	if e.applyAggThermalModeEdit(controlID, value) {
		return true
	}
	th := e.analysis.Thermal()
	switch controlID {
	case "delta_t":
		th.DeltaK = panelNum(value, th.DeltaK)
	case "cold_temp":
		th.ColdTempK = panelNum(value, th.ColdTempK)
	case "heat_flux":
		th.HeatFluxQ = panelNum(value, th.HeatFluxQ)
	case "heat_drive":
		th.HeatDriveMode = strings.TrimSpace(value)
	default:
		return false
	}
	e.analysis.SetThermal(th)
	return true
}

// applyAggThermalModeEdit routes the 5 heat-drive-mode parameters (convection film + sink,
// body-source rate, radiation emissivity + ambient) to the Analysis thermal template.
func (e *Engine) applyAggThermalModeEdit(controlID, value string) bool {
	th := e.analysis.Thermal()
	switch controlID {
	case "film_coeff":
		th.FilmCoeff = panelNum(value, th.FilmCoeff)
	case "sink_temp":
		th.SinkTempK = panelNum(value, th.SinkTempK)
	case "body_heat":
		th.BodyHeatRate = panelNum(value, th.BodyHeatRate)
	case "emissivity":
		th.Emissivity = panelNum(value, th.Emissivity)
	case "rad_ambient":
		th.RadAmbientK = panelNum(value, th.RadAmbientK)
	default:
		return false
	}
	e.analysis.SetThermal(th)
	return true
}
```

- [ ] **Step 5: Delete `applyFieldBCEdit` and `applyHeatModeEdit`; reroute `applyLoadEdit`.**

Replace the whole `applyLoadEdit` (+ delete both legacy helpers) with:

```go
// applyLoadEdit routes the non-tree study controls: support and thermal BCs reach the femmodel
// aggregate; the study-wide switches reach the SolverObject; the remaining EM controls write to
// e.extras (still to be migrated). Each helper returns whether it matched, so the first match wins.
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
	e.applyEMEdit(controlID, value)
}
```

Then DELETE the `applyFieldBCEdit` and `applyHeatModeEdit` functions entirely (their thermal
cases moved to the two new helpers; their study-switch + EM delegation is now in `applyLoadEdit`).
Confirm no dangling caller: `grep -n 'applyFieldBCEdit\|applyHeatModeEdit' ccx/`.

- [ ] **Step 6: Run to verify pass**

Run: `go test ./ccx/ -run TestThermalEditsRouteToAggregate -v` → PASS.
Then `go test ./ccx/...` (equivalence guard included) → PASS.
Then `golangci-lint run ./ccx/...` → clean (watch for unused symbols / funlen).
Then `gofmt -l ccx/` → empty.

- [ ] **Step 7: Commit**

```bash
git add ccx/project.go ccx/panel.go ccx/*_test.go
git commit -m "feat(ccx): route thermal-BC params to the aggregate + overlay; retire field-BC/heat-mode helpers"
```

---

### Task T3: verification gate (no commit)

- [ ] **Step 1:** `go test ./...` + `go test -race ./ccx/...` — all green.
- [ ] **Step 2:** `golangci-lint run ./ccx/...` clean; `gofmt -l ccx/` empty.
- [ ] **Step 3:** Migration proof:
  - `grep -nE 'e\.extras\.(DeltaK|ColdTempK|HeatFluxQ|HeatDriveMode|FilmCoeff|SinkTempK|BodyHeatRate|Emissivity|RadAmbientK)' ccx/panel.go` → **empty**.
  - `grep -n 'applyFieldBCEdit\|applyHeatModeEdit' ccx/` → **empty** (both deleted, no dangling caller).
  - `overlayThermal` sets all 9 in `project.go`; `projectAnalysis` calls it.
- [ ] **Step 4:** No commit (verification only).
