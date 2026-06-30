<!-- SPDX-License-Identifier: Apache-2.0 -->
# CalculiX add-in → FreeCAD-FEM UI/UX parity — design

Date: 2026-06-30
Status: Approved for planning
Primary repo: `Oblikovati.AddIns.CalculiX` (links only `oblikovati.org/api`, Apache-2.0)
Touches: `Oblikovati.API` (public contract), `Oblikovati` (GPL host: router + Vulkan head)

## 1. Goal and framing

Bring the CalculiX FEA add-in to UI/UX parity with FreeCAD's FEM workbench, the same
way the CAM add-in was brought to parity with FreeCAD's Path workbench.

The FEA **engine already exists** and is mature: static / frequency / buckling /
thermomechanical / heat-transfer / electromagnetic analyses; dozens of loads and
boundary conditions; isotropic / orthotropic / temperature-dependent / Neo-Hookean
materials; auto-detected tie + contact; gmsh volume meshing (C3D4/C3D10); ccx solve;
result rendering. The add-in already has CAM's architecture: a cgo-free `ccx/` bridge,
a `HostCaller` interface, an `Engine`, and 110 unit tests.

This effort is therefore **UI/UX restructuring** plus a **new post-processing math
layer** — not new solver capability. The existing solve pipeline
(mesh → deck → ccx → `.frd`/`.dat` → render) does not move.

The current UI is a single 9-section dockable panel (`ccx/panel.go`) plus three
commands. The target is FreeCAD's model: an Analysis-container browser tree, a
categorized ribbon, per-object modal Task panels, and a results post-processing
pipeline.

### Scope decisions (locked with the user)

- **Full parity, including post-processing** (warp / clip / cut / contour / glyph /
  probe / linearized stress / calculator).
- **Full public-API extension** is acceptable to get a faithful editing UX: add a
  geometry **reference-list** control and a **modal Task-panel** spec, plus the
  rendering primitives the result viz needs.
- **Faithful modal Task panels** (FreeCAD OK/Cancel semantics), not a single
  swap-content dockable panel.

Delivery is **ordered, independently-shippable phases** (Section 6).

## 2. Architecture overview

Three pillars:

- **Pillar A — Public API extensions** (`Oblikovati.API`, then GPL host). Five additive
  changes, each semver-minor, each degrading gracefully on old hosts.
- **Pillar B — Add-in restructure** behind a *projection seam*: a pure `ccx/femmodel`
  Analysis aggregate becomes the source of truth; the flat `StudySettings` is demoted
  to a solve-time projection, so the engine/pipeline is untouched.
- **Pillar C — Post-processing math layer**: two pure cores (marching-tetrahedra;
  point-location + isoparametric interpolation) power all ten filters.

Dependency directions (arrows point inward, toward the pure domain):

```
cgo shell (export.go/hostcaller.go) ─► ccx (engine + UI + filters) ─► ccx/femmodel (pure)
                                              │
                                              └─► oblikovati.org/api/client (Apache-2.0)
```

`Oblikovati.API` never imports the GPL host. `ccx/femmodel` imports neither `api` nor
the host. Only `ccx/project.go` bridges the aggregate to the engine's `StudySettings`.

## 3. Pillar A — Public Window-Layout & graphics API extensions

All additions follow ADR-0018 ordering: `api/types` (enums/value structs, defined
once) → `api/wire` (JSON DTOs + method-name/event-name constants) → `api/client`
(typed helpers with `// mcp:tool` / `// mcp:summary` annotations) → GPL host
`addin/router` + `head/ui` rendering. Every new exported `.go` carries an SPDX header.
This extends ADR-0018 (two-part surface) and ADR-0019 (declarative panel containers);
a new ADR pair is written under `Oblikovati/architecture`.

### A1 — `referenceList` control kind

FreeCAD constraint panels center on a list of picked geometry references with
Add-from-selection and per-row Remove. The add-in README calls "no list widget" the
key host-API gap.

- `api/types`: new `PanelReferenceList PanelControlKind = 12` (`"referenceList"`),
  added to `panelControlKindNames`. Unknown-kind-degrades-to-`Text` rule preserved, so
  old hosts render the caption.
