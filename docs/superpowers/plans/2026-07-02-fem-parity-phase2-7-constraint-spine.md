<!-- SPDX-License-Identifier: GPL-2.0-only -->
# FEM Parity — Phase 2.7 (ConstraintObject spine) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Move the **explicit constraint list** off the `extras` projection side-channel into a pure `femmodel.ConstraintObject` collection. The user's added constraints become first-class aggregate nodes; `ccx` gains the neutral↔spec mapper. The crux of the remainder (ADR-1).

**Architecture:** Per the architect brief (ADR-1): `femmodel.ConstraintObject` holds **intent + params** with a **neutral `Kind string`** (never the ccx `ConstraintKind` enum — the `MaterialModel`-string precedent); `ccx` keeps `ConstraintKind`/`ConstraintSpec` and owns the one place neutral→typed happens (`ccx/constraintmap.go`: `objectForKind` builds an object from settings, `constraintSpecFor` maps it back to a `ConstraintSpec`). The explicit list only ever holds the **8 face-based builder kinds** (Fixed/Roller/Symmetry/ElasticSupport/Force/Pressure/Hydrostatic/Displacement); whole-body loads (gravity/centrifugal/thermal) come only from the implicit convention (`defaultConstraints`), which stays in ccx (ADR-3) — NOT migrated here. `addConstraintFromSelection`→`Analysis.AddConstraint`; `clearConstraints`→`Analysis.ClearConstraints`; `projectAnalysis` sets `s.Constraints = mapConstraints(a.Constraints())`; the tree reads `a.Constraints()`. The **safety net** is a round-trip test: for every builder kind, `constraintSpecFor(objectForKind(kind, faces, s))` must equal `newConstraintSpec(kind, name, faces, s)`.

**Tech Stack:** Go; `oblikovati.org/calculix`; pure `ccx/femmodel`; links only `oblikovati.org/api`.

## Global Constraints

- Every new `.go` file carries `// SPDX-License-Identifier: GPL-2.0-only`.
- `ccx/femmodel` stays PURE (stdlib only) — `ConstraintObject.Kind` is a `string`, NOT `ccx.ConstraintKind`. `ConstraintKind`, all `*Spec` types, `newConstraintSpec`, `defaultConstraints` stay in `ccx`.
- Style: functions 4–20 lines, files <500 lines, explicit types, early returns.
- **Behavior-preserving:** `TestProjectDefaultAnalysisEqualsDefaultSettings` (empty default constraints → `nil` specs) + the full suite stay green. The **round-trip test** (C2) is the per-kind behavioral guard.
- The deck/solve pipeline (`buildModel`, `resolveSpecs`, the `len(specs)==0 → defaultConstraints` fallback) is UNCHANGED — it still consumes `[]ConstraintSpec`.
- Run `go test ./...` + `golangci-lint run ./ccx/...` + `gofmt -l` before each commit.
- Branch: `feature/fem-parity-phase2-7-constraint-spine` (already off the merged `main`).

## Anchors (verified)

- `ccx.ConstraintKind` (string enum) + `builderKinds() []ConstraintKind` = `{KindFixed, KindRoller, KindSymmetry, KindElasticSupport, KindForce, KindPressure, KindHydrostatic, KindDisplacement}`. `builderKindOrDefault(k)` maps `""→KindFixed`.
- `newConstraintSpec(kind ConstraintKind, name string, faces []string, s StudySettings) ConstraintSpec` (constraintbuilder.go:24) — the per-kind mapping:
  - Roller→`RollerSpec{Name,Faces}`; Symmetry→`SymmetrySpec{Name,Faces}`; Fixed(default)→`FixedSpec{Name,Faces}`.
  - ElasticSupport→`ElasticSupportSpec{Name,Faces,StiffnessTotal:s.SpringStiffMM}`.
  - Force→`ForceSpec{Name,Faces,Dir:[3]float64{0,0,-1},TotalN:s.LoadN}`.
  - Pressure→`PressureSpec{Name,Faces,MPa:s.PressureMPa}`.
  - Hydrostatic→`HydrostaticSpec{Name,Faces,GradientMPa:s.HydroGradientMPaMM,SurfaceZ:s.HydroSurfaceZ}`.
  - Displacement→`DisplacementSpec{Name,Faces,DOF:3,Value:s.DisplacementMM}`.
