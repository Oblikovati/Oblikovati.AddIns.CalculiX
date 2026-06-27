// SPDX-License-Identifier: GPL-2.0-only

package ccx

import "math"

// vonMises returns the von Mises equivalent stress for a symmetric stress tensor
// (sxx, syy, szz, sxy, syz, szx):
//
//	sigma_vM = sqrt( 0.5*((sxx-syy)^2 + (syy-szz)^2 + (szz-sxx)^2) + 3*(sxy^2+syz^2+szx^2) )
func vonMises(s [6]float64) float64 {
	sxx, syy, szz, sxy, syz, szx := s[0], s[1], s[2], s[3], s[4], s[5]
	dev := (sxx-syy)*(sxx-syy) + (syy-szz)*(syy-szz) + (szz-sxx)*(szz-sxx)
	shear := sxy*sxy + syz*syz + szx*szx
	return math.Sqrt(0.5*dev + 3*shear)
}

// vonMisesField computes per-node von Mises stress from the parsed stress tensors.
func vonMisesField(res *ResultField) map[int]float64 {
	out := make(map[int]float64, len(res.Stress))
	for id, s := range res.Stress {
		out[id] = vonMises(s)
	}
	return out
}

// dispMagnitude returns the displacement magnitude per node.
func dispMagnitude(res *ResultField) map[int]float64 {
	out := make(map[int]float64, len(res.Disp))
	for id, u := range res.Disp {
		out[id] = math.Sqrt(u[0]*u[0] + u[1]*u[1] + u[2]*u[2])
	}
	return out
}

// peak returns the maximum value of a per-node scalar field (0 for an empty field).
func peak(field map[int]float64) float64 {
	max := 0.0
	for _, v := range field {
		if v > max {
			max = v
		}
	}
	return max
}