- `api/wire/docking.go`: extend `PanelControlSpec` with `omitempty` fields
  `Rows []PanelReferenceRow` and `Accepts []string` (allowed kinds:
  `"face"`/`"edge"`/`"vertex"`; empty = any). A leaf control still marshals exactly as
  before. New value struct:
  ```go
  type PanelReferenceRow struct {
      Ref   string `json:"ref"`             // host selection reference, e.g. "face/<url-b64>"
      Label string `json:"label,omitempty"` // host derives one (e.g. "Face3") if empty
  }
  ```
- New **event** (not an overload of the scalar `PanelValueChangedEvent`): refs are the
  **full new row set** (bulk-state, matching the rest of the panel model):
  ```go
  type PanelReferencesChangedEvent struct {
      Type      string   `json:"type"` // EventPanelReferencesChanged
      WindowId  string   `json:"windowId"`
      ControlId string   `json:"controlId"`
      Refs      []string `json:"refs"`
      Action    string   `json:"action,omitempty"` // "add" | "remove" (diagnostics only)
  }
  ```
- Programmatic driver (mirrors `dockableWindows.setValue` for MCP/tests):
  `dockableWindows.setReferences` with `SetDockableWindowReferencesArgs{WindowId,
  ControlId, Refs}`.
- `api/wire/methods.go`: `MethodDockableWindowsSetReferences =
  "dockableWindows.setReferences"`, `EventPanelReferencesChanged =
  "panel.referencesChanged"`.
- `api/client`: `PanelReferenceList(id, text string, accepts []string, rows
  []wire.PanelReferenceRow) wire.PanelControlSpec` and
  `DockableWindows.SetReferences(windowID, controlID string, refs []string)` with mcp
  annotations (`set_panel_references`).
- **Host behavior**: the control renders rows plus a built-in **[Add]** and per-row
  **[×]**. *Add* reads the live viewport selection, keeps refs whose kind ∈ `Accepts`,
  appends, and emits `PanelReferencesChangedEvent{Action:"add", Refs: <full set>}`.
  *[×]* drops the row and emits `Action:"remove"`. Clicking a row highlights that ref
  in the viewport (host-internal; no event). The add-in no longer hand-rolls
  `Model().Selection()` (back-compat: it still may).

### A2 — Modal `TaskPanelSpec`

A new surface (not a `Modal` flag on `DockableWindowSpec`, which would muddy the
non-modal docking invariant; not `WebDialogSpec`, which is HTML and can't embed
`referenceList`). It reuses `PanelControlSpec`, so the reference-list composes inside
it for free.

```go
// api/wire/dialogs.go
type TaskPanelSpec struct {
    ID          string             `json:"id"`
    Title       string             `json:"title"`
    Controls    []PanelControlSpec `json:"controls,omitempty"`
    OKLabel     string             `json:"okLabel,omitempty"`     // default "OK"
    CancelLabel string             `json:"cancelLabel,omitempty"` // default "Cancel"
}
type ShowTaskPanelArgs  struct{ Panel TaskPanelSpec `json:"panel"` }
type CloseTaskPanelArgs struct{ ID string `json:"id"` }
type TaskPanelClosedEvent struct {
    Type     string `json:"type"`     // EventTaskPanelClosed
    ID       string `json:"id"`
    Accepted bool   `json:"accepted"` // true = OK, false = Cancel/closed
}
```

- `methods.go`: `MethodTaskPanelShow = "taskPanel.show"`,
  `MethodTaskPanelClose = "taskPanel.close"`, `EventTaskPanelClosed =
  "taskPanel.closed"`.
- `api/client`: `Client.TaskPanels()` group with `Show(p) (OKResult, error)` and
  `Close(id) (OKResult, error)` (mcp: `task_panel_show`, `task_panel_close`).
- **Async** — `Show` never blocks the session goroutine; the result arrives only via
  `TaskPanelClosedEvent`. While open, control edits stream via the existing
  value/references events keyed on the panel id. On `Accepted=true` the add-in keeps
  the staged edits; on `false` it discards them (no values echoed in the close event —
  they already arrived incrementally).
- **Old-host fallback**: `taskPanel.show` is a new method → an old host returns
  method-not-found; the add-in feature-detects once and falls back to a non-modal
  dockable window with explicit OK/Cancel `PanelButton`s.

### A3 — Screen-space HUD lane (legend + plot)

Every graphics primitive today anchors in world space, so a color legend would swim
under orbit. Add a screen-anchored output lane:

