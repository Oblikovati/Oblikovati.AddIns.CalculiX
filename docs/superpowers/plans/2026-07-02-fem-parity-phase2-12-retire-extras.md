<!-- SPDX-License-Identifier: GPL-2.0-only -->
# FEM Parity Phase 2.12 — retire the `extras` base

> **For agentic workers:** REQUIRED SUB-SKILL: superpowers:subagent-driven-development.
> Steps use checkbox (`- [ ]`) syntax.

**Goal:** Retire the flat `StudySettings` `extras` base now that every field is overlaid from the
`femmodel.Analysis` aggregate. Drop the `extras` parameter from `projectAnalysis` (build from a
zero-value `StudySettings` instead), delete the `Engine.extras` field and its `defaultSettings()`
seed, and re-point the three equivalence guards at the new no-arg projection.

**Architecture:** This is the terminal slice of the strangler migration. Since Phase 2.11 the
overlay set is exhaustive over `StudySettings`: every field is written by an `overlay*` group or
by the primary-result / constraints steps, EXCEPT `BuilderKind` — which is `""` in
`defaultSettings()` and is applied separately by `ShowPanel` from `Engine.builderKind`, never by
`projectAnalysis`. Therefore replacing `s := extras` with `s := StudySettings{}` produces
byte-identical output for ALL inputs, not just the default — the `extras` base contributed nothing
after 2.11.

**What this slice does NOT do:** it does not delete `panel.go` (that file holds the live task-panel
machinery — `ShowPanel`, `applyPanelEdit`, and the `applyAgg*` routing helpers), does not delete
`StudySettings` (it remains the mesh/deck/solve/render pipeline DTO), and does not delete
`defaultSettings()` (it remains the golden reference the equivalence guards compare against). The
loose roadmap phrase "retire panel.go" means retiring the `extras` mechanism, which is what this
plan does.

**Tech Stack:** Go; `ccx/femmodel` untouched; `ccx` links only `oblikovati.org/api`.

## Global Constraints

- Behavior must be preserved: `projectAnalysis(NewDefaultAnalysis())` MUST still equal
  `defaultSettings()` field-for-field (the rewritten equivalence guard). `study()` on a fresh
  engine MUST still equal `defaultSettings()`.
- `StudySettings`, `defaultSettings()`, `panel.go`, and the `BuilderKind` field all STAY.
- Style: golangci funlen 30 lines / 20 statements; no `any`; explicit types; functions ≤20 lines;
  SPDX headers unchanged.
- After this slice: `grep -rn 'e\.extras\|extras StudySettings\|extras:' ccx/*.go` (excluding the
  plan docs) returns NOTHING — the `extras` identifier is gone from the package.

---

### Task R1: drop the `extras` base from `projectAnalysis` and delete `Engine.extras`

**Files:**
- Modify: `ccx/project.go` (`projectAnalysis` signature + first line; docstring)
- Modify: `ccx/engine.go` (delete `extras` field; drop it from the `NewEngine` literal; update the
  mutex comment; `study()` call site)
- Modify: `ccx/constraintbuilder.go` (`addConstraintFromSelection` call site)
- Modify: `ccx/project_test.go` (2 `projectAnalysis(..., defaultSettings())` call sites)
- Test: `ccx/project_test.go`, `ccx/engine_study_test.go` (guards; the latter is call-site-stable)

**Interfaces:**
- Changes: `projectAnalysis(a *femmodel.Analysis, extras StudySettings)` →
  `projectAnalysis(a *femmodel.Analysis) (StudySettings, []ConstraintSpec)`.
- Consumes: `overlaySolver/overlayMesh/overlayMaterial/overlayLoad/overlaySupport/overlayThermal/overlayEM`
  (unchanged), `a.PrimaryResult()`, `mapConstraints(a.Constraints())`.

- [ ] **Step 1: Update the equivalence guard test FIRST (it will fail to compile → RED).**

In `ccx/project_test.go`, change the two call sites to the new signature:

- The default-equivalence test (currently `projectAnalysis(femmodel.NewDefaultAnalysis(), defaultSettings())`):

```go
	got, specs := projectAnalysis(femmodel.NewDefaultAnalysis())
	want := defaultSettings()
```

- The second test (currently `projectAnalysis(a, defaultSettings())`):

```go
	got, _ := projectAnalysis(a)
```

Leave every assertion below these lines unchanged — the whole point is that the projected value is
still `defaultSettings()`.