- Spec structs (all in ccx): `FixedSpec{Name,Faces}`, `RollerSpec{Name,Faces}`, `SymmetrySpec{Name,Faces}`, `ElasticSupportSpec{Name,Faces,StiffnessTotal}`, `ForceSpec{Name,Faces,Dir,TotalN}`, `PressureSpec{Name,Faces,MPa}`, `HydrostaticSpec{Name,Faces,GradientMPa,SurfaceZ}`, `DisplacementSpec{Name,Faces,DOF,Value}`.
- `addConstraintFromSelection` (constraintbuilder.go:51): `name := fmt.Sprintf("C%d", len(e.extras.Constraints)); spec := newConstraintSpec(e.extras.BuilderKind, name, faces, e.extras); e.extras.Constraints = append(...)`. `clearConstraints`: `e.extras.Constraints = nil`.
- `constraint_type` panel control (panel.go:286): `e.extras.BuilderKind = ConstraintKind(strings.TrimSpace(value))`; display reads `builderKindOrDefault(s.BuilderKind)`.
- `analysis_tree.go`: `ShowAnalysisTree` calls `analysisNodes(e.analysis, e.extras.Constraints)`; `analysisNodes(a, cons []ConstraintSpec)`; `constraintsNode(cons)` builds `con:N` nodes labeled `"Constraint N"` (reads only count/index).
- `project.go`: `s := extras; …; return s, s.Constraints`. `study.go buildModel`: `specs := settings.Constraints; if len(specs)==0 { specs = defaultConstraints(settings, faces) }; resolveSpecs(...)`.
- `femmodel.Analysis` mutator pattern: `AddResult` = `a.nextResult++; r := newResultObject("result"+strconv.Itoa(a.nextResult), …); a.results = append(…); return r`.
- `constraintbuilder_test.go` currently asserts `e.extras.BuilderKind = …`, `len(e.extras.Constraints)`, `e.extras.Constraints[0].(RollerSpec)`, `.Kind()` — these MUST be updated to the aggregate in C3.

---

### Task C1: `femmodel.ConstraintObject` + `Analysis` constraint collection

**Files:**
- Create: `ccx/femmodel/constraint_object.go`
- Modify: `ccx/femmodel/analysis.go` (add `constraints []ConstraintObject` + `nextConstraint int` + mutators/accessor)
- Test: `ccx/femmodel/constraint_object_test.go`

**Interfaces:**
- Produces: `femmodel.ConstraintObject{ Kind string; Faces []string; StiffnessTotal, TotalN float64; Dir [3]float64; MPa, GradientMPa, SurfaceZ float64; DOF int; Value float64 }` with unexported `id,name`, methods `ObjectID()/Category()/Name()` (`Category()==CategoryConstraint`); `(*Analysis).Constraints() []ConstraintObject`; `(*Analysis).AddConstraint(name string, o ConstraintObject) ConstraintObject` (assigns `con:N`-style id + name, appends, returns); `(*Analysis).ClearConstraints()`.

- [ ] **Step 1: Write the failing test**

