// SPDX-License-Identifier: GPL-2.0-only

package ccx

// ConstraintKind names one kind of study constraint (a support or a load). It is the tag the
// panel builder and the spec factory key on.
type ConstraintKind string

const (
	KindFixed          ConstraintKind = "fixed"
	KindRoller         ConstraintKind = "roller"
	KindSymmetry       ConstraintKind = "symmetry"
	KindElasticSupport ConstraintKind = "elastic support"
	KindForce          ConstraintKind = "force"
	KindPressure       ConstraintKind = "pressure"
	KindHydrostatic    ConstraintKind = "hydrostatic"
	KindGravity        ConstraintKind = "gravity"
	KindCentrifugal    ConstraintKind = "centrifugal"
	KindDisplacement   ConstraintKind = "displacement"
	KindThermalLoad    ConstraintKind = "thermal load"
)

// ConstraintSpec is one study constraint as user INTENT: a kind plus its parameters and (for the
// face-bound kinds) a selection of host face reference keys. It is NOT mesh-bound — Resolve binds
// its faces to mesh nodes / element-faces (via the run-time FaceGroups) and appends the matching
// resolved entries to the AnalysisModel. This is the intent-side analog of ConstraintWriter (the
// deck-side seam): a new constraint kind is a new spec + a factory case, nothing else.
type ConstraintSpec interface {
	Kind() ConstraintKind
	Resolve(rc *ResolveContext)
}

// ResolveContext is the run-time binding environment handed to each spec: the mesh exists and the
// picked faces are bound, so a spec can turn its face refs into nodes/element-faces and append to
// the model.
type ResolveContext struct {
	Model  *AnalysisModel
	Mesh   *TetMesh
	Groups *FaceGroups
}

// resolveSpecs folds a list of constraint specs into the model in order.
func resolveSpecs(specs []ConstraintSpec, rc *ResolveContext) {
	for _, s := range specs {
		s.Resolve(rc)
	}
}

// groupNodes returns the deduplicated union of the mesh nodes bound to the given face refs.
func groupNodes(groups *FaceGroups, faces []string) []int {
	var nodes []int
	for _, key := range faces {
		nodes = append(nodes, groups.Nodes[key]...)
	}
	return dedupeInts(nodes)
}

// groupElemFaces returns the union of the element-faces bound to the given face refs.
func groupElemFaces(groups *FaceGroups, faces []string) []ElemFace {
	var out []ElemFace
	for _, key := range faces {
		out = append(out, groups.ElemFaces[key]...)
	}
	return out
}
