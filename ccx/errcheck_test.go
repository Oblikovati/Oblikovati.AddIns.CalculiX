// SPDX-License-Identifier: GPL-2.0-only

package ccx

import (
	"strings"
	"testing"
)

func TestScrapeCcxErrorsCleanOutput(t *testing.T) {
	clean := " Job finished\n Total CalculiX Time: 0.0016\n"
	if got := scrapeCcxErrors(clean); got != "" {
		t.Errorf("clean output scraped an error: %q", got)
	}
}

func TestScrapeCcxErrorsNoMaterial(t *testing.T) {
	// The verbatim message our vendored ccx 2.22 prints for a section with no material.
	out := "   materials:            0\n *ERROR reading *SOLID SECTION: nonexistent material\n"
	got := scrapeCcxErrors(out)
	if !strings.Contains(got, "nonexistent material") {
		t.Errorf("scrape = %q, want the *ERROR line", got)
	}
	if !strings.Contains(got, "Young's modulus") {
		t.Errorf("scrape = %q, want the material hint", got)
	}
}

func TestScrapeCcxErrorsNonpositiveJacobianListsElements(t *testing.T) {
	out := "*ERROR in e_c3d: nonpositive jacobian determinant in element 42\n" +
		"*ERROR in e_c3d: nonpositive jacobian determinant in element 7\n" +
		"*ERROR in e_c3d: nonpositive jacobian determinant in element 42\n"
	got := scrapeCcxErrors(out)
	if !strings.Contains(got, "[7 42]") {
		t.Errorf("scrape = %q, want the sorted, de-duplicated element list [7 42]", got)
	}
}
