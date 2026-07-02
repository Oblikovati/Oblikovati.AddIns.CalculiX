// SPDX-License-Identifier: GPL-2.0-only

package ccx

import "embed"

// iconFS bundles the CalculiX add-in's ribbon and Analysis-tree glyphs. Each file is
// "icons/<key>.svg" in the Oblikovati glyph convention: a 24×24 viewBox with the sentinel paints
// the host recolours per theme — a green (#00ff00) fill tile, a black (#000) outline, and red
// (#ff0000) action accents. The add-in ships its own glyphs via the command/browser IconSVG field
// (api Oblikovati#671) so its buttons and tree nodes are not limited to the host's bundled icons.
//
//go:embed icons/*.svg
var iconFS embed.FS

// iconSVG returns the inline SVG markup for a CalculiX glyph, or "" when no such asset is bundled
// (the host then falls back to a text button / an unglyphed node). Callers pass a bare key such as
// "solver" or "mesh"; the ".svg" extension and "icons/" prefix are supplied here.
func iconSVG(key string) string {
	b, err := iconFS.ReadFile("icons/" + key + ".svg")
	if err != nil {
		return ""
	}
	return string(b)
}
