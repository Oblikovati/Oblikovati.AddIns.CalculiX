// SPDX-License-Identifier: GPL-2.0-only

package ccx

// writeStepBegin opens the analysis step with the procedure card for the analysis type:
// *STATIC for stress, *FREQUENCY for modal, *BUCKLE for buckling (the latter two take an
// eigenmode/factor count). Unsupported types fall back to a static solve.
func writeStepBegin(d *deckBuf, a AnalysisType, eigenCount int) {
	d.line("*STEP")
	switch a {
	case AnalysisFrequency:
		d.line("*FREQUENCY")
		d.line("%d", eigenCount)
	case AnalysisBuckling:
		d.line("*BUCKLE")
		d.line("%d", eigenCount)
	default:
		d.line("*STATIC")
	}
}

// writeStepOutput requests the result fields the add-in reads back: nodal displacement
// (mode shapes / deflection) always, and element stress only for a static stress study
// (modal/buckling report eigenvalues, not a physical stress field).
func writeStepOutput(d *deckBuf, a AnalysisType) {
	d.line("*NODE FILE")
	d.line("U")
	if a != AnalysisFrequency && a != AnalysisBuckling {
		d.line("*EL FILE")
		d.line("S")
	}
}

// writeStepEnd closes the analysis step.
func writeStepEnd(d *deckBuf) { d.line("*END STEP") }
