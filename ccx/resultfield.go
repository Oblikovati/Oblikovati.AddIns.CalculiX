// SPDX-License-Identifier: GPL-2.0-only

package ccx

// computeResultField returns the per-node scalar field to colour a stress result by, with a
// human-readable label and unit, for the requested field kind.
func computeResultField(res *ResultField, kind ResultFieldKind) (map[int]float64, string, string) {
	switch kind {
	case ResultDisplacement:
		return dispMagnitude(res), "displacement", "mm"
	case ResultMaxPrincipal:
		return maxPrincipalField(res), "max principal stress", "MPa"
	case ResultMinPrincipal:
		return minPrincipalField(res), "min principal stress", "MPa"
	default:
		return vonMisesField(res), "von Mises stress", "MPa"
	}
}
