# Oblikovati CalculiX

A host add-in that integrates **CalculiX** (`ccx` — Guido Dhondt's open-source
three-dimensional finite-element solver) as a **stress-analysis provider** for
Oblikovati. It links **only** the Apache-2.0 public API (`oblikovati.org/api`) and
reaches the running host over the C ABI (ADR-0016) — never the GPL application
internals.

> Built/versioned/shipped exactly like the [FEMM bridge](../Oblikovati.AddIns.FEMMBridge):
> a cgo `c-shared` library, its own Go module pinned to a published `oblikovati.org/api`
> release, sibling repos wired by `.github/actions/siblings`, and an API-tracking release
> pipeline.

## Pipeline

1. **Resolve study** — material + the selected faces that carry loads / boundary
   conditions (`Materials.List`/`Get`, `Model.Selection`).
2. **Surface mesh** — pull the body's triangulated surface (`Body.CalculateFacets`) and
   weld it to a watertight, manifold soup.
3. **Volume mesh** — drive a vendored **gmsh** (subprocess) to turn the surface into a
   solid tetrahedral (C3D10) mesh; recover B-rep `FaceKey` → mesh-facet groups
   geometrically so loads/BCs bind to the right element faces.
4. **Write deck** — emit a CalculiX `.inp` input deck (`*NODE`/`*ELEMENT`, `*MATERIAL`,
   `*SOLID SECTION`, `*STEP` + `*BOUNDARY`/`*CLOAD`/`*DLOAD`, output requests).
5. **Solve** — the vendored headless `ccx` solves → `.frd`/`.dat`.
6. **Render** — parse the results, compute von Mises, and push the stress + displacement
   (and deformed shape) back as a `clientGraphics` heatmap + color-mapper legend.

## The central design problem: surface mesh → solid mesh

CalculiX consumes a **solid tetrahedral** mesh, but the public API exposes only a
**surface triangulation**. The add-in interposes a vendored **gmsh** stage (welded
surface STL → C3D10 tets) and recovers the face bindings by geometry (centroid proximity
+ normal alignment), because the whole-body facet set carries no `FaceKey` labels and the
volume mesher renumbers nodes. See the inline `GAP #n` markers and the ADRs in
`architecture/decisions/`.

## API-gap findings (this add-in is also a v1 API gap audit)

1. **No volume mesh** over the wire (surface tessellation only; STEP export reserved) —
   worked around by vendoring gmsh.
2. **No `FaceKey` identity on the whole-body facet set** — recovered geometrically via
   per-face `Body.FaceCalculateFacets`.
3. **No edge identity in stroke results** — loads/BCs bind to faces only in v1.
4. **Client-graphics results don't persist in `.obk`** — overlays are live-only.

## Build

```sh
make build      # cgo c-shared library into build/
make install    # build + copy library + manifest into the host's add-ins dir
make test       # cgo-free ccx engine unit tests + add-in<->host integration
```

The vendored solver (`ccx`) + mesher (`gmsh`) build headless via CMake (landing with the
M1 static slice); the `requireSolver`-gated end-to-end tests look for them under
`vendor-src/{ccx,gmsh}/build` or at `OBK_CCX_BIN` / `OBK_GMSH_BIN`:

```sh
make build-solvers   # cmake-build ccx + gmsh into vendor-src/*/build
```

Local dev resolves the sibling `Oblikovati.API` + `Oblikovati` checkouts via a
git-ignored `go.work`; CI injects the equivalent replaces (`.github/actions/siblings`).

## Layout

```
export.go / hostcaller.go / manifest.go   C-ABI c-shared shell (the only cgo)
manifest.json                             add-in manifest (capabilities)
ccx/                                       cgo-free FEA engine + pipeline
  engine.go      Notify/launch/status orchestration + HostCaller
  commands.go    CCX.RunStudy command + Setup
  panel.go       dockable study-parameters window
  analysis.go    AnalysisType / ElementOrder / StudySettings
  study.go       surface→volume→deck→solve→render orchestration
  ...            (mesh / deck-writer / results / render files land in M1+)
architecture/decisions/                    ADRs for the integration design
vendor-src/                                vendored ccx + gmsh (M1; built via CMake)
```

## Licensing

This repository is **GPL-2.0-only** — the add-in's own code is GPL; the only thing it
links is the Apache-2.0 public `oblikovati.org/api`.

**CalculiX (`ccx`) and gmsh are both GPL**, so they are license-compatible with this
repo. They are nonetheless built as **standalone binaries** the add-in runs **as
subprocesses** (arm's-length, file-based `.inp`/`.frd`/`.msh` exchange) — the engine never
links them into its own process. Vendored sources and their upstream notices live under
`vendor-src/{ccx,gmsh}/`.