- General form: a `GraphicsSpace` (`"world"`|`"screen"`) on the primitive (or a
  `GraphicsLaneHUD`), with `Anchor`/coords in NDC `0..1` or pixels. Lets the add-in
  draw the colorbar, axis triad, and (optionally) the data-along-line XY plot as
  lines+text+triangles in screen space.
- Recommended pairing: a purpose-built **`legend`** primitive rendered host-side from
  `{mapperName, min, max, title, bandCount, corner}` so tick typography is host-owned
  and stays synced to the registered mapper.
- The data-along-line **XY plot** is rendered in the add-in's **task panel** (where
  FreeCAD shows its matplotlib plot), needing no viewport API; an in-viewport HUD plot
  is offered only if required.

### A4 — Host-side warp uniform

- Add `Displacements []float64` (per-vertex xyz) on the result primitive and a cheap
  `SetWarpScale(clientId string, scale float64)` method. Host computes
  `pos = base + scale·disp` in the vertex shader.
- Makes the **deform-scale slider** and **mode-shape animation** a single scalar
  update per frame — zero per-frame wire traffic. (Without it, every tick re-ships the
  whole coords array.)

### A5 — Partial update + mapper banding

- `UpdateGraphicsVertices(clientId, …)` / `UpdateGraphicsScalars(clientId, …)` re-ship
  only positions/values (time-step cycling, field switch) rather than topology +
  mapper.
- Optional `Mode: "smooth"|"banded"`, `Bands: N` on `GraphicsColorMapper` lets the host
  band crisply without the add-in remeshing. Otherwise covered by a staircase LUT
  (cheap) or contour-split (crisp) add-in-side.

### A6 — Colormap fix (add-in-side, no API change)

Replace the 5-stop jet/rainbow ramp (`ccx/mapper.go`). Jet has non-monotonic luminance
(false banding, hidden gradient, not colorblind-safe). Use **Turbo** (dense ≥32-stop
LUT) for unsigned magnitude fields (von Mises, |disp|, Peeq, temperature), offer
**Viridis** as the perceptually-uniform option, and a **coolwarm diverging** map
centered at zero for signed fields (principal stress) with auto-symmetrized min/max.

## 4. Pillar B — Add-in restructure (the projection seam)

### Aggregate (`ccx/femmodel`, pure — no `api`, no host)

```
Analysis  (root aggregate; FreeCAD Fem::FemAnalysis)
├── Solver     SolverObject     (analysis type, eigenmodes, transient time, working dir)
├── Mesh       MeshObject       (max element size mm, element order)
├── Materials  []MaterialObject (elastic/thermal/EM props; one per BodyScope; "all" = default)
├── Loads&BCs  []ConstraintObject (existing ConstraintSpec kinds + whole-body loads
│                                  gravity/centrifugal + thermal/EM BCs + contact)
└── Results    []ResultObject   (result field, deform scale + post-pipeline filters)
```

- `FEMObject` interface: `ObjectID() string; Category() Category; Name() string`.
  `Category` ∈ {Solver, Mesh, Material, Constraint, Result}.
- **Invariants** (enforced inside `Analysis` mutators): exactly one Solver and one Mesh
  (v1); ≥1 Material with `BodyScopeAll` as fallback; unique `ObjectID`s; a
  `ConstraintObject`'s refs only of kinds it accepts; static-solvability stays
  **permissive** (the engine already synthesizes a default support when the constraint
  set is empty — the aggregate *warns*, does not block).
- The existing `ConstraintSpec` (`kind + params + face refs`) is already first-class,
  so `ConstraintObject` is largely a wrapper; whole-body loads and field BCs that today
  are flat `StudySettings` scalars become `ConstraintObject`s.

### Projection seam (`ccx/project.go`)

```go
func projectAnalysis(a *femmodel.Analysis) (StudySettings, []ConstraintSpec)
```

`Engine.settings StudySettings` becomes `Engine.analysis *femmodel.Analysis`.
`RunStudyOnHost` changes by one line at the top
(`settings, specs := projectAnalysis(e.analysis)`); the rest of the pipeline and every
test below the seam are byte-for-byte unchanged.

### UI layer (`ccx`, links `api/client`; mirrors CAM's `bridge/`)

