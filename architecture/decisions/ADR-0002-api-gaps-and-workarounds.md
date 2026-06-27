# ADR-0002 — v1 API gaps surfaced by FEA, and their workarounds

**Status:** accepted (2026-06) · **Relates to:** [ADR-0001](ADR-0001-calculix-fea-integration.md).

## Context

Driving a real 3D FEA solver against the v1 public API surfaces gaps the API does not
yet close. This ADR records them, the v1 workaround behind a stable seam, and the
future API extension that would retire the workaround — so each gap is a localized
change later, not a rewrite.

## The gaps

| # | Gap | v1 workaround (seam) | Future API extension |
|---|-----|----------------------|----------------------|
| 1 | **No volume mesh** over the wire (surface tessellation only; solid-geometry export reserved) | vendor + subprocess gmsh; weld surface → STL → C3D10 (`VolumeMesher` seam) | host-side tet meshing, or solid-geometry export so the mesher reads exact geometry instead of a chordal approximation |
| 2 | **No face identity on the whole-body facet set** (the per-face index partition is unlabeled) | per-face `Body.FaceCalculateFacets` (keyed by face reference key) + spatial/normal matching to the volume mesh's boundary facets (`FaceGroups` seam) | a facet→face-key tag on the facet result, or a face-tagged mesh export op |
| 3 | **No edge identity in stroke results** (faces have a reference key, edges do not) | loads/BCs bind to **faces only** in v1; edge selections are rejected | edge reference keys in the stroke result |
| 4 | **Client-graphics results are live-only** — they do not persist into the saved document | the result overlay is pushed live each run (same as the FEMM bridge) | a persisted result attribute set / document-scoped result store |
| 5 | **No analytic surface evaluator** — meshes are built from a chordal tessellation | mesh-tolerance knob; nearest-face fallback for seam facets on tangent-continuous faces | a surface evaluator, or face-classified meshing |
| 6 | **No surrounding-air mesh** (only the conductor solid is meshed) — blocks true CalculiX `*ELECTROMAGNETICS` (magnetostatics/induction need an A–V–Φ air domain) | electromagnetic analysis is scoped to **electric conduction** via the heat-transfer analogy on the conductor-only mesh (see [ADR-0003](ADR-0003-electromagnetic-scope.md)) | generate an air box around the part and tie the domain interfaces (a meshing project, not a deck change) |
| 7 | **No node-conformal mesh across body interfaces** — each body is meshed independently, so touching bodies don't share interface nodes | multi-material studies are valid for disjoint / independently-constrained bodies; the per-material-ELSET deck is exact (see [ADR-0004](ADR-0004-multi-material-per-body.md)) | conformal combined-geometry meshing, or `*TIE` constraints generated at coincident interfaces, for bonded multi-material contact |

## Decision

Ship v1 with each gap isolated behind the `VolumeMesher`, `FaceGroups`, and result-render
seams named above, and file the corresponding host-side API extensions upstream. Closing
any one gap is then a change at a single seam, not across the pipeline.
