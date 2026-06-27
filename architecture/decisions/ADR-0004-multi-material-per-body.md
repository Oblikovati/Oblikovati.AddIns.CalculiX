# ADR-0004 — Multi-material studies via per-body element sets

**Status:** accepted (2026-06) · **Builds on:** [ADR-0001](ADR-0001-calculix-fea-integration.md)
(solid tet mesh from per-body surface facets).

## Context

A part can hold several solid bodies, each assigned its own material (a bi-material
moulding, a steel insert in an aluminium housing, …). The v1 study analysed only the
first body with a single material taken from the study panel. Two gaps blocked
multi-material analysis:

1. **The host exposed no read-back of a body's material.** `model.assignMaterial` was
   write-only; nothing reported which material is on a body.
2. CalculiX assigns a material through a `*SOLID SECTION` over an `*ELSET`, so a
   mixed-material part needs **one `*ELSET` + `*MATERIAL` + `*SOLID SECTION` per
   material** — but the deck writer emitted a single `Eall` set.

## Decision

1. **Extend the public API to report each body's material** (api v0.92.0): `BodyInfo`
   gains `MaterialID`, the body's effective assigned material (its own override, else the
   part default), populated by `body.list` from the host's assignment store. This is the
   read-back counterpart of the write-only `model.assignMaterial`. The add-in resolves the
   id to properties with `materials.get`. (Contract-first per the host's ADR-0018; the
   field is additive and backward-compatible.)

2. **Mesh every solid body and tag elements by body.** The study enumerates `body.list`,
   meshes each solid separately, and merges the meshes into one global numbering (node /
   element ids and gmsh surface tags offset per body), tagging each element with its source
   body. The picked faces bind by probing each body (the selection ref carries no body
   index).

3. **Resolve each body's material to deck units.** The host material (GPa, g/cm³,
   W/(m·K), Ω·m) converts to the CalculiX convention (MPa, t/mm³, mW/(mm·K), S/m); an
   unassigned body falls back to the panel material. Names are sanitised for the deck.

4. **Write per-material element sets.** Each body's elements form a `MaterialSection`; the
   deck writes one `*ELEMENT`/`*ELSET` block, one `*MATERIAL`, and one `*SOLID SECTION` per
   body, with materials deduplicated by name (bodies sharing a material collapse to one
   `*MATERIAL`). A single-body study still writes one `Eall` section — an unchanged deck.

## Consequences

- A mixed-material part solves with each body carrying its assigned stiffness/density/
  conductivity. Validated by an analytic **series-stiffness oracle** (a bar split into a
  stiff and a soft half: tip extension `P·L/2/(A·E₁) + P·L/2/(A·E₂)`, 0.9% error) and a
  live run where a host-assigned material (beryllium) measurably stiffened the result
  versus the panel default.
- **Bodies are meshed independently, so a shared interface between two touching bodies is
  NOT node-conformal** — the meshes do not share interface nodes, so load is not
  transmitted across a bonded interface, and a body left unconstrained and unloaded makes
  the static system singular. Multi-body studies are therefore valid for **disjoint bodies
  or bodies each independently constrained/loaded**; **bonded multi-material contact needs
  either a conformal combined-geometry mesh or tie constraints** at the interface — future
  work, recorded as a meshing limitation alongside the others in
  [ADR-0002](ADR-0002-api-gaps-and-workarounds.md). The per-material-ELSET deck itself is
  exact on a conformal mesh (the oracle runs on one).
- The single-body path is unchanged: same `Eall` deck, but the material now comes from the
  body's host assignment when one exists, instead of always the panel default.
