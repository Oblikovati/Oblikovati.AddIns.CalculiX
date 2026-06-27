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
- **Drive (two modes):**
  - *Voltage* (Dirichlet–Dirichlet): an applied potential on the first selected face and
    ground (0 V) on the rest, both `*BOUNDARY` on DOF 11 — the Laplace problem. The steady
    field is the linear solution and is **independent of the conductivity magnitude**
    (validated: mid-plane potential = V₀/2, span [0, V₀]).
  - *Current* (Neumann–Dirichlet): the first face is grounded (`*BOUNDARY` 0 V) and a
    current density is **injected** on the loaded faces via `*DFLUX` (the heat-flux card
    reused). The current flows to ground, so the potential **scales with 1/conductivity**
    (Ohm's law, the analog of Fourier's `q·L/k` drop) — validated: fed-face potential
    `= J·L/σ`, exact through the real solver.
- **Material:** `*CONDUCTIVITY` carries the **electrical** conductivity (a dedicated
  `ElectricalSigma`, distinct from the thermal `Conductivity`).
- **Result:** the nodal potential field, rendered with the shared scalar-field flood plot
  and reported in volts; the constraint aids mark the high-potential face (red) and the
  ground face (cyan).

## Consequences

- The electromagnetic path reuses the entire heat-transfer pipeline (step writer,
  material writer, DOF-11 Dirichlet writer, `*DFLUX` flux writer for the current drive,
  `NT` parser, scalar-field render) with only relabeling — no new solver machinery.
- **True magnetics is explicitly out of scope** for the solid-only mesh: magnetostatics,
  induction, and EM–thermal coupling all require air-domain meshing with `*MAGNETIC
  PERMEABILITY` domain tagging. Closing this gap is a future meshing project (generate an
  air box around the part and tie the interfaces), not a deck change — it is recorded in
  [ADR-0002](ADR-0002-api-gaps-and-workarounds.md) alongside the other meshing limits.
- The conductivity value does not affect the rendered potential for a two-Dirichlet
  problem; it is retained for honesty and to leave room for a future current/flux-driven
  variant (`*DFLUX`/`*CFLUX`), where it would scale the computed current.
