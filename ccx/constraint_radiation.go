// SPDX-License-Identifier: GPL-2.0-only

package ccx

// stefanBoltzmannConsistent is the Stefan-Boltzmann constant in the deck's consistent units
// (mm, t, s, K → power mW): 5.67e-8 W/(m²·K⁴) = 5.67e-11 mW/(mm²·K⁴).
const stefanBoltzmannConsistent = 5.67e-11

// radiateWriter emits a *RADIATE card with one `element, Rn, ambient, emissivity` line per
// element-face. CalculiX applies q = ε·σ·(T⁴ − ambient⁴), so it relies on the *PHYSICAL
// CONSTANTS card supplying σ and the absolute-zero offset (writePhysicalConstants).
type radiateWriter struct{ c *RadiationBC }

func (radiateWriter) WriteSets(*deckBuf) {} // addressed per element-face; no set needed

func (w radiateWriter) WriteStep(d *deckBuf) {
	if len(w.c.Faces) == 0 || w.c.Emissivity == 0 {
		return
	}
	d.line("*RADIATE")
	for _, ef := range w.c.Faces {
		d.line("%d, R%d, %.10g, %.10g", ef.Elem, ef.Face, w.c.AmbientK, w.c.Emissivity)
	}
}

// writePhysicalConstants emits the *PHYSICAL CONSTANTS card radiation needs: the absolute-zero
// temperature (0, since the add-in works in kelvin) and the Stefan-Boltzmann constant. Written
// in the model data, before the *STEP, only when the model radiates.
func writePhysicalConstants(d *deckBuf, m *AnalysisModel) {
	if len(m.Radiations) == 0 {
		return
	}
	d.line("*PHYSICAL CONSTANTS, ABSOLUTE ZERO=0, STEFAN BOLTZMANN=%.10g", stefanBoltzmannConsistent)
}
