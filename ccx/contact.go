// SPDX-License-Identifier: GPL-2.0-only

package ccx

import "fmt"

// ContactPair models unilateral (separation-capable) contact between two coincident interface
// surfaces of separately-meshed bodies: unlike a *TIE — which glues the faces so they never
// part — a contact pair transmits compression and friction while still being free to open up
// in tension. It is written as a penalty *SURFACE INTERACTION (linear pressure-overclosure)
// with optional Coulomb *FRICTION, and a *CONTACT PAIR linking the slave to the master surface.
type ContactPair struct {
	Name       string
	Slave      []ElemFace
	Master     []ElemFace
	Stiffness  float64 // linear pressure-overclosure slope K (MPa/mm); penalty contact stiffness
	FrictionMu float64 // Coulomb friction coefficient μ; 0 = frictionless sliding
}

// contactStiffnessFactor scales the stiffest material's Young's modulus into a penalty
// contact stiffness. A value well above the modulus keeps interpenetration negligible while
// staying low enough for the Newton iterations to converge (the CalculiX guidance for a
// linear pressure-overclosure penalty).
const contactStiffnessFactor = 50.0

// detectContacts turns each touching interface into a unilateral contact pair with the given
// friction coefficient and penalty stiffness, so two stacked bodies press on each other and
// transmit load through the interface while remaining free to separate.
func detectContacts(mesh *TetMesh, frictionMu, stiffness float64) []ContactPair {
	matches := matchInterfaces(mesh)
	pairs := make([]ContactPair, 0, len(matches))
	for i, m := range matches {
		pairs = append(pairs, ContactPair{
			Name:       fmt.Sprintf("CONTACT%d", i),
			Slave:      m.Slave,
			Master:     m.Master,
			Stiffness:  stiffness,
			FrictionMu: frictionMu,
		})
	}
	return pairs
}

// writeContacts emits, for each pair, the element-face surfaces, the surface interaction with
// its penalty behaviour and optional friction, and the *CONTACT PAIR linking them. These are
// model-level cards, written before the *STEP (like *TIE).
func writeContacts(d *deckBuf, pairs []ContactPair) {
	for _, p := range pairs {
		writeFaceSurface(d, p.Name+"_S", p.Slave)
		writeFaceSurface(d, p.Name+"_M", p.Master)
		writeSurfaceInteraction(d, p)
		d.line("*CONTACT PAIR, INTERACTION=%s_SI, TYPE=SURFACE TO SURFACE", p.Name)
		d.line("%s_S, %s_M", p.Name, p.Name)
	}
}

// writeSurfaceInteraction emits the named surface interaction: a linear pressure-overclosure
// penalty plus, when μ > 0, a Coulomb friction law whose stick stiffness matches the normal
// penalty (the CalculiX convention for a well-conditioned tangential return).
func writeSurfaceInteraction(d *deckBuf, p ContactPair) {
	d.line("*SURFACE INTERACTION, NAME=%s_SI", p.Name)
	d.line("*SURFACE BEHAVIOR, PRESSURE-OVERCLOSURE=LINEAR")
	d.line("%.10g", p.Stiffness)
	if p.FrictionMu > 0 {
		d.line("*FRICTION")
		d.line("%.10g, %.10g", p.FrictionMu, p.Stiffness)
	}
}
