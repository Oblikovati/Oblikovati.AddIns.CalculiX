// SPDX-License-Identifier: GPL-2.0-only

package ccx

import (
	"math"
	"testing"
)

// approxEq compares two stresses to a small absolute tolerance.
func approxEq(a, b, tol float64) bool { return math.Abs(a-b) <= tol }

func TestPrincipalStressesDiagonal(t *testing.T) {
	// A diagonal tensor: the principal stresses are the diagonal entries, sorted.
	s1, s2, s3 := principalStresses([6]float64{30, -10, 5, 0, 0, 0})
	if !approxEq(s1, 30, 1e-9) || !approxEq(s2, 5, 1e-9) || !approxEq(s3, -10, 1e-9) {
		t.Errorf("diagonal principals = (%v, %v, %v), want (30, 5, -10)", s1, s2, s3)
	}
}

func TestPrincipalStressesPureShear(t *testing.T) {
	// Pure shear sxy=tau has principal stresses +tau, 0, -tau.
	const tau = 12.0
	s1, s2, s3 := principalStresses([6]float64{0, 0, 0, tau, 0, 0})
	if !approxEq(s1, tau, 1e-6) || !approxEq(s2, 0, 1e-6) || !approxEq(s3, -tau, 1e-6) {
		t.Errorf("pure-shear principals = (%v, %v, %v), want (%v, 0, %v)", s1, s2, s3, tau, -tau)
	}
}

func TestPrincipalStressesInvariants(t *testing.T) {
	// The principal stresses must reproduce the trace and von Mises invariants of a
	// general tensor — an analysis-free cross-check of the eigenvalue solve.
	s := [6]float64{40, 20, -10, 15, -5, 8}
	s1, s2, s3 := principalStresses(s)
	if !approxEq(s1+s2+s3, s[0]+s[1]+s[2], 1e-6) {
		t.Errorf("trace not preserved: %v vs %v", s1+s2+s3, s[0]+s[1]+s[2])
	}
	vmTensor := vonMises(s)
	vmPrincipal := vonMises([6]float64{s1, s2, s3, 0, 0, 0})
	if !approxEq(vmTensor, vmPrincipal, 1e-6) {
		t.Errorf("von Mises mismatch: tensor %v vs principal %v", vmTensor, vmPrincipal)
	}
	if !(s1 >= s2 && s2 >= s3) {
		t.Errorf("principals not sorted descending: (%v, %v, %v)", s1, s2, s3)
	}
}

func TestComputeResultFieldSelectsKind(t *testing.T) {
	res := &ResultField{
		Disp:   map[int][3]float64{1: {3, 4, 0}},             // magnitude 5
		Stress: map[int][6]float64{1: {30, -10, 5, 0, 0, 0}}, // s1=30, s3=-10
	}
	cases := []struct {
		kind  ResultFieldKind
		value float64
		unit  string
	}{
		{ResultDisplacement, 5, "mm"},
		{ResultMaxPrincipal, 30, "MPa"},
		{ResultMinPrincipal, -10, "MPa"},
	}
	for _, c := range cases {
		field, _, unit := computeResultField(res, c.kind)
		if !approxEq(field[1], c.value, 1e-6) || unit != c.unit {
			t.Errorf("%s: got (%v, %q), want (%v, %q)", c.kind, field[1], unit, c.value, c.unit)
		}
	}
}
