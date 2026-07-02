// SPDX-License-Identifier: GPL-2.0-only

package ccx

import (
	"strings"
	"testing"
)

// ccxGlyphKeys is every glyph the add-in references — the 5 ribbon commands plus the Analysis-tree
// node types. Kept beside the icon assets so a renamed/removed asset fails the resolution test.
var ccxGlyphKeys = []string{
	"solve", "constraint", "clearconstraints", "panel", "tree",
	"analysis", "solver", "mesh", "material", "result",
}

func TestEveryReferencedGlyphResolves(t *testing.T) {
	for _, key := range ccxGlyphKeys {
		if iconSVG(key) == "" {
			t.Errorf("glyph %q does not resolve (icons/%s.svg missing?)", key, key)
		}
	}
}

func TestMissingGlyphReturnsEmpty(t *testing.T) {
	if got := iconSVG("no-such-glyph"); got != "" {
		t.Errorf("unknown glyph should resolve to \"\", got %q", got)
	}
}

// TestGlyphsFollowSentinelConvention checks each asset is a 24×24 themed glyph carrying the sentinel
// paints the host recolours (green fill tile, red accent); malformed markup is rejected by the host.
func TestGlyphsFollowSentinelConvention(t *testing.T) {
	for _, key := range ccxGlyphKeys {
		svg := iconSVG(key)
		for _, want := range []string{`viewBox="0 0 24 24"`, `#00ff00`, `#ff0000`} {
			if !strings.Contains(svg, want) {
				t.Errorf("glyph %q missing %q", key, want)
			}
		}
	}
}
