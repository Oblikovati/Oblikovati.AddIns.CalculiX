// SPDX-License-Identifier: GPL-2.0-only

package ccx

import (
	"math"
	"sort"
)

// principalStresses returns the three principal stresses — the eigenvalues of the
// symmetric stress tensor (sxx, syy, szz, sxy, syz, szx) — sorted descending so that
// sigma1 >= sigma2 >= sigma3. It uses the closed-form solution for a symmetric 3x3 matrix
// (Smith 1961): the off-diagonal magnitude decides between the diagonal shortcut and the
// trigonometric eigenvalue formula, with the cosine argument clamped against round-off.
func principalStresses(s [6]float64) (float64, float64, float64) {
	sxx, syy, szz, sxy, syz, szx := s[0], s[1], s[2], s[3], s[4], s[5]
	offDiag := sxy*sxy + syz*syz + szx*szx
	if offDiag == 0 {
		return sortDesc3(sxx, syy, szz)
	}
	q := (sxx + syy + szz) / 3
	p2 := (sxx-q)*(sxx-q) + (syy-q)*(syy-q) + (szz-q)*(szz-q) + 2*offDiag
	p := math.Sqrt(p2 / 6)
	r := math.Max(-1, math.Min(1, deviatoricDet(s, q, p)/2))
	phi := math.Acos(r) / 3
	hi := q + 2*p*math.Cos(phi)
	lo := q + 2*p*math.Cos(phi+2*math.Pi/3)
	mid := 3*q - hi - lo // the trace is invariant: sigma1 + sigma2 + sigma3 = 3q
	return hi, mid, lo
}

// deviatoricDet returns det((A - qI)/p) for the symmetric tensor A, the quantity whose
// half is the cosine of the eigenvalue phase in Smith's algorithm.
func deviatoricDet(s [6]float64, q, p float64) float64 {
	bxx, byy, bzz := (s[0]-q)/p, (s[1]-q)/p, (s[2]-q)/p
	bxy, byz, bzx := s[3]/p, s[4]/p, s[5]/p
	return bxx*(byy*bzz-byz*byz) - bxy*(bxy*bzz-byz*bzx) + bzx*(bxy*byz-byy*bzx)
}

// sortDesc3 returns its three arguments in descending order.
func sortDesc3(a, b, c float64) (float64, float64, float64) {
	v := []float64{a, b, c}
	sort.Sort(sort.Reverse(sort.Float64Slice(v)))
	return v[0], v[1], v[2]
}

// maxPrincipalField / minPrincipalField compute the per-node extreme principal stress.
func maxPrincipalField(res *ResultField) map[int]float64 {
	out := make(map[int]float64, len(res.Stress))
	for id, s := range res.Stress {
		s1, _, _ := principalStresses(s)
		out[id] = s1
	}
	return out
}

func minPrincipalField(res *ResultField) map[int]float64 {
	out := make(map[int]float64, len(res.Stress))
	for id, s := range res.Stress {
		_, _, s3 := principalStresses(s)
		out[id] = s3
	}
	return out
}
