// SPDX-License-Identifier: GPL-2.0-only

package ccx

// heatFluxWriter emits a *DFLUX with one `element, Sn, flux` card per element-face,
// applying a uniform surface heat flux to the loaded surface.
type heatFluxWriter struct{ c *HeatFlux }

func (heatFluxWriter) WriteSets(*deckBuf) {} // addressed per element-face; no set needed

func (h heatFluxWriter) WriteStep(d *deckBuf) {
	if len(h.c.Faces) == 0 || h.c.Flux == 0 {
		return
	}
	d.line("*DFLUX")
	for _, ef := range h.c.Faces {
		d.line("%d, S%d, %.10g", ef.Elem, ef.Face, h.c.Flux)
	}
}
