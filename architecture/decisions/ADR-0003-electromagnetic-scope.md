# ADR-0003 — Electromagnetic analysis scoped to electric conduction (the heat-transfer analogy)

**Status:** accepted (2026-06) · **Builds on:** [ADR-0001](ADR-0001-calculix-fea-integration.md)
(CalculiX as a subprocess FEA provider on a solid tetrahedral mesh).

## Context

The capability roadmap lists *electromagnetic analysis* as part of the full CalculiX
feature set. CalculiX exposes a true `*ELECTROMAGNETICS` step (magnetostatics,
time-domain induction, electromagnetic–thermal coupling), but it is built on a
three-domain **A–V–Φ formulation**: in addition to the conductor, the surrounding
**air/free space must be meshed** (a Φ-domain and an A-domain), with tied-contact MPC
interfaces generated between the conductor and the air regions. Every material in such a
run must also carry a `*MAGNETIC PERMEABILITY` card tagged with a domain number.

This add-in produces a **conductor-only solid mesh** (the part's surface triangulation
welded and tet-meshed, per ADR-0001). It has no surrounding air box and no way to
generate one over the public API. A conductor-only mesh therefore **cannot** drive
`*ELECTROMAGNETICS` — the formulation has no Φ/A region to close against and aborts.

The same constraint is visible upstream: a mature reference FEA workbench does **not**
use CalculiX for magnetics at all (it reserves true magnetostatics/magnetodynamics for a
different solver). Its CalculiX electromagnetic support is limited to the **electrostatic
/ electric-conduction** case, which it maps onto the steady-state heat equation.

## Decision

Scope `AnalysisElectromagnetic` to **steady-state electric conduction / electrostatic
potential**, solved through CalculiX's **electric–thermal analogy** on the existing
solid-only mesh:

- **Step:** `*HEAT TRANSFER, STEADY STATE` (the Laplace operator is shared).
- **Field DOF:** the temperature degree of freedom (11) *is* the electric potential; the
  `.frd` `NT`/`NDTEMP` block is read back as the potential field.
- **Material:** `*CONDUCTIVITY` carries the **electrical** conductivity (a dedicated
  `ElectricalSigma`, distinct from the thermal `Conductivity`).
- **Boundary conditions:** an applied potential on the first selected face and ground
  (0 V) on the rest, both written as `*BOUNDARY` on DOF 11 — reusing the heat-transfer
  Dirichlet writer.
- **Result:** the nodal potential field, rendered with the shared scalar-field flood plot
  and reported in volts; the constraint aids mark the high-potential face (red) and the
  ground face (cyan).

Because both ends are prescribed (a potential drop across the conductor), the steady
field is the linear Laplace solution and is **independent of the conductivity magnitude**
— validated by an analytic oracle (mid-plane potential = V₀/2, field span [0, V₀]).

## Consequences

- The electromagnetic path reuses the entire heat-transfer pipeline (step writer,
  material writer, DOF-11 Dirichlet writer, `NT` parser, scalar-field render) with only
  relabeling — no new solver machinery.
- **True magnetics is explicitly out of scope** for the solid-only mesh: magnetostatics,
  induction, and EM–thermal coupling all require air-domain meshing with `*MAGNETIC
  PERMEABILITY` domain tagging. Closing this gap is a future meshing project (generate an
  air box around the part and tie the interfaces), not a deck change — it is recorded in
  [ADR-0002](ADR-0002-api-gaps-and-workarounds.md) alongside the other meshing limits.
- The conductivity value does not affect the rendered potential for a two-Dirichlet
  problem; it is retained for honesty and to leave room for a future current/flux-driven
  variant (`*DFLUX`/`*CFLUX`), where it would scale the computed current.