Create `ccx/femmodel/constraint_object_test.go`:
```go
// SPDX-License-Identifier: GPL-2.0-only

package femmodel

import "testing"

func TestAddConstraintAssignsIDAndName(t *testing.T) {
	a := NewDefaultAnalysis()
	if len(a.Constraints()) != 0 {
		t.Fatalf("default analysis should have no constraints, got %d", len(a.Constraints()))
	}
	c1 := a.AddConstraint("C0", ConstraintObject{Kind: "force", Faces: []string{"face/a"}, TotalN: 100})
	c2 := a.AddConstraint("C1", ConstraintObject{Kind: "fixed", Faces: []string{"face/b"}})
	if c1.ObjectID() == c2.ObjectID() {
		t.Fatalf("constraint ids collide: %q", c1.ObjectID())
	}
	if c1.Name() != "C0" || c1.Kind != "force" || c1.TotalN != 100 || c1.Category() != CategoryConstraint {
		t.Fatalf("constraint 1 wrong: %+v", c1)
	}
	if got := a.Constraints(); len(got) != 2 || got[1].Kind != "fixed" {
		t.Fatalf("Constraints() = %+v, want two ending fixed", got)
	}
}

func TestClearConstraints(t *testing.T) {
	a := NewDefaultAnalysis()
	a.AddConstraint("C0", ConstraintObject{Kind: "fixed"})
	a.ClearConstraints()
	if len(a.Constraints()) != 0 {
		t.Fatalf("ClearConstraints left %d", len(a.Constraints()))
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /home/vmiguel/git/oblikovati-workspace/Oblikovati.AddIns.CalculiX && go test ./ccx/femmodel/ -run 'TestAddConstraint|TestClearConstraints'`
Expected: FAIL — `undefined: ConstraintObject` / `AddConstraint`.

- [ ] **Step 3: Write minimal implementation**

Create `ccx/femmodel/constraint_object.go`:
```go
// SPDX-License-Identifier: GPL-2.0-only

package femmodel

// ConstraintObject is one study constraint as pure intent: a neutral Kind tag, the host face
// reference keys it binds to, and the typed parameters of the 8 face-based builder kinds. Only the
// fields relevant to Kind are meaningful (a discriminated union, like MaterialObject). ccx maps it
// to a ConstraintSpec (which does the mesh binding) — femmodel never learns the CalculiX taxonomy.
type ConstraintObject struct {
	id, name       string
	Kind           string     // ConstraintKind underlying string: "fixed","roller","force",…
	Faces          []string   // host face reference keys
	StiffnessTotal float64    // elastic support (N/mm)
	TotalN         float64    // force
	Dir            [3]float64 // force direction
	MPa            float64    // pressure
	GradientMPa    float64    // hydrostatic γ
	SurfaceZ       float64    // hydrostatic free-surface height
	DOF            int        // displacement DOF
	Value          float64    // displacement magnitude
}

func (o ConstraintObject) ObjectID() string  { return o.id }
func (o ConstraintObject) Category() Category { return CategoryConstraint }
func (o ConstraintObject) Name() string       { return o.name }
```
In `ccx/femmodel/analysis.go`, add the fields to the `Analysis` struct (after `results`):
```go
	constraints    []ConstraintObject
	nextConstraint int
```
And the mutators/accessor (mirror `AddResult`):
```go
// Constraints returns the explicit constraint list in creation order.
func (a *Analysis) Constraints() []ConstraintObject { return a.constraints }

// AddConstraint appends a constraint with a fresh unique id and the given name, returning it.
func (a *Analysis) AddConstraint(name string, o ConstraintObject) ConstraintObject {
	a.nextConstraint++
	o.id = "con" + strconv.Itoa(a.nextConstraint)
	o.name = name
	a.constraints = append(a.constraints, o)
	return o
}

// ClearConstraints empties the explicit constraint list.
func (a *Analysis) ClearConstraints() { a.constraints = nil }
```
(`strconv` is already imported by `analysis.go`.)

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./ccx/femmodel/ -run 'TestAddConstraint|TestClearConstraints' -v` → PASS. Then `go test ./ccx/femmodel/` → PASS.

- [ ] **Step 5: Commit**
```bash
git add ccx/femmodel/constraint_object.go ccx/femmodel/analysis.go ccx/femmodel/constraint_object_test.go
git commit -m "feat(femmodel): ConstraintObject + Analysis constraint collection (Add/Clear/Constraints)"
```

---

### Task C2: `ccx/constraintmap.go` — the neutral↔spec mapper + round-trip guard

**Files:**
- Create: `ccx/constraintmap.go`
- Test: `ccx/constraintmap_test.go`

**Interfaces:**
- Consumes: `femmodel.ConstraintObject`, `ConstraintKind`/`builderKinds`/`builderKindOrDefault`, all `*Spec` types, `StudySettings`, `newConstraintSpec`.
- Produces: `objectForKind(kind ConstraintKind, faces []string, s StudySettings) femmodel.ConstraintObject` (extracts the per-kind params from settings, mirroring `newConstraintSpec`); `constraintSpecFor(o femmodel.ConstraintObject) ConstraintSpec` (builds the `*Spec` from the object); `mapConstraints(objs []femmodel.ConstraintObject) []ConstraintSpec`.

- [ ] **Step 1: Write the failing test (the safety net)**

Create `ccx/constraintmap_test.go`:
```go
// SPDX-License-Identifier: GPL-2.0-only

