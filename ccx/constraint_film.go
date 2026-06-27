// SPDX-License-Identifier: GPL-2.0-only

package ccx

// filmWriter emits a *FILM convective heat exchange with one `element, Fn, sink_temp,
// film_coeff` card per element-face. CalculiX applies q = h·(T − T_sink) on each face, so the
// face number Fn is the same as the surface number Sn used by pressure/flux loads.
type filmWriter struct{ c *FilmBC }

func (filmWriter) WriteSets(*deckBuf) {} // addressed per element-face; no set needed

func (w filmWriter) WriteStep(d *deckBuf) {
	if len(w.c.Faces) == 0 || w.c.Coeff == 0 {
		return
	}
	d.line("*FILM")
	for _, ef := range w.c.Faces {
		d.line("%d, F%d, %.10g, %.10g", ef.Elem, ef.Face, w.c.SinkTempK, w.c.Coeff)
	}
}
