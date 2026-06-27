# ADR-0005 — Bonded multi-body contact via auto-detected `*TIE` constraints

**Status:** accepted (2026-06) · **Builds on:** [ADR-0004](ADR-0004-multi-material-per-body.md)
(multi-body meshing with per-body element sets), closing its non-conformal-interface gap.

## Context

ADR-0004 meshes each solid body independently and merges the meshes, so two touching
bodies do **not** share interface nodes. Without bonding, load is not transmitted across
the interface and a body that is loaded but not directly constrained makes the static
system singular — multi-body studies were therefore limited to disjoint or independently-
constrained bodies.

CalculiX bonds non-conformal meshes with a `*TIE` (tied / mesh-tie contact): it generates
MPCs gluing a slave surface's nodes to a master surface's element faces, so the assembly
behaves as one continuous part. No `*SURFACE INTERACTION` or `*CONTACT PAIR` is needed —
`*TIE` is standalone and must precede `*STEP`.

## Decision

**Auto-detect bonded interfaces from the merged mesh and emit a `*TIE` per interface** —
no user setup, mirroring how a part of touching bodies is physically bonded.

1. **Detect interfaces geometrically.** Group the merged mesh's boundary facets per body,
   **merge a body's coplanar patches** back into one face (gmsh may split a flat face into
   several triangular patches — merging by plane makes detection independent of patching),
   then pair two faces from *different* bodies whose outward normals oppose and whose
   centroids coincide (within a small fraction of the mesh diagonal). Anti-parallel faces a
   body-length apart — the assembly's outer ends — fail the centroid test, so only genuinely
   touching faces pair.

2. **Emit `*TIE` + two element-face `*SURFACE`s** before the `*STEP`: the slave face is
   listed first, the master second (the CalculiX order), each as a `TYPE=ELEMENT` surface of
   `element, Sn` lines reusing the element-face resolution already validated by the pressure
   path. `POSITION TOLERANCE` is left to the CalculiX default (auto), which captures the
   coincident slave nodes onto the master faces.

3. **Detection runs for every study**; a single-body mesh has no cross-body pairs and emits
   no tie, so single-body decks are unchanged.

## Consequences

- A bonded multi-body part now solves as one continuous structure: load crosses the
  interface and a body held only through the tie is no longer singular. Validated by a
  **monolithic-bar oracle** — two boxes meshed *separately* (non-conformal interface),
  stacked, merged, and tied, extend under an axial load like the single bar they form
  (`δ = P·L/(A·E)`, 0.6% through the real solver).
- This **supersedes the ADR-0004 / ADR-0002 gap 7 limitation** for coincident **planar**
  interfaces. Residual limits: detection assumes the touching faces are coplanar and their
  centroids coincide (full-face contact); **partial-overlap or curved interfaces** are not
  yet detected, and the tie uses CalculiX's automatic position tolerance rather than a
  user-tuned value. These are refinements, not blockers.
- The interface is identified purely from mesh geometry; no host API or user constraint
  setup is involved, so a multi-body study "just bonds" the way the solid model implies.
