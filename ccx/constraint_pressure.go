// SPDX-License-Identifier: GPL-2.0-only

package ccx

// pressureWriter emits a *DLOAD with one `element, Pn, pressure` card per element-face,
// applying a uniform pressure normal to the loaded surface.
type pressureWriter struct{ c *PressureLoad }

func (pressureWriter) WriteSets(*deckBuf) {} // pressure is addressed per element-face; no set needed

func (p pressureWriter) WriteStep(d *deckBuf) {
	perFace := len(p.c.PerFaceMPa) == len(p.c.Faces)
	if len(p.c.Faces) == 0 || (!perFace && p.c.MPa == 0) {
		return
	}
	d.line("*DLOAD")
	for i, ef := range p.c.Faces {
		mpa := p.c.MPa
		if perFace {
			mpa = p.c.PerFaceMPa[i]
		}
		d.line("%d, P%d, %.10g", ef.Elem, ef.Face, mpa)
	}
}