```
ccx/femmodel/            (PURE; F.I.R.S.T tests)
  analysis.go            aggregate + mutators (Add/Remove/Reorder, invariants)
  solver_object.go  mesh_object.go  material_object.go
  constraint_object.go   (category + param bag + reference list + Accepts)
  result_object.go       (+ post-pipeline filters)
  category.go

ccx/  (UI layer)
  project.go             projectAnalysis(): aggregate → StudySettings + []ConstraintSpec  ← SEAM
  analysis_tree.go       BrowserPaneSpec: Analysis ▸ Solver/Mesh/Materials/Loads&BCs/Results
  analysis_tree_events.go double-click→task panel, menu→add/delete/reorder
  ribbon_layout.go       femRibbonSpots → "FEA" tab panels
  task_solver.go  task_mesh.go  task_material.go
  task_constraint.go     one builder per constraint Category (with referenceList)
  task_result.go         result-display + filter task panel
  results.go             post-processing subtree (pipeline + filters)
  commands.go            per-object Create + Open-task-panel commands
  engine.go              Engine.analysis; Notify routes the 3 events
  recordinghost_test.go  named host fake (à la CAM)
  panel.go               RETIRED (replaced by tree + task panels)
```

- **Ribbon "FEA" tab** panels (keyed on command id, CAM's `ribbon_layout.go` shape):
  *Model* (New Analysis, Solver, Material) · *Mechanical Loads & BCs* (Fixed, Roller,
  Symmetry, Elastic, Force, Pressure, Hydrostatic, Displacement, Gravity, Centrifugal) ·
  *Thermal BCs* (Temperature, Heat Flux, Convection, Body Heat, Radiation) · *Mesh*
  (Gmsh Mesh) · *Solve* (Run) · *Results* (Show Results, Add Filter).
- **Tree node-id scheme** (parsed like CAM's `op:N`): `solver`, `mesh`, `mat:N`,
  `con:N`, `result:N`, `result:N/filter:M`. Double-click → open task panel; context
  menu → Edit/Delete/Duplicate; `runAndRefreshTree` re-declares the pane after a
  mutation.
- **Goroutine discipline** (reused verbatim): any command / menu / task-panel-accept
  that makes host calls runs **off** the session goroutine via `launchRun` (else the
  dispatcher deadlocks); a `PanelValueChangedEvent` that only mutates `femmodel` runs
  inline. The `running` coalescing guard for the solve stays.

## 5. Pillar C — Post-processing math layer

Two pure cores power all ten filters. Tolerances are model-scaled
(`ε_geom = 1e-9·bboxDiag`, `ε_vol = ε_geom³`, `ε_area = ε_geom²`,
`ε_bary ≈ 1e-7` dimensionless, field thresholds relative to field range).

### Core 1 — Marching tetrahedra (clip, cut, contour)

- Classify the 4 tet vertices by `sign(φ)`; only **three** non-trivial topologies
  (1 positive → 1 triangle; 2 positive → quad/2 triangles; 0/4 → empty). No
  marching-cubes ambiguity.
- Edge crossing `t = φ_i/(φ_i − φ_j)`; interpolate geometry **and** color with the
  **same** `t` (continuity). Well-conditioned because `φ_i, φ_j` have opposite signs.
- **Consistent tie-break**: fold `φ == 0` into the **positive** side **uniformly**
  (no symmetric "on-surface" band) — crack-free by construction.
- **Crack-free dedup**: key each emitted vertex by `(min(nodeI,nodeJ),
  max(nodeI,nodeJ), levelIndex)` — topological, never coordinate hashing.
- **Winding**: orient triangle normal to agree with `∇φ` (constant on a linear tet).
- **C3D10**: split each quadratic tet into **8 linear sub-tets** using all 10 nodal
  values (corners `1,2,3,4`; mids `5=(1,2) 6=(2,3) 7=(3,1) 8=(1,4) 9=(2,4) 10=(3,4)`;
  4 corner sub-tets + central octahedron split along `5–10`). Corner-only is acceptable
  only when the field is locally near-affine.
- **Box clip**: successive 6-plane Sutherland–Hodgman (not MT on `max`-of-planes, which
  rounds box corners). Transform into box-local axes for oriented boxes.

### Core 2 — Point location + isoparametric interpolation (probe, line, linearization)

- **Point-in-tet** via barycentric solve (`[x1−x0|x2−x0|x3−x0]λ = p−x0`); inside iff
  `λ_i ≥ −ε_bary`; the 3×3 determinant `= 6V` is the sliver detector.
- **Shape functions**: C3D4 linear `N_i = λ_i`; C3D10 quadratic corners
  `N_i = λ_i(2λ_i − 1)`, mids `N_ij = 4 λ_i λ_j`; verify `Σ N ≡ 1`.
- **Geometry→(ξ,η,ζ)**: straight-edged tets (the common case) — locate with the linear
  corner barycentric solve, then feed those `λ` into the quadratic shape functions
  (exact). Curved-edge tets — Newton on `r(ξ)=x(ξ)−p` seeded from the linear `λ`.
- **Acceleration**: BVH over element AABBs (built once, reused); face-adjacency walk
  for spatially-coherent samples (data-along-line, SCL) — O(1) amortized.

### Filter-by-filter

1. **Warp** — `x' = x + k·U`; recompute deformed-mesh vertex normals; auto-scale
   `k = α·bboxDiag/max|U|` (α≈0.05–0.10); guard `max|U|→0`.
2. **Scalar clip** — Core 1 with `φ = g − c`; emit the **sub-mesh** boundary (keep
   interior tets whole, cut straddlers); optional band via a second `g ≤ c_hi` pass.
3. **Cut (plane/sphere/cylinder)** — Core 1 with the spatial implicit sampled at nodes;
   emit the cross-section surface; plane is exact, sphere/cylinder converge with mesh.
4. **Box clip** — successive plane clips.
5. **Contours / isosurfaces** — Core 1 at N levels; `levelIndex` in the dedup key.
6. **Glyphs** — one **batched** arrow mesh (reuse `glyphmesh.arrow`), grid-downsampled
   to keep the max-|U| node per cell; bbox-fraction auto-scale.
7. **Data along line** — Core 2 walk; sample `g(p)=Σ N_k g_k`; mark out-of-mesh gaps;
   plot in the task panel.
8. **Data at point** — Core 2 probe; report value + element id + barycentric coords.
9. **Linearized stresses** — sample the **full stress tensor** along the SCL by Core 2;
   per component `σ_m = (1/t)∫σ dx`, `σ_b = (6/t²)∫σ(t/2−x)dx` (Simpson, M≥5,
   subdivided at element crossings); reduce (von Mises/principal) **after** linearizing
   the tensor, never before.
10. **Calculator** — per-node expression eval over raw tensor/displacement/principal
    fields (shunting-yard/AST); validate referenced field names.

### Rendering (over the client-graphics seam)

- Each filter output is a per-vertex-scalar triangle mesh pushed as its own named
  client-graphics display object (`AddFloodPlot` with `MapperName` indirection so
  min/max/band edits re-push only the mapper). Show alongside (separate group) or
  instead (hide/delete the base group) per the task panel. Glyphs = one batched mesh.
- Live deform slider / animation use A4 `SetWarpScale`; long transients with a
  different field per step use A5 partial-scalar update or a hidden-sibling "flipbook"
  flipped with `SetNodeVisible`.

## 6. Phasing (each phase = its own PR set)

The API work splits into two waves so the add-in restructure is never blocked: the
panel/task surface (A1+A2) only gates Phase 3; the graphics surface (A3–A5) only gates
Phase 4. Phases 1–2 need **no** new API and can start immediately.

| Phase | Deliverable | Repos | Depends on |
|---|---|---|---|
| **0a** | Panel/task API: `referenceList` (A1) + `TaskPanelSpec` (A2) — api release + host impl + pin bump | API → Host | — |
| **0b** | Graphics API: HUD legend lane (A3) + `SetWarpScale`/`Displacements` (A4) + partial update/banding (A5) — api release + host impl + pin bump | API → Host | — |
| **1** | `femmodel` aggregate + `projectAnalysis` seam (pure refactor; all tests stay green) | CalculiX | — |
| **2** | Analysis browser tree + FEA ribbon; retire god panel | CalculiX | 1 |
| **3** | Per-object modal Task panels (incl. `referenceList` constraint pickers) | CalculiX | 0a, 2 |
| **4** | Result display: colormap (A6), legend HUD (A3), deform slider (A4), min/max/bands | CalculiX | 0b, 3 |
| **5** | Post-processing engine + the 10 filters as tree objects | CalculiX | 3 (0b for live warp/legend) |
| **6** | Animation (mode-shape/time-step), histogram, examples, polish | CalculiX | 0b, 5 |

Each API wave is the outward-facing api→host release dance (pre-bump the add-in's `api`
pin before pushing or CI stalls on the api-version bot). Phases 1–6 are add-in PRs.
Every phase is shippable on its own and the add-in keeps working at each step: the
tree/ribbon replace the god panel in Phase 2, after which task panels and filters layer
on. **Planning note:** this design is the umbrella; each phase gets its own
implementation plan rather than one plan for all six. The first plan should cover
**Phase 0a + Phase 1** (the API panel/task surface and the pure-refactor spine, which
have no interdependency and unblock the visible Phases 2–3).

## 7. Testing strategy

- **TDD throughout** (red → green → refactor). Every new function gets a test; bug
  fixes get a regression test.
- **`recordingHost` fake** (named, à la CAM) for engine/UI tests — assert
  `DockableWindows`/`TaskPanels`/`Browser`/`Commands`/`ClientGraphics` calls without a
  host.
- **`femmodel` unit tests** — aggregate invariants, projection equivalence (a built
  Analysis projects to the same `StudySettings`+specs the old flat path produced).
- **Math golden tests** — marching-tetrahedra and interpolation cores vs analytic
  oracles (a known isosurface area, a probe value at a known point, membrane/bending of
  a linear-through-thickness stress); degenerate/sliver and `φ=0`-on-node cases.
- **Live MCP validation** at the end of phases 2–6 via MCPBridge: build a part, run a
  study, open a task panel, **Add a face to a `referenceList`**, switch result fields,
  drag the deform slider, add a clip filter — and **capture the viewport** to confirm
  the legend stays screen-fixed under orbit, the field reads correctly, and bands are
  mesh-independent.
- Coverage > 80%, duplication < 3%, full lint + markdownlint locally before each PR.

## 8. Key decisions (ADRs to write)

- **ADR-A1** — `referenceList` is a new control kind with its own change event, not an
  overloaded scalar value. (In `Oblikovati/architecture`, extends ADR-0018/0019.)
- **ADR-A2** — Modal Task panel is a new `TaskPanelSpec` surface, not a `Modal` flag on
  `DockableWindowSpec`.
- **ADR-A3** — Screen-space HUD lane for legends/plots; warp uniform + partial graphics
  update for live result viz.
- **ADR-B1** — The Analysis aggregate is the source of truth; `StudySettings` becomes a
  solve-time projection. (In the CalculiX add-in `architecture/decisions`.)
- **ADR-B2** — The UI layer edits the aggregate; the engine owns only the pipeline.
- **ADR-C1** — Marching-tetrahedra + point-location are the two post-processing cores;
  C3D10 handled by 8-sub-tet split; box clip by successive plane clips.

## 9. Handoff contract (free vs fixed)

- **Fixed**: the `api` DTO/field/method-name/event-name strings (frozen once shipped;
  reuse, never re-declare); the bulk-state semantics of references; the projection seam
  (the engine consumes `StudySettings` and nothing else); the dependency directions and
  the `femmodel`-is-pure rule.
- **Free**: `femmodel` object field layout and mutators; task-panel/tree builders;
  ribbon glyphs and grouping; projection internals; engine pipeline internals; the
  filter algorithms' internals (behind golden tests).

## 10. References

- In-repo parity precedent (copy faithfully):
  `Oblikovati.AddIns.CAM/bridge/{browser_tree.go, browser_tree_events.go,
  ribbon_layout.go, job_edit_window.go, op_editor_pages.go, simulator.go}`.
- FreeCAD FEM workbench: `FreeCAD/src/Mod/Fem/` (Gui/Workbench.cpp commands;
  femtaskpanels/*; femsolver/calculix/writer.py; femobjects/result_mechanical.py;
  feminout/importCcxFrdResults.py).
- Math: Lorensen & Cline 1987 (marching cubes); Treece/Prager/Gee 1999 (regularised
  marching tetrahedra); Zienkiewicz/Taylor/Zhu (isoparametric shape functions);
  de Berg et al. (point location); Sutherland & Hodgman 1974 (polygon clipping);
  ASME BPVC VIII-2 Annex 5.A (stress linearization).
- Existing add-in: `ccx/{render.go, mapper.go, frd.go, tetmesh.go, vonmises.go,
  principal.go, glyphmesh.go, units.go, panel.go, engine.go, study.go}` and
  `architecture/decisions/ADR-0001..0008`.
