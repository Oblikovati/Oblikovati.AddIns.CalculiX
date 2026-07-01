// SPDX-License-Identifier: GPL-2.0-only

package ccx

import (
	"oblikovati.org/api/types"
	"oblikovati.org/api/wire"
)

// ccxRibbonTab is the CalculiX add-in's dedicated document ribbon tab.
const ccxRibbonTab = "FEA"

// ccxRibbonSpot places one command on a panel of the FEA tab, with its button style. Buttons are
// text-only for now (the add-in ships no glyph assets yet); an icon pass is a later follow-up.
type ccxRibbonSpot struct {
	panel string
	style types.ButtonStyle
}

// ccxRibbonSpots places every CalculiX command on a panel of the FEA tab. Kept exhaustive so a
// command can never land on an unnamed panel — guarded by TestEveryCommandHasARibbonSpot.
var ccxRibbonSpots = map[string]ccxRibbonSpot{
	RunStudyCommandID:         {"Solve", types.LargeIconButton},
	AddConstraintCommandID:    {"Constraints", types.LargeIconButton},
	ClearConstraintsCommandID: {"Constraints", types.SmallIconButton},
	ShowPanelCommandID:        {"Windows", types.SmallIconButton},
	ShowTreeCommandID:         {"Windows", types.SmallIconButton},
}

// commandArgs builds the host command-registration args, placing the command on its FEA-tab panel.
func commandArgs(id, name, tip string) wire.CreateCommandArgs {
	spot := ccxRibbonSpots[id]
	return wire.CreateCommandArgs{
		ID: id, DisplayName: name, Tooltip: tip,
		Ribbon: types.PartRibbon, Tab: ccxRibbonTab, Category: spot.panel, ButtonStyle: spot.style,
	}
}
