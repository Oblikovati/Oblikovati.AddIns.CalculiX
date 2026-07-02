<!-- SPDX-License-Identifier: GPL-2.0-only -->
# FEM Parity Phase 2.9 — default-support params → aggregate

> **For agentic workers:** REQUIRED SUB-SKILL: superpowers:subagent-driven-development.
> Steps use checkbox (`- [ ]`) syntax.

**Goal:** Move the 2 mechanical default-support fields (`SupportType`, `SpringStiffMM`)
out of the flat `StudySettings.extras` DTO into the `femmodel.Analysis` aggregate, exactly
as slice 2.8 did for the default-load params.

**Architecture:** Strangler migration. `femmodel.Analysis` gains a `SupportDefaults` template
(`Support()`/`SetSupport()`), seeded in `NewDefaultAnalysis`. `projectAnalysis(a, extras)`
re-flattens the aggregate onto `StudySettings` via a new `overlaySupport` helper, so the
deck/solve pipeline is byte-for-byte unchanged. The 2 support panel controls route to a new
`applyAggSupportEdit` instead of `e.extras`.

**Tech Stack:** Go; `ccx/femmodel` stays pure (stdlib only — `SupportType` is a plain
`string` there); `ccx` links only `oblikovati.org/api`.

## Global Constraints

- `ccx/femmodel` imports ONLY stdlib. In femmodel, `SupportType` is a plain `string`.
- The equivalence guard `TestProjectDefaultAnalysisEqualsDefaultSettings`
  (`reflect.DeepEqual(projectAnalysis(NewDefaultAnalysis(), defaultSettings()), defaultSettings())`)
  MUST stay green at every commit — the behavioral anchor.
- Seed values MUST equal `defaultSettings()`: `SupportType="fixed"` (`SupportFixed`),
  `SpringStiffMM=1000`.
- `applyPanelEdit` already holds `e.mu.Lock(); defer Unlock()` before dispatching — the new
  `applyAggSupportEdit` runs under that lock; do NOT add a second lock.
- Style: golangci funlen 30 lines / 20 statements; no `any`; explicit types; functions ≤20
  lines; every new `.go` carries `// SPDX-License-Identifier: GPL-2.0-only`.
- After the migration, NO `e.extras.SupportType` / `e.extras.SpringStiffMM` write remains in
  `ccx/panel.go`.

---

### Task S1: `femmodel.SupportDefaults` template on `Analysis`

**Files:**
- Create: `ccx/femmodel/support_defaults.go`
- Modify: `ccx/femmodel/analysis.go` (add `support SupportDefaults` field; `Support()`/`SetSupport()`; seed in `NewDefaultAnalysis`)
- Test: `ccx/femmodel/support_defaults_test.go`

**Interfaces:**
- Produces: `type SupportDefaults struct { SupportType string; SpringStiffMM float64 }`;
  `func (a *Analysis) Support() SupportDefaults`; `func (a *Analysis) SetSupport(SupportDefaults)`.

- [ ] **Step 1: Write the failing test** (`ccx/femmodel/support_defaults_test.go`)

```go
// SPDX-License-Identifier: GPL-2.0-only
package femmodel

import "testing"

func TestDefaultSupport(t *testing.T) {
	s := NewDefaultAnalysis().Support()
	if s.SupportType != "fixed" {
		t.Fatalf("SupportType = %q, want \"fixed\"", s.SupportType)
	}
	if s.SpringStiffMM != 1000 {
		t.Fatalf("SpringStiffMM = %v, want 1000", s.SpringStiffMM)
	}
}

func TestSetSupport(t *testing.T) {
	a := NewDefaultAnalysis()
	a.SetSupport(SupportDefaults{SupportType: "elastic (spring)", SpringStiffMM: 42})
	got := a.Support()
	if got.SupportType != "elastic (spring)" || got.SpringStiffMM != 42 {
		t.Fatalf("Support() = %+v, want {elastic (spring) 42}", got)
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./ccx/femmodel/ -run 'TestDefaultSupport|TestSetSupport' -v`
Expected: FAIL — `SupportDefaults`/`Support`/`SetSupport` undefined.

- [ ] **Step 3: Create `ccx/femmodel/support_defaults.go`**

```go
// SPDX-License-Identifier: GPL-2.0-only
package femmodel

// SupportDefaults holds the mechanical support parameters synthesized at solve time
// (the first selected face is the support). It is not a browser-tree node; it is a
// study-wide template, mirroring LoadDefaults. SupportType is a neutral string here —
// the ccx layer maps it to its SupportType display enum.
type SupportDefaults struct {
	SupportType   string  // "fixed" clamps; "elastic (spring)" rests on a grounded *SPRING
	SpringStiffMM float64 // total elastic-support stiffness (N/mm) for the elastic type
}
```

- [ ] **Step 4: Modify `ccx/femmodel/analysis.go`** — add the field, accessors, and seed.

Add to the `Analysis` struct (beside `load LoadDefaults`):

```go
	support SupportDefaults
```

Add accessors (beside `Load`/`SetLoad`):

```go
// Support returns the default-support parameters.
func (a *Analysis) Support() SupportDefaults { return a.support }

// SetSupport replaces the default-support parameters.
func (a *Analysis) SetSupport(s SupportDefaults) { a.support = s }
```

Seed in `NewDefaultAnalysis` (right after the `a.SetLoad(...)` call):

```go
	a.SetSupport(SupportDefaults{SupportType: "fixed", SpringStiffMM: 1000})
```

- [ ] **Step 5: Run to verify pass**

