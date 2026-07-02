// SPDX-License-Identifier: GPL-2.0-only

package ccx

import (
	"oblikovati.org/api/types"
	"oblikovati.org/api/wire"
)

// ccxRibbonTab is the CalculiX add-in's dedicated document ribbon tab.
const ccxRibbonTab = "FEA"

// ccxRibbonSpot places one command on a panel of the FEA tab, with its inline glyph (an embedded
// icons/<icon>.svg — see iconSVG) and button style.
type ccxRibbonSpot struct {
	panel string
	icon  string
	style types.ButtonStyle
}

// ccxRibbonSpots places every CalculiX command on a panel of the FEA tab, each carrying its own
// Oblikovati-style glyph. Kept exhaustive so a command can never land on an unnamed panel or
// without a glyph — guarded by TestEveryCommandHasARibbonSpot / TestEveryCommandIsAnIconButton.
var ccxRibbonSpots = map[string]ccxRibbonSpot{
	RunStudyCommandID:         {"Solve", "solve", types.LargeIconButton},
	AddConstraintCommandID:    {"Constraints", "constraint", types.LargeIconButton},
	ClearConstraintsCommandID: {"Constraints", "clearconstraints", types.SmallIconButton},
	ShowPanelCommandID:        {"Windows", "panel", types.SmallIconButton},
	ShowTreeCommandID:         {"Windows", "tree", types.SmallIconButton},
}

// commandArgs builds the host command-registration args, placing the command on its FEA-tab panel
// with its bundled glyph.
func commandArgs(id, name, tip string) wire.CreateCommandArgs {
	spot := ccxRibbonSpots[id]
	return wire.CreateCommandArgs{
		ID: id, DisplayName: name, Tooltip: tip,
		Ribbon: types.PartRibbon, Tab: ccxRibbonTab, Category: spot.panel,
		IconSVG: iconSVG(spot.icon), ButtonStyle: spot.style,
	}
}
