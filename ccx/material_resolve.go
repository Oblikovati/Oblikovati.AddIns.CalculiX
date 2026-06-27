// SPDX-License-Identifier: GPL-2.0-only

package ccx

import (
	"strings"

	"oblikovati.org/api/types"
	"oblikovati.org/api/wire"
)

// bodyMaterial resolves the CalculiX material for one body: the host's assigned material
// (BodyInfo.MaterialID, api v0.92.0) converted to deck units, or the panel fallback material
// when the body carries no assignment. A unique CalculiX material name is derived so a part
// of mixed materials writes one *MATERIAL per distinct material.
func (e *Engine) bodyMaterial(b wire.BodyInfo, fallback MaterialProps) (MaterialProps, error) {
	if b.MaterialID == "" {
		return fallback, nil
	}
	info, err := e.api.Materials().Get(b.MaterialID)
	if err != nil {
		return MaterialProps{}, err
	}
	return materialPropsFromInfo(info), nil
}

// materialPropsFromInfo converts a host material to the CalculiX unit convention (mm, t, s →
// N, MPa). Young's modulus GPa→MPa and density g/cm³→t/mm³ match settings.material(); thermal
// conductivity in W/(m·K) is numerically the consistent-unit value (mW/(mm·K)); electrical
// conductivity is the reciprocal of resistivity (Ω·m → S/m). The name is sanitised for the
// deck.
func materialPropsFromInfo(info wire.MaterialInfo) MaterialProps {
	return MaterialProps{
		Name:            sanitizeMatName(materialLabel(info)),
		YoungMPa:        info.Mechanical.YoungsModulus * gpaToMPa,
		Poisson:         info.Mechanical.PoissonsRatio,
		DensityTonneMM3: info.Density * gCm3ToTonneMM3,
		ExpansionPerK:   info.Thermal.ExpansionCoeff,
		Conductivity:    info.Thermal.Conductivity,
		ElectricalSigma: reciprocalOrZero(info.Electrical.Resistivity),
		Ortho:           orthoFromInfo(info),
	}
}

// orthoFromInfo returns the orthotropic elastic constants (E, G converted GPa→MPa) when the
// host material declares a non-isotropic elastic symmetry, else nil so the isotropic Young/
// Poisson path is used.
func orthoFromInfo(info wire.MaterialInfo) *OrthoElastic {
	if !types.IsotropyClass(info.IsotropyClass).Anisotropic() {
		return nil
	}
	a := info.Anisotropic
	return &OrthoElastic{
		E1MPa: a.E1 * gpaToMPa, E2MPa: a.E2 * gpaToMPa, E3MPa: a.E3 * gpaToMPa,
		Nu12: a.Nu12, Nu13: a.Nu13, Nu23: a.Nu23,
		G12MPa: a.G12 * gpaToMPa, G13MPa: a.G13 * gpaToMPa, G23MPa: a.G23 * gpaToMPa,
	}
}

// materialLabel prefers the display name, falling back to the id, so the deck's material
// names read like the model browser's.
func materialLabel(info wire.MaterialInfo) string {
	if info.DisplayName != "" {
		return info.DisplayName
	}
	return info.ID
}

// reciprocalOrZero returns 1/x, or 0 when x is non-positive (an unset resistivity yields no
// electrical conductivity rather than a divide-by-zero).
func reciprocalOrZero(x float64) float64 {
	if x <= 0 {
		return 0
	}
	return 1 / x
}

// matNameMaxLen is CalculiX's material-name length limit.
const matNameMaxLen = 80

// sanitizeMatName makes a CalculiX-safe *MATERIAL name: keep letters/digits/underscore,
// replace every other rune (spaces, punctuation) with an underscore, and bound the length.
// CalculiX rejects names with spaces, so an unsanitised "Shop Steel" would break the deck.
func sanitizeMatName(name string) string {
	var b strings.Builder
	for _, r := range name {
		switch {
		case r >= 'A' && r <= 'Z', r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '_':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	out := b.String()
	if out == "" {
		return "MATERIAL"
	}
	if len(out) > matNameMaxLen {
		out = out[:matNameMaxLen]
	}
	return out
}
