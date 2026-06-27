// SPDX-License-Identifier: GPL-2.0-only

package ccx

// modelUnitMM is the host length unit expressed in millimetres: the kernel length unit
// is the centimetre (1 model unit = 10 mm, see ADR-0042 / units of measure #146). The
// CalculiX deck is written in mm / N / MPa, so host coordinates are scaled by this on the
// way in and results are scaled back by its inverse on the way out.
const modelUnitMM = 10.0

// gpaToMPa converts Young's modulus from the catalogue's GPa to CalculiX MPa.
//
// NOTE: v1 takes the material from the study panel (settings.material()). Resolving the
// body's *assigned* material from the host (with its g/cm^3 density -> t/mm^3) lands when
// body-loads need density (gravity, M2) and the assigned-material lookup is wired.
const gpaToMPa = 1000.0