package ccx

import (
	"reflect"
	"testing"
)

// The migration must be behavior-identical: for every builder kind, mapping settings → object →
// spec must reproduce EXACTLY what newConstraintSpec produced. This is the guard for slice 2.7.
func TestConstraintRoundTripMatchesNewConstraintSpec(t *testing.T) {
	s := defaultSettings()
	faces := []string{"face/a", "face/b"}
	for _, k := range builderKinds() {
		a := femmodelNewDefaultForTest() // NewDefaultAnalysis via a tiny local helper (see note)
		obj := a.AddConstraint("C0", objectForKind(k, faces, s))
		got := constraintSpecFor(obj)
		want := newConstraintSpec(k, "C0", faces, s)
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("kind %q: constraintSpecFor(objectForKind) = %#v, want newConstraintSpec = %#v", k, got, want)
		}
	}
}

func TestMapConstraintsPreservesOrderAndKind(t *testing.T) {
	s := defaultSettings()
	a := femmodelNewDefaultForTest()
	a.AddConstraint("C0", objectForKind(KindForce, []string{"face/a"}, s))
	a.AddConstraint("C1", objectForKind(KindFixed, []string{"face/b"}, s))
	specs := mapConstraints(a.Constraints())
	if len(specs) != 2 || specs[0].Kind() != KindForce || specs[1].Kind() != KindFixed {
		t.Fatalf("mapConstraints = %+v, want [force fixed]", specs)
	}
}
```
NOTE: `femmodelNewDefaultForTest()` — add a one-line test helper `func femmodelNewDefaultForTest() *femmodel.Analysis { return femmodel.NewDefaultAnalysis() }` in the test file (import `oblikovati.org/calculix/ccx/femmodel`), or inline `femmodel.NewDefaultAnalysis()` directly. The point is to build the object *through* `AddConstraint` so it carries the name `"C0"` that `constraintSpecFor` reads.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./ccx/ -run 'TestConstraintRoundTrip|TestMapConstraints'`
Expected: FAIL — `undefined: objectForKind` / `constraintSpecFor`.

- [ ] **Step 3: Write minimal implementation**

