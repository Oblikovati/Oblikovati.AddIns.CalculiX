# ADR-0001 — CalculiX as a subprocess FEA provider behind the public API

**Status:** accepted (2026-06) · **Builds on:** Oblikovati ADR-0016 (in-process
C-ABI shared-library add-ins), ADR-0018 (Apache-2.0 `/api` contract), ADR-0003
(analysis as an out-of-core compute domain).

## Context

Oblikovati needs finite-element stress analysis. Per the host architecture, FEA is an
**add-in domain**: an established solver integrated behind the public API, not built
into the kernel. The sibling FEMM bridge already proved a full simulation workflow can
ship as a C-ABI shared library that links only the Apache-2.0 `oblikovati.org/api`.

This add-in does the same for structural FEA using **CalculiX** (`ccx`), an
open-source 3D finite-element solver. CalculiX is driven by a text **input deck**
(`.inp`) describing nodes, elements, materials, sections, boundary conditions, loads,
and an analysis step; it writes results to `.frd`/`.dat` files.

The defining constraint: **CalculiX consumes a solid tetrahedral mesh, but the public
API exposes only a surface triangulation.** There is no volume mesh, no analytic
surface evaluator, and no solid-geometry (STEP/B-rep) export over the wire today.

## Decision

1. **Run the solver as a subprocess, never linked.** The engine writes an `.inp` deck
   to a per-run temporary directory and invokes a vendored headless `ccx` binary
   (`ccx -i <base>`), then reads back `.frd`/`.dat`. Binary discovery: `OBK_CCX_BIN`,
   else `vendor-src/ccx/build`. This keeps the boundary file-based and arm's-length,
   matching the FEMM bridge's posture.

2. **Bridge surface → solid with a vendored mesher.** Because the host yields only
   surface facets, the add-in vendors **gmsh** (also a subprocess) and feeds it a
   welded, watertight surface (STL) to produce quadratic tetrahedra (C3D10 by default;
   linear C3D4 is too stiff to match an analytic bending oracle). Discovery:
   `OBK_GMSH_BIN`, else `vendor-src/gmsh/build`.

3. **Recover face bindings geometrically.** The whole-body facet set carries no face
   identity and the volume mesher renumbers nodes, so the engine maps each selected
   face's triangles (`Body.FaceCalculateFacets`, which is keyed by the face reference
   key) onto the gmsh boundary facets by centroid proximity **and** normal alignment.
   The resulting node/element-face groups feed `*BOUNDARY`/`*CLOAD`/`*DLOAD`.

4. **Model the deck as an ordered writer with a constraint-writer registry.** The deck
   has a fixed section order (mesh → element sets → material → section → step →
   loads/BCs → output). Each load/boundary-condition kind is a `ConstraintWriter` with
   a two-phase contract (emit sets before the step, emit cards inside it), registered
   in order. New analysis types and constraints are added by registering a writer, not
   by editing the orchestrator.

5. **Render results as client graphics.** Parsed nodal stress/displacement is mapped
   onto the surface as a color-mapped heatmap with a legend, plus a deformed-shape
   overlay, pushed on the owning document's persistent graphics lane.

## Consequences

- Structural and thermal FEA need **no new public-API material fields** — the existing
  mechanical/thermal/anisotropic material groups already cover CalculiX `*ELASTIC` /
  `*DENSITY` / `*EXPANSION` / `*CONDUCTIVITY`.
- The add-in is also a v1 **API gap audit** (see [ADR-0002](ADR-0002-api-gaps-and-workarounds.md)):
  no volume mesh, no facet→face identity, no edge identity, no result persistence.
- Licensing is clean: `ccx` and gmsh are both GPL, compatible with this GPL-2.0 repo,
  and are still isolated as subprocesses so the engine process links neither.
- The geometric face-binding is exact for crease-bounded faces; tangent-continuous
  faces fall back to nearest-face assignment for seam facets (a documented limitation
  until the host can tag facets by face).
