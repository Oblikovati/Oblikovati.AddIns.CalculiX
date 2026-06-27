// SPDX-License-Identifier: GPL-2.0-only

package ccx

// modelUnitMM is the host length unit expressed in millimetres: the kernel length unit
// is the centimetre (1 model unit = 10 mm, see ADR-0042 / units of measure #146). The
// CalculiX deck is written in mm / N / MPa, so host coordinates are scaled by this on the
// way in and results are scaled back by its inverse on the way out.
const modelUnitMM = 10.0

// gpaToMPa converts Young's modulus from GPa to CalculiX MPa; gCm3ToTonneMM3 converts
// density from g/cm^3 to the CalculiX t/mm^3 convention (consumed by gravity body loads).
//
// NOTE: v1 takes the material from the study panel (settings.material()). Resolving the
// body's *assigned* material from the host is a follow-up.
const (
	gpaToMPa       = 1000.0
	gCm3ToTonneMM3 = 1e-9
)