Create `ccx/constraintmap.go`:
```go
// SPDX-License-Identifier: GPL-2.0-only

package ccx

import "oblikovati.org/calculix/ccx/femmodel"

// objectForKind builds a pure ConstraintObject for a builder kind, extracting the per-kind params
// from the (projected) settings — the exact inverse of newConstraintSpec's read of StudySettings.
func objectForKind(kind ConstraintKind, faces []string, s StudySettings) femmodel.ConstraintObject {
	o := femmodel.ConstraintObject{Kind: string(builderKindOrDefault(kind)), Faces: faces}
	switch builderKindOrDefault(kind) {
	case KindElasticSupport:
		o.StiffnessTotal = s.SpringStiffMM
	case KindForce:
		o.TotalN, o.Dir = s.LoadN, [3]float64{0, 0, -1}
	case KindPressure:
		o.MPa = s.PressureMPa
	case KindHydrostatic:
		o.GradientMPa, o.SurfaceZ = s.HydroGradientMPaMM, s.HydroSurfaceZ
	case KindDisplacement:
		o.DOF, o.Value = 3, s.DisplacementMM
	}
	return o
}

// constraintSpecFor maps a pure ConstraintObject back to the ccx ConstraintSpec that binds faces
// and writes the AnalysisModel — the one place neutral Kind → typed spec happens (ADR-1).
func constraintSpecFor(o femmodel.ConstraintObject) ConstraintSpec {
	name, faces := o.Name(), o.Faces
	switch ConstraintKind(o.Kind) {
	case KindRoller:
		return RollerSpec{Name: name, Faces: faces}
	case KindSymmetry:
		return SymmetrySpec{Name: name, Faces: faces}
	case KindElasticSupport:
		return ElasticSupportSpec{Name: name, Faces: faces, StiffnessTotal: o.StiffnessTotal}
	case KindForce:
		return ForceSpec{Name: name, Faces: faces, Dir: o.Dir, TotalN: o.TotalN}
	case KindPressure:
		return PressureSpec{Name: name, Faces: faces, MPa: o.MPa}
	case KindHydrostatic:
		return HydrostaticSpec{Name: name, Faces: faces, GradientMPa: o.GradientMPa, SurfaceZ: o.SurfaceZ}
	case KindDisplacement:
		return DisplacementSpec{Name: name, Faces: faces, DOF: o.DOF, Value: o.Value}
	default:
		return FixedSpec{Name: name, Faces: faces}
	}
}

// mapConstraints projects the aggregate's constraint objects to solver-pipeline specs, in order.
func mapConstraints(objs []femmodel.ConstraintObject) []ConstraintSpec {
	if len(objs) == 0 {
		return nil
	}
	specs := make([]ConstraintSpec, len(objs))
	for i, o := range objs {
		specs[i] = constraintSpecFor(o)
	}
	return specs
}
```
(If `constraintSpecFor`/`objectForKind` push a func over 20 lines, they are switch dispatchers — acceptable; funlen counts statements (≤20) and cases are cheap. Confirm lint.)

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./ccx/ -run 'TestConstraintRoundTrip|TestMapConstraints' -v` → PASS (the round-trip proves per-kind equivalence). 

- [ ] **Step 5: Commit**
```bash
git add ccx/constraintmap.go ccx/constraintmap_test.go
git commit -m "feat(ccx): constraint neutral<->spec mapper (objectForKind/constraintSpecFor) + round-trip guard"
```

---

### Task C3: wire the engine — constraints flow through the aggregate

**Files:**
- Modify: `ccx/engine.go` (add `builderKind ConstraintKind` engine field)
- Modify: `ccx/constraintbuilder.go` (`addConstraintFromSelection`/`clearConstraints` use the aggregate)
- Modify: `ccx/panel.go` (`constraint_type` control → `e.builderKind`; display reads `e.builderKind`)
- Modify: `ccx/project.go` (`s.Constraints = mapConstraints(a.Constraints())`)
- Modify: `ccx/analysis_tree.go` (`analysisNodes(a)` reads `a.Constraints()`; drop the `cons` param)
- Modify: `ccx/constraintbuilder_test.go` (assert on the aggregate, not `e.extras.Constraints`)

**Interfaces:**
- Consumes: C1 mutators, C2 `objectForKind`/`mapConstraints`, `e.study()`.
- Produces: added constraints live on `e.analysis`; `study()`/projection source constraints from the aggregate; the tree reads `a.Constraints()`; `extras.Constraints`/`extras.BuilderKind` no longer written.

- [ ] **Step 1: Write the failing test**

Update `ccx/constraintbuilder_test.go` to assert on the aggregate. Replace the `e.extras.BuilderKind =` writes with `e.builderKind =`, and the `e.extras.Constraints` assertions with `e.analysis.Constraints()` checks on the `ConstraintObject` (its `Kind`), e.g.:
```go
	e.builderKind = KindRoller
	e.addConstraintFromSelection() // (as the existing test drives it)
	cons := e.analysis.Constraints()
	if len(cons) != 1 || cons[0].Kind != string(KindRoller) || len(cons[0].Faces) != 2 || cons[0].Name() != "C0" {
		t.Fatalf("expected 1 roller C0 over 2 faces, got %+v", cons)
	}
	e.builderKind = KindPressure
	e.addConstraintFromSelection()
	if cons = e.analysis.Constraints(); len(cons) != 2 || cons[1].Kind != string(KindPressure) {
		t.Fatalf("second add should be a pressure constraint, got %+v", cons)
	}
	e.clearConstraints()
	if len(e.analysis.Constraints()) != 0 {
		t.Fatalf("clear should empty the aggregate, got %d", len(e.analysis.Constraints()))
	}