Run: `go test ./ccx/femmodel/ -run 'TestDefaultSupport|TestSetSupport' -v` → PASS.
Then `go test ./ccx/femmodel/` → PASS.

- [ ] **Step 6: Commit**

```bash
git add ccx/femmodel/support_defaults.go ccx/femmodel/support_defaults_test.go ccx/femmodel/analysis.go
git commit -m "feat(femmodel): SupportDefaults template on Analysis (Support/SetSupport + seed)"
```

---

### Task S2: overlay + re-route the 2 support controls

**Files:**
- Modify: `ccx/project.go` (add `overlaySupport(a, s) StudySettings`; call it in `projectAnalysis`)
- Modify: `ccx/panel.go` (new `applyAggSupportEdit`; remove the 2 cases from `applySupportEdit`)
- Test: `ccx/panel_routing_test.go` (beside `TestLoadEditsRouteToAggregate`)

**Interfaces:**
- Consumes: `femmodel.Analysis.Support()` (from S1), the `ccx.SupportType` display enum.
- Produces: `func overlaySupport(a *femmodel.Analysis, s StudySettings) StudySettings`;
  `func (e *Engine) applyAggSupportEdit(controlID, value string) bool`.

- [ ] **Step 1: Write the failing test** — add to `ccx/panel_routing_test.go`, mirroring
  `TestLoadEditsRouteToAggregate` (which uses `NewEngine(nil)` + `e.study()`):

```go
func TestSupportEditsRouteToAggregate(t *testing.T) {
	e := NewEngine(nil)
	e.applyPanelEdit("support_type", "elastic (spring)")
	e.applyPanelEdit("spring_stiffness", "250")

	sup := e.analysis.Support()
	if sup.SupportType != "elastic (spring)" {
		t.Fatalf("aggregate SupportType = %q, want \"elastic (spring)\"", sup.SupportType)
	}
	if sup.SpringStiffMM != 250 {
		t.Fatalf("aggregate SpringStiffMM = %v, want 250", sup.SpringStiffMM)
	}
	s, _ := e.study()
	if s.SupportType != SupportElastic || s.SpringStiffMM != 250 {
		t.Fatalf("study() support = {%v %v}, want {elastic 250}", s.SupportType, s.SpringStiffMM)
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./ccx/ -run TestSupportEditsRouteToAggregate -v`
Expected: FAIL — controls still write `e.extras`, so `e.analysis.Support()` is unchanged.

- [ ] **Step 3: Add `overlaySupport` to `ccx/project.go`** (place beside `overlayLoad`):

```go
// overlaySupport copies the 2 default-support fields from the Analysis aggregate onto s.
func overlaySupport(a *femmodel.Analysis, s StudySettings) StudySettings {
	sup := a.Support()
	s.SupportType = SupportType(sup.SupportType)
	s.SpringStiffMM = sup.SpringStiffMM
	return s
}
```

Wire it into `projectAnalysis` right after the `s = overlayLoad(a, s)` line:

```go
	s = overlaySupport(a, s)
```

- [ ] **Step 4: Add `applyAggSupportEdit` to `ccx/panel.go` and remove the 2 cases from `applySupportEdit`.**

New helper (place beside `applyAggLoadEdit`):

```go
// applyAggSupportEdit routes the 2 support controls (clamp vs elastic spring) to the
// Analysis support template. Returns whether the control was recognised.
func (e *Engine) applyAggSupportEdit(controlID, value string) bool {
	sup := e.analysis.Support()
	switch controlID {
	case "support_type":
		sup.SupportType = strings.TrimSpace(value)
	case "spring_stiffness":
		sup.SpringStiffMM = panelNum(value, sup.SpringStiffMM)
	default:
		return false
	}
	e.analysis.SetSupport(sup)
	return true
}
```

Route it: in `applyLoadEdit`, try the aggregate support edit BEFORE the legacy one, and
strip the 2 migrated cases out of `applySupportEdit` (leaving it as a pure fall-through that
now matches nothing — so DELETE `applySupportEdit` entirely and inline its former role):

```go
func (e *Engine) applyLoadEdit(controlID, value string) {
	if e.applyAggSupportEdit(controlID, value) {
		return
	}
	e.applyFieldBCEdit(controlID, value)
}
```

Delete the now-empty `applySupportEdit` function. Confirm no other caller references it
(`grep -n applySupportEdit ccx/`).

- [ ] **Step 5: Run to verify pass**

Run: `go test ./ccx/ -run TestSupportEditsRouteToAggregate -v` → PASS.
Then `go test ./ccx/...` (equivalence guard included) → PASS.
Then `golangci-lint run ./ccx/...` → clean (watch for unused `applySupportEdit` if you left a stub).

- [ ] **Step 6: Commit**

```bash
git add ccx/project.go ccx/panel.go ccx/*_test.go
git commit -m "feat(ccx): route support params to the aggregate + overlay"
```

---

### Task S3: verification gate (no commit)

- [ ] **Step 1:** `go test ./...` + `go test -race ./ccx/...` — all green.
- [ ] **Step 2:** `golangci-lint run ./ccx/...` clean; `gofmt -l ccx/` empty.
- [ ] **Step 3:** Migration proof:
  - `grep -nE 'e\.extras\.(SupportType|SpringStiffMM)' ccx/panel.go` → **empty**.
  - `grep -n 'applySupportEdit' ccx/` → **empty** (function deleted, no dangling caller).
  - `overlaySupport` sets both fields in `project.go`; `projectAnalysis` calls it.
- [ ] **Step 4:** No commit (verification only).
