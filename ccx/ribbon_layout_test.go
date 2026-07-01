// SPDX-License-Identifier: GPL-2.0-only

package ccx

import (
	"testing"

	"oblikovati.org/api/types"
)

func TestCommandArgsPlacesOnFEATab(t *testing.T) {
	a := commandArgs(RunStudyCommandID, "Run Study", "tip")
	if a.ID != RunStudyCommandID || a.DisplayName != "Run Study" || a.Tooltip != "tip" {
		t.Fatalf("identity fields wrong: %+v", a)
	}
	if a.Ribbon != types.PartRibbon || a.Tab != "FEA" || a.Category != "Solve" {
		t.Fatalf("placement wrong: ribbon=%q tab=%q cat=%q", a.Ribbon, a.Tab, a.Category)
	}
	if a.ButtonStyle != types.LargeIconButton {
		t.Fatalf("Run Study should be a large button, got %v", a.ButtonStyle)
	}
}

func TestEveryCommandHasARibbonSpot(t *testing.T) {
	for _, id := range []string{
		RunStudyCommandID, AddConstraintCommandID, ClearConstraintsCommandID,
		ShowPanelCommandID, ShowTreeCommandID,
	} {
		if _, ok := ccxRibbonSpots[id]; !ok {
			t.Errorf("command %q has no ribbon spot", id)
		}
	}
}