```
(Keep the existing test's host/selection fake + call structure — only change the field it reads/writes. Read `constraintbuilder_test.go` first and mirror its `addConstraintFromSelection` invocation exactly.)

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./ccx/ -run 'Constraint'`
Expected: FAIL to compile / assert — `e.builderKind` undefined, constraints still in extras.

- [ ] **Step 3: Write minimal implementation**

In `ccx/engine.go`, add to the `Engine` struct (near `analysis`/`extras`, under `e.mu`): `builderKind ConstraintKind // the constraint type the panel builder adds next`.
In `ccx/constraintbuilder.go` `addConstraintFromSelection`, replace the build+append with the aggregate path (source params from the PROJECTED settings so future load/support migrations can't break the builder):
```go
	settings, _ := e.study() // projected params (locks internally)
	e.mu.Lock()
	name := fmt.Sprintf("C%d", len(e.analysis.Constraints()))
	obj := e.analysis.AddConstraint(name, objectForKind(e.builderKind, faces, settings))
	count := len(e.analysis.Constraints())
	kind := ConstraintKind(obj.Kind)
	e.mu.Unlock()
	_, _ = e.ShowPanel()
	e.reportStatus(fmt.Sprintf(/* keep the existing message, using builderKindOrDefault(kind), len(faces), count */))
```
(Preserve the existing status message wording — it used `builderKindOrDefault(spec.Kind())`; use `builderKindOrDefault(kind)`.) In `clearConstraints`, replace `e.extras.Constraints = nil` with `e.analysis.ClearConstraints()` (under the lock).
In `ccx/panel.go`, change the `constraint_type` case (`e.extras.BuilderKind = …`) to `e.builderKind = ConstraintKind(strings.TrimSpace(value))`, and the panel display (`builderKindOrDefault(s.BuilderKind)`) to read the engine's `e.builderKind` — NOTE `panelControls(s)` is a pure function of `s StudySettings`; the builder kind is engine state, so either (a) add `s.BuilderKind` to the projection (overlay `s.BuilderKind = e...`? no — projection has no engine ref) OR (b) pass `e.builderKind` into `ShowPanel`'s control build. SIMPLEST: keep `StudySettings.BuilderKind` as the panel's display source but write it from `e.builderKind` at `ShowPanel` time: in `ShowPanel`, after `s, _ := e.study()`, set `s.BuilderKind = e.builderKind` before `panelControls(s)`. Then the `constraint_type` edit writes `e.builderKind` (and the next `ShowPanel` reflects it). This keeps `panelControls` unchanged.
In `ccx/project.go`, before `return`, set `s.Constraints = mapConstraints(a.Constraints())` (replaces the passed-through `extras.Constraints`).
In `ccx/analysis_tree.go`, change `analysisNodes(a *femmodel.Analysis)` (drop `cons`), build `constraintsNode` from `a.Constraints()` (label stays `"Constraint N"` by index — read only `len`); update `ShowAnalysisTree` to call `analysisNodes(e.analysis)` (no `e.extras.Constraints`).

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./ccx/ -run 'Constraint' -v` → PASS. Then `go test ./...` → all green (incl. the equivalence guard: default analysis has no constraints → `mapConstraints` nil → `s.Constraints` nil == `defaultSettings().Constraints`). Verify: `grep -nE 'e\.extras\.(Constraints|BuilderKind)' ccx/*.go` → empty; `go vet ./ccx/...` clean. `golangci-lint run ./ccx/...` clean.

- [ ] **Step 5: Commit**
```bash
git add ccx/engine.go ccx/constraintbuilder.go ccx/panel.go ccx/project.go ccx/analysis_tree.go ccx/constraintbuilder_test.go
git commit -m "feat(ccx): route the explicit constraint list through the femmodel aggregate"
```

---

### Task C4: verification gate

- [ ] **Step 1:** `go test ./...` — all green (incl. round-trip + equivalence guards).
- [ ] **Step 2:** `golangci-lint run ./ccx/...` clean; `gofmt -l ccx/` empty.
- [ ] **Step 3:** Migration proof: `grep -nE 'e\.extras\.(Constraints|BuilderKind)' ccx/*.go` (excluding tests) → **empty**; `grep -n 'mapConstraints(a.Constraints())' ccx/project.go` → present; `grep -n 'analysisNodes(e.analysis)' ccx/analysis_tree.go` → present (no `cons` threaded).
- [ ] **Step 4:** No commit (verification). Fix gaps via a focused TDD cycle. NOTE: `StudySettings.Constraints`/`BuilderKind` fields REMAIN (the pipeline read-model + the projected display) — that is correct; only their SOURCE OF TRUTH moved. They're deleted in the 2.12 cleanup.

---

## Self-Review (completed by plan author)

- **Spec coverage:** implements ADR-1 (the constraint spine) — `femmodel.ConstraintObject` with neutral `Kind` (C1); `ccx/constraintmap.go` `objectForKind`/`constraintSpecFor`/`mapConstraints` with the per-kind round-trip guard (C2); the engine sources constraints from the aggregate, tree reads it, projection maps it (C3). Whole-body loads + the implicit convention stay in ccx (ADR-3) — out of scope, unchanged. Non-modal (ADR-3): the `constraint_type` selector + Add/Clear stay panel/command-driven.
- **Placeholder scan:** the "keep the existing status message / mirror the test's invocation" directives are explicit locate-and-preserve instructions against named functions; the `objectForKind`/`constraintSpecFor`/mutators are fully coded. The round-trip test is the concrete behavioral spec.
- **Type consistency:** `ConstraintObject.Kind` is a `string`, `ConstraintKind(o.Kind)` cast in `constraintSpecFor` (mirrors `MaterialModel`); `objectForKind`↔`constraintSpecFor` are exact inverses per kind (guarded by the round-trip vs `newConstraintSpec`); `AddConstraint(name, o)` assigns id+name.
- **Equivalence + safety:** default analysis has zero constraints → `nil` specs → equivalence green; the round-trip test pins per-kind behavioral identity to `newConstraintSpec`, so no solve input drifts.

## Next slices
- **2.8/2.9** — default-load / default-support param groups → `Analysis.defaultLoad`/`defaultSupport` templates (the convention's numbers). **2.10/2.11** — thermal-BC / field-drive param groups → new BC objects. **2.12** — drop the `extras` arg from `projectAnalysis`, delete the dead `StudySettings` fields, retire `panel.go`.
