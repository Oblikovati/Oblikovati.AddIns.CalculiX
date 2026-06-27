// SPDX-License-Identifier: GPL-2.0-only

package ccx

// writeStepBegin opens the analysis step with the procedure card for the analysis type:
// *STATIC for stress, *FREQUENCY for modal, *BUCKLE for buckling (the latter two take an
// eigenmode/factor count), *HEAT TRANSFER for heat/electrostatic, and
// *COUPLED TEMPERATURE-DISPLACEMENT for a coupled thermomechanical study (steady-state, or
// transient with a "tinc, tper" time line). Unsupported types fall back to a static solve.
func writeStepBegin(d *deckBuf, m *AnalysisModel) {
	d.line("*STEP")
	switch m.Analysis {
	case AnalysisFrequency:
		d.line("*FREQUENCY")
		d.line("%d", m.EigenmodeCount)
	case AnalysisBuckling:
		d.line("*BUCKLE")
		d.line("%d", m.EigenmodeCount)
	case AnalysisHeatTransfer, AnalysisElectromagnetic:
		d.line("*HEAT TRANSFER, STEADY STATE")
	case AnalysisCoupledThermal:
		writeCoupledProcedure(d, m.Transient)
	default:
		writeStaticProcedure(d, m)
	}
}

// writeStaticProcedure emits the *STATIC card, adding a time-increment data line when the step
// is nonlinear — either contact or an elastic-plastic material. CalculiX then ramps the load
// over increments (tinc, tper) so the Newton iteration converges through contact closure or
// past yield, rather than applying it in one shot; a purely linear study omits it (one
// increment, as before).
func writeStaticProcedure(d *deckBuf, m *AnalysisModel) {
	d.line("*STATIC")
	if m.hasContact() || m.hasPlasticity() {
		d.line("0.1, 1.0")
	}
}

// writeCoupledProcedure emits the coupled temperature-displacement procedure card: steady
// state, or transient with the time-increment data line when a TransientStep is set.
func writeCoupledProcedure(d *deckBuf, ts *TransientStep) {
	if ts == nil {
		d.line("*COUPLED TEMPERATURE-DISPLACEMENT, STEADY STATE")
		return
	}
	d.line("*COUPLED TEMPERATURE-DISPLACEMENT")
	d.line("%.10g, %.10g", ts.IncrementS, ts.TotalS)
}

// writeStepOutput requests the result fields the add-in reads back: nodal temperature (NT)
// for the field-only analyses, nodal displacement (U) plus element stress (S) for the
// mechanical analyses, and all three for a coupled thermomechanical study (which produces a
// temperature field and a thermal-stress field together).
func writeStepOutput(d *deckBuf, a AnalysisType) {
	if a == AnalysisHeatTransfer || a == AnalysisElectromagnetic {
		// NT is the nodal DOF-11 field: temperature for heat, electric potential for the
		// electrostatic analogy.
		d.line("*NODE FILE")
		d.line("NT")
		return
	}
	d.line("*NODE FILE")
	if a == AnalysisCoupledThermal {
		d.line("U, NT")
	} else {
		d.line("U")
	}
	if a != AnalysisFrequency && a != AnalysisBuckling {
		d.line("*EL FILE")
		d.line("S")
	}
}

// writeStepEnd closes the analysis step.
func writeStepEnd(d *deckBuf) { d.line("*END STEP") }
