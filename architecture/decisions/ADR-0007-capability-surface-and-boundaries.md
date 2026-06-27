# ADR-0007 — Solver capability surface and its architectural boundaries

**Status:** accepted (2026-06) · **Builds on:** [ADR-0001](ADR-0001-calculix-fea-integration.md)
(integration architecture), and the capability ADRs [ADR-0003](ADR-0003-electromagnetic-scope.md)
(electrostatic scope), [ADR-0004](ADR-0004-multi-material-per-body.md) (multi-material),
[ADR-0005](ADR-0005-tie-constraints-bonded-bodies.md) (bonded bodies),
[ADR-0006](ADR-0006-coupled-temperature-displacement.md) (coupled thermomechanical).

## Context

The add-in set out to bring an established FEA solver to the host behind the public API, and has
since grown to the breadth ADR-0001 architected for. This ADR records, in one place, the
**complete capability surface** the add-in now supports, and — more importantly — the two
**architectural boundaries** that follow inevitably from one upstream constraint, so they are
documented as deliberate scope rather than rediscovered as gaps.

The single shaping constraint is the one ADR-0001 already named: the public API exposes only a
body's **surface tessellation**, so the add-in meshes the solid into a **tetrahedral** volume
(welded surface → gmsh → C3D10/C3D4 tets). Everything the solver does, it does on that solid tet
mesh. Two whole families of analysis assume a *different* discretization, and so sit outside the
boundary — see Decision.

## Decision

**Treat the solid tetrahedral mesh as the capability boundary.** Support every analysis, load,
boundary condition, and material model that is well-posed on a solid tet mesh; exclude the two
families that require a fundamentally different discretization, and say so explicitly.

### Supported surface (all validated through the real vendored solver by an analytic oracle)

- **Analyses:** linear static; natural-frequency / modal; buckling load factors; uncoupled
  thermal stress; coupled temperature–displacement (steady-state and transient); steady-state
  heat transfer; electric-conduction (the electrostatic analogy of ADR-0003).
- **Loads:** nodal force; surface pressure; gravity / self-weight; centrifugal (rotational) body
  force; enforced (prescribed) displacement.
- **Thermal boundary conditions:** prescribed temperature; surface heat flux; convective film
  exchange q = h·(T − T_sink); volumetric internal heat generation; and radiative exchange
  q = εσ(T⁴ − T_amb⁴) with the consistent-unit Stefan–Boltzmann constant.
- **Materials:** isotropic linear elastic; elastic–plastic (yield with a nonlinear ramped step);
  orthotropic (engineering-constants) elastic; plus density, thermal expansion, conductivity,
  specific heat, and electrical conductivity as the analyses require.
- **Multi-body interfaces:** auto-detected bonded `*TIE` (ADR-0005) and auto-detected unilateral
  **contact** pairs (linear penalty pressure-overclosure with optional Coulomb friction), free to
  transmit compression and friction while opening in tension.
- **Results:** nodal von Mises, displacement magnitude, and max/min principal fields; modal and
  buckling eigenvalues — rendered as a heatmap + legend + deformed shape on the owning document.

### Excluded by the solid-mesh boundary (deliberate, not deferred bugs)

1. **Beam and shell section analyses.** Beam (1-D line) and shell (2-D surface) elements carry
   their cross-section / thickness as a *property* on a lower-dimensional mesh, not as resolved
   solid geometry. A part is meshed here as a solid volume, so a slender or thin-walled part is
   analysed as the solid it is, **not** reduced to a beam/shell section. This is correct (just
   heavier) for parts that are genuinely solid, and out of scope for parts intended to be modelled
   as a section.
2. **True magnetostatics / induction (full electromagnetics).** Magnetic problems need the field
   solved in the **air/vacuum surrounding** the conductor, i.e. an air region meshed around the
   part. The add-in meshes only the part's solid, so it supports the electric-conduction /
   electrostatic analogy (ADR-0003) but **not** magnetic field, eddy-current, or induction
   analyses, which would require meshing the exterior domain.

## Consequences

- The capability surface is now broad and **uniformly validated**: every supported analysis,
  load, BC, and material has an analytic oracle that runs through the real solver in CI, so
  "supported" means "numerically checked," not merely "emits a card."
- The two exclusions are pinned to a single root cause — the solid tet mesh — so the path to
  lifting either is clear and localized: a host-side or add-in beam/shell section abstraction for
  (1), and an exterior-air meshing stage for (2). Neither is a defect in the current design; both
  are extensions beyond its boundary.
- New analyses/loads/BCs/materials continue to arrive as ADR-0001 intended — a registered
  constraint writer or a step/material-writer branch — not by reshaping the orchestrator. This
  ADR is the index of where that boundary currently sits.