- [ ] **Step 2: Run to verify it fails (RED).**

Run: `go test ./ccx/ -run 'TestProject' -v`
Expected: COMPILE FAILURE — `projectAnalysis` still takes 2 args. This is the red state.

- [ ] **Step 3: Change `projectAnalysis` in `ccx/project.go`.**

Signature and first line:

```go
func projectAnalysis(a *femmodel.Analysis) (StudySettings, []ConstraintSpec) {
	s := StudySettings{}
	s = overlaySolver(a, s)
	// ... the rest of the overlay chain is UNCHANGED ...
```

Update the docstring to drop the `extras` argument reference:

```go
// projectAnalysis flattens the Analysis tree into a StudySettings: it starts from the zero value
// and overlays every field the tree owns — the aggregate is the sole source of truth. This single
// seam keeps the mesh/deck/solve/render pipeline reading a plain StudySettings while the edit model
// is a tree. projectAnalysis(NewDefaultAnalysis()) reproduces defaultSettings() exactly (the
// equivalence guard). Constraints are carried on StudySettings and returned alongside for callers
// that want them directly.
```

- [ ] **Step 4: Update the two runtime call sites.**

`ccx/engine.go` `study()`:

```go
func (e *Engine) study() (StudySettings, []ConstraintSpec) {
	e.mu.Lock()
	defer e.mu.Unlock()
	return projectAnalysis(e.analysis)
}
```

> Match the exact existing `study()` body — only the `projectAnalysis(e.analysis, e.extras)` call
> changes to `projectAnalysis(e.analysis)`.

`ccx/constraintbuilder.go` `addConstraintFromSelection` (the line inside the lock):

```go
	settings, _ := projectAnalysis(e.analysis)
```

- [ ] **Step 5: Delete the `Engine.extras` field and its seed.**

In `ccx/engine.go`:
- Remove the `extras StudySettings` field from the `Engine` struct.
- In the `NewEngine` constructor literal, remove `extras: defaultSettings()` (leave
  `analysis: femmodel.NewDefaultAnalysis()` and the rest).
- Update the mutex doc comment: change `// guards analysis, extras, builderKind and running` to
  `// guards analysis, builderKind and running`.

- [ ] **Step 6: Run to verify pass (GREEN).**

Run: `go test ./ccx/ -run 'TestProject' -v` → PASS (projection still equals defaults).
Then `go test ./ccx/...` → PASS (the whole suite, incl. `engine_study_test.go`'s
`study() == defaultSettings()` and the constraint/deck tests).

- [ ] **Step 7: Full local gate.**

Run: `go test ./...` && `go test -race ./ccx/...` && `golangci-lint run ./ccx/...` && `gofmt -l ccx/`
Expected: all green; lint 0 issues; gofmt empty (watch for `unused` on `defaultSettings` — it must
still be referenced by the tests; if lint flags it as unused, a test call site was missed).

- [ ] **Step 8: Commit.**

```bash
git add ccx/project.go ccx/engine.go ccx/constraintbuilder.go ccx/project_test.go
git commit -m "refactor(ccx): retire the extras base; projectAnalysis builds from the aggregate alone

Every StudySettings field is overlaid from the femmodel.Analysis aggregate as of 2.11, so the
flat extras base is dead weight. Drop the extras parameter (build from the zero value), delete the
Engine.extras field and its defaultSettings() seed, and re-point the equivalence guards at the
no-arg projection. BuilderKind — the only non-overlaid field — is empty in defaults and applied by
ShowPanel, so zero-base output is byte-identical to the old extras-base output for all inputs."
```

---

### Task R2: verification gate (no commit)

- [ ] **Step 1:** `go test ./...` + `go test -race ./ccx/...` — all green.
- [ ] **Step 2:** `golangci-lint run ./ccx/...` clean; `gofmt -l ccx/` empty.
- [ ] **Step 3:** Retirement proof:
  - `grep -rn 'e\.extras' ccx/*.go` → **empty**.
  - `grep -rn 'extras StudySettings\|extras:' ccx/*.go` → **empty**.
  - `grep -n 'func projectAnalysis' ccx/project.go` shows the single-arg signature.
  - `grep -c 'defaultSettings()' ccx/*_test.go` shows the golden reference is still used by the guards.
- [ ] **Step 4:** Sanity: `study()` on a fresh engine equals `defaultSettings()` (covered by
  `engine_study_test.go`); confirm that test still passes unmodified.
- [ ] **Step 5:** No commit (verification only).
