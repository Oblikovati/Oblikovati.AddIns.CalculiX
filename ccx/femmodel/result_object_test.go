// SPDX-License-Identifier: GPL-2.0-only

package femmodel

import "testing"

func TestResultObject(t *testing.T) {
	r := newResultObject("result1", "von Mises stress", 0)
	if r.ObjectID() != "result1" || r.Category() != CategoryResult || r.Name() != "Results" {
		t.Fatalf("result identity wrong: %+v", r)
	}
	if r.Field != "von Mises stress" || r.DeformScale != 0 {
		t.Fatalf("result fields wrong: %+v", r)
	}
}
