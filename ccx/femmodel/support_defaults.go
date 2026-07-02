// SPDX-License-Identifier: GPL-2.0-only

package femmodel

// SupportDefaults holds the mechanical support parameters synthesized at solve time
// (the first selected face is the support). It is not a browser-tree node; it is a
// study-wide template, mirroring LoadDefaults. SupportType is a neutral string here —
// the ccx layer maps it to its SupportType display enum.
type SupportDefaults struct {
	SupportType   string  // "fixed" clamps; "elastic (spring)" rests on a grounded *SPRING
	SpringStiffMM float64 // total elastic-support stiffness (N/mm) for the elastic type
}
