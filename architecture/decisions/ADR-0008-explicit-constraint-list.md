# ADR-0008 — A study is an explicit, resolved-at-run-time list of constraints

**Status:** accepted (2026-06) · **Builds on:** [ADR-0001](ADR-0001-calculix-fea-integration.md)
(the deck-writer architecture and its `ConstraintWriter` registry).

## Context

The mechanical study was configured by an **implicit selection convention** baked into
`buildModel`: the *first* selected face became a full clamp, the *remaining* faces carried one
load whose type was a single global setting. This caps a study at **one support and one load**
and — more limiting — cannot express **directional or partial** constraints. A roller or a
symmetry plane fixes only the face-*normal* direction; such a constraint is singular on its own
and is only well-posed *combined* with others. The implicit convention has no way to hold
several heterogeneous constraints, so roller/symmetry (and later a local-frame or bearing
constraint) were simply unreachable.

The deck side already had the right shape — the `ConstraintWriter` registry makes *adding a deck
card* an additive, one-file change. The **intent** side had no equivalent seam.

## Decision

Introduce a **`ConstraintSpec`** — one study constraint as *intent*: a kind, a selection of host
face reference keys, and its parameters. It is **not** mesh-bound; it knows how to **`Resolve`**
itself — bind its face refs to mesh nodes / element-faces (via the run-time `FaceGroups`) and
append the matching entries to the existing `AnalysisModel`. This is the intent-side analog of
`ConstraintWriter`: a new constraint kind is a new small spec struct plus a factory case, nothing
more.

`buildModel`'s mechanical path now **resolves a list of specs**:

```
specs := settings.Constraints        // the explicit list the panel builder adds
if len(specs) == 0 {
    specs = defaultConstraints(...)  // synthesizes today's first-clamp / one-load convention
}
for _, s := range specs { s.Resolve(rc) }   // rc = {Model, Mesh, FaceGroups}
```

`defaultConstraints` reproduces the former convention exactly as an explicit spec list, so when a
study adds no constraints the behaviour is unchanged. The `AnalysisModel` value types and the
`ConstraintWriter` registry are **untouched**; the heat / electromagnetic / coupled field-BC
paths keep their own apply functions and are **out of scope** — the constraint list governs the
**mechanical** (static / modal / thermal-stress) path only.

## Consequences

- Adding a constraint kind is one small struct + one `Resolve` method (open/closed), symmetric
  with the deck-writer seam. A roller is a `FixedConstraint` on a *normal-derived* DOF — no new
  deck card and no new `AnalysisModel` field.
- Multiple heterogeneous constraints become expressible, so roller / symmetry / local-frame /
  bearing constraints are now reachable (each a future spec).
- A spec is plain data (refs + params), a natural unit for future `.obk` study persistence.
- The change is **behaviour-preserving**: the entire existing oracle suite passes unchanged
  through the real solver, and a model-level test asserts the synthesizer reproduces every
  support / load / analysis branch plus the explicit-override path — the seam is proven before
  any new constraint physics is added.
- Two representations of "a constraint" now coexist — the intent `ConstraintSpec` and the
  resolved `AnalysisModel` entry; `Resolve` is the deliberate translation between them.
- The panel must evolve from one-support / one-load fields to a constraint **builder**; with no
  list widget in the host's control vocabulary, editing existing constraints is add/clear-only
  until the host gains one (a later phase).

## Rejected alternatives

- **A god settings struct with N optional constraint slots / a tagged param bag** — anemic,
  couples every kind's fields into one type, and recreates the problem one level up.
- **Letting the panel mutate the `AnalysisModel` directly** — collapses the intent/resolved
  distinction and bypasses run-time face binding (refs are not mesh nodes until the part is
  meshed).
- **Folding the heat / EM / coupled BCs into the list now** — a larger blast radius on validated
  paths for no roller/symmetry benefit; deferred.

## Phasing

This ADR records **Phase 1** (the behaviour-preserving seam). Follow-ons: **Phase 2** adds the
panel constraint-builder and populates `StudySettings.Constraints`; **Phase 3** adds roller /
symmetry specs whose constrained DOF is *derived from the face normal* at resolve time, gated by
a symmetry-model oracle; later phases add local-frame and bearing constraints behind the same
seam.
