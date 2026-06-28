// SPDX-License-Identifier: GPL-2.0-only

package ccx

import "math"

// RollerSpec holds a selected face against motion ALONG ITS NORMAL only, leaving it free to slide
// in-plane — a frictionless roller / sliding support. The constrained DOF is derived from the
// face's outward normal at resolve time (the dominant global axis), so the user picks a face and
// the geometry decides the direction. A symmetry plane is the same constraint (see SymmetrySpec).
type RollerSpec struct {
	Name  string
	Faces []string
}

func (RollerSpec) Kind() ConstraintKind { return KindRoller }

func (s RollerSpec) Resolve(rc *ResolveContext) { resolveNormalFix(rc, s.Name, s.Faces) }

// SymmetrySpec fixes the out-of-plane (normal) translation on a symmetry plane — mechanically a
// roller, named distinctly because the user's intent ("this is a plane of symmetry") differs.
type SymmetrySpec struct {
	Name  string
	Faces []string
}

func (SymmetrySpec) Kind() ConstraintKind { return KindSymmetry }

func (s SymmetrySpec) Resolve(rc *ResolveContext) { resolveNormalFix(rc, s.Name, s.Faces) }

// resolveNormalFix appends a FixedConstraint on the single global DOF closest to the selected
// face's outward normal — the shared body of the roller and symmetry constraints.
func resolveNormalFix(rc *ResolveContext, name string, faces []string) {
	dof := dominantAxis(faceNormal(rc.Groups, faces))
	rc.Model.Fixed = append(rc.Model.Fixed, FixedConstraint{
		Name: name, Nodes: groupNodes(rc.Groups, faces), DOFLow: dof, DOFHigh: dof,
	})
}

// faceNormal returns the mean outward unit normal of the selected faces (coplanar in practice).
func faceNormal(groups *FaceGroups, faces []string) [3]float64 {
	var sum [3]float64
	for _, key := range faces {
		n := groups.Normals[key]
		for k := 0; k < 3; k++ {
			sum[k] += n[k]
		}
	}
	return sum
}

// dominantAxis returns the CalculiX translational DOF (1, 2, or 3) of the global axis closest to
// a direction — the axis with the largest absolute component. Defaults to Z for a zero vector.
func dominantAxis(v [3]float64) int {
	dof, best := 3, -1.0
	for i := 0; i < 3; i++ {
		if a := math.Abs(v[i]); a > best {
			best, dof = a, i+1
		}
	}
	return dof
}
