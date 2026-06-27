// SPDX-License-Identifier: GPL-2.0-only

package ccx

// writeStepBegin opens the analysis step with the procedure card for the analysis type.
// The static slice writes *STATIC; the other procedures (*FREQUENCY/*BUCKLE/coupled
// temperature-displacement/heat transfer) are added with their parameter lines in M4,
// keyed on the same AnalysisType.
func writeStepBegin(d *deckBuf, a AnalysisType) {
	d.line("*STEP")
	switch a {
	case AnalysisStatic:
		d.line("*STATIC")
	default:
		// Until the other procedures land, fall back to a static solve rather than
		// emitting an unsupported card the solver would reject.
		d.line("*STATIC")
	}
}

// writeStepOutput requests the result fields the add-in reads back: nodal displacement
// (.frd U) and element stress (.frd S) for the von Mises field.
func writeStepOutput(d *deckBuf) {
	d.line("*NODE FILE")
	d.line("U")
	d.line("*EL FILE")
	d.line("S")
}

// writeStepEnd closes the analysis step.
func writeStepEnd(d *deckBuf) { d.line("*END STEP") }
