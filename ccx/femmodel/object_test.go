// SPDX-License-Identifier: GPL-2.0-only

package femmodel

import "testing"

func TestCategoryString(t *testing.T) {
	cases := map[Category]string{
		CategorySolver: "Solver", CategoryMesh: "Mesh", CategoryMaterial: "Material",
		CategoryConstraint: "Constraint", CategoryResult: "Result",
	}
	for c, want := range cases {
		if got := c.String(); got != want {
			t.Fatalf("Category(%d).String() = %q, want %q", c, got, want)
		}
	}
}
