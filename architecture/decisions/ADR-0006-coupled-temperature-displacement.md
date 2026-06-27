# ADR-0006 — Coupled temperature-displacement (steady-state and transient)

**Status:** accepted (2026-06) · **Builds on:** [ADR-0001](ADR-0001-calculix-fea-integration.md);
extends the heat-transfer and uncoupled thermal-stress paths.

## Context

The add-in already had two thermal analyses:

- **Heat transfer** (`*HEAT TRANSFER, STEADY STATE`) — solves a temperature field from
  prescribed face temperatures and conduction, but no mechanics.
- **Thermomech** (uncoupled) — applies a single *uniform* `*TEMPERATURE` rise and solves the
  static stress from `*EXPANSION`; the temperature is a user input, not a solved field.

Neither produces the realistic case: a temperature field that is **solved** from boundary
conditions and whose *non-uniform* thermal expansion drives the deformation/stress. CalculiX
expresses this with `*COUPLED TEMPERATURE-DISPLACEMENT`, which solves the temperature
(DOF 11) and displacement (DOF 1-3) DOFs together — steady-state, or transient with a heat-
capacity term and time stepping.

## Decision

Add `AnalysisCoupledThermal` using `*COUPLED TEMPERATURE-DISPLACEMENT`, reusing the existing
thermal and mechanical machinery:

- **Boundary conditions:** the first selected face is the mechanical support (`*BOUNDARY`
  DOF 1-3) and is held at the cold/reference temperature; the remaining faces are held hotter
  (`cold + ΔT`) on DOF 11. The temperature field between them is solved by conduction.
- **Reference temperature:** `*INITIAL CONDITIONS, TYPE=TEMPERATURE` sets the stress-free
  reference (the cold temperature), so thermal strain is `α·(T − T_cold)`; it is omitted when
  the reference is zero (the CalculiX default).
- **Material:** `*ELASTIC` + `*EXPANSION` + `*CONDUCTIVITY`; a transient run adds `*DENSITY`
  + `*SPECIFIC HEAT` for the heat-capacity term.
- **Step:** `*COUPLED TEMPERATURE-DISPLACEMENT, STEADY STATE`, or — when a non-zero transient
  total time is set — `*COUPLED TEMPERATURE-DISPLACEMENT` with a `tinc, tper` time line.
- **Output / result:** `U`, `NT`, and `S`; the result is reported as the thermal-stress field
  (von Mises / displacement, via the existing static collector — the `.frd`'s final converged
  increment is read, which is the transient end-state).

## Consequences

- A part heated across a temperature gradient now solves its deformation from the *solved*
  field, the physically correct coupled thermomechanical case — distinct from the uncoupled
  uniform-ΔT thermomech. Validated by an analytic oracle: a bar held cold (fixed) on one face
  and hot on the other expands at its tip by `α·ΔT·L/2` (the integral of the linear thermal
  strain), **within 5%** through the real solver; a transient run reaching steady state
  converges to the same value.
- Steady-state and transient share one code path, differing only by the procedure card and
  the heat-capacity material cards — a single "transient total time" knob (0 = steady) selects
  between them.
- Reuses the temperature-BC writer, the expansion/conductivity material cards, and the static
  result collector; the only new deck pieces are the coupled procedure card, the
  `*INITIAL CONDITIONS` reference, and `*SPECIFIC HEAT`.
- Not yet covered: surface-film/convection and radiation boundary conditions, and
  time-history output (only the final increment is rendered) — straightforward follow-ons.
