// SPDX-License-Identifier: GPL-2.0-only

package ccx

// pressureWriter emits a *DLOAD with one `element, Pn, pressure` card per element-face,
// applying a uniform pressure normal to the loaded surface.
type pressureWriter struct{ c *PressureLoad }

func (pressureWriter) WriteSets(*deckBuf) {} // pressure is addressed per element-face; no set needed

func (p pressureWriter) WriteStep(d *deckBuf) {
	if len(p.c.Faces) == 0 || p.c.MPa == 0 {
		return
	}
	d.line("*DLOAD")
	for _, ef := range p.c.Faces {
		d.line("%d, P%d, %.10g", ef.Elem, ef.Face, p.c.MPa)
	}
}
