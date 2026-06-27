// SPDX-License-Identifier: GPL-2.0-only

package ccx

// FixedSpec clamps a selected face fully (all three translations) — the rigid support.
type FixedSpec struct {
	Name  string
	Faces []string
}

func (FixedSpec) Kind() ConstraintKind { return KindFixed }

func (s FixedSpec) Resolve(rc *ResolveContext) {
	rc.Model.Fixed = append(rc.Model.Fixed, FixedConstraint{
		Name: s.Name, Nodes: groupNodes(rc.Groups, s.Faces), DOFLow: 1, DOFHigh: 3,
	})
}

// ElasticSupportSpec rests a selected face on a grounded spring foundation (see SpringSupport).
type ElasticSupportSpec struct {
	Name           string
	Faces          []string
	StiffnessTotal float64
}

func (ElasticSupportSpec) Kind() ConstraintKind { return KindElasticSupport }

func (s ElasticSupportSpec) Resolve(rc *ResolveContext) {
	rc.Model.Springs = append(rc.Model.Springs, SpringSupport{
		Name:           s.Name,
		Nodes:          groupNodes(rc.Groups, s.Faces),
		StiffnessTotal: s.StiffnessTotal,
		FirstElem:      maxElementID(rc.Mesh) + 1,
	})
}
