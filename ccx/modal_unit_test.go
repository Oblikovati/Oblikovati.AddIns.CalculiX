// SPDX-License-Identifier: GPL-2.0-only

package ccx

import (
	"fmt"
	"strings"
	"testing"
)

func TestStudyResultSummaryStatic(t *testing.T) {
	r := &StudyResult{ElementCount: 100, PeakVonMisesMPa: 5.9, MaxDisplacement: 0.0064}
	got := r.Summary()
	if !strings.Contains(got, "100 elements") || !strings.Contains(got, "von Mises") {
		t.Errorf("static summary = %q", got)
	}
}

func TestStudyResultSummaryModalTruncates(t *testing.T) {
	r := &StudyResult{
		ElementCount: 50,
		Modes:        []float64{830.7, 831.0, 1200, 2000, 3000}, // 5 modes
		ModeKind:     "natural frequencies",
		ModeUnit:     "Hz",
	}
	got := r.Summary()
	if !strings.Contains(got, "natural frequencies") || !strings.Contains(got, "830.7 Hz") {
		t.Errorf("modal summary = %q", got)
	}
	if !strings.Contains(got, "…") {
		t.Errorf("modal summary should truncate >4 modes with an ellipsis: %q", got)
	}
}

// frdDispLine formats a fixed-width .frd displacement record (" -1" + I10 id + 3×E12.5).
func frdDispLine(id int, x, y, z float64) string {
	return fmt.Sprintf(" -1%10d%12.5E%12.5E%12.5E", id, x, y, z)
}

func TestParseFirstModeDispKeepsFirstBlock(t *testing.T) {
	frd := strings.Join([]string{
		" -4  DISP        4    1",
		frdDispLine(1, 1.0, 0, 0),
		frdDispLine(2, 2.0, 0, 0),
		" -3",
		" -4  DISP        4    2", // a second mode — must be ignored
		frdDispLine(1, 9.0, 0, 0),
		" -3",
	}, "\n")
	disp, err := parseFirstModeDisp(strings.NewReader(frd))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(disp) != 2 {
		t.Fatalf("got %d nodes, want 2 (first block only)", len(disp))
	}
	if disp[1][0] != 1.0 {
		t.Errorf("node 1 x = %v, want 1.0 (mode 1, not mode 2's 9.0)", disp[1][0])
	}
}

func TestApplyMaterialAndLoadEdits(t *testing.T) {
	e := NewEngine(&recordingHost{})
	e.applyPanelEdit("eigenmodes", "10")
	e.applyPanelEdit("young", "70")
	e.applyPanelEdit("poisson", "0.33")
	e.applyPanelEdit("density", "2.7")
	e.applyPanelEdit("load_type", string(LoadPressure))
	e.applyPanelEdit("pressure", "2.5")
	e.applyPanelEdit("gravity", "2")
	s := e.settings
	if s.Eigenmodes != 10 || s.YoungGPa != 70 || s.Poisson != 0.33 || s.DensityGCm3 != 2.7 {
		t.Errorf("material/eigen edits not applied: %+v", s)
	}
	if s.LoadType != LoadPressure || s.PressureMPa != 2.5 || s.GravityG != 2 {
		t.Errorf("load edits not applied: %+v", s)
	}
}
