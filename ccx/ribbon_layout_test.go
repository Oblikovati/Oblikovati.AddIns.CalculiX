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
	if a.IconSVG == "" {
		t.Fatal("Run Study should carry an inline glyph")
	}
}

// TestEveryCommandIsAnIconButton guards the icon pass: every ribbon command must resolve a bundled
// glyph, so a command added without an icon asset fails here rather than shipping an unglyphed button.
func TestEveryCommandIsAnIconButton(t *testing.T) {
	for _, c := range ccxCommands {
		if commandArgs(c.id, c.name, c.tip).IconSVG == "" {
			t.Errorf("command %q has no glyph (icons/%s.svg missing?)", c.id, ccxRibbonSpots[c.id].icon)
		}
	}
}

func TestEveryCommandHasARibbonSpot(t *testing.T) {
	// Derived from the registration list, so a command appended to ccxCommands without a
	// ribbon spot fails here rather than silently landing on an unnamed panel.
	for _, c := range ccxCommands {
		if _, ok := ccxRibbonSpots[c.id]; !ok {
			t.Errorf("command %q has no ribbon spot", c.id)
		}
	}
}
