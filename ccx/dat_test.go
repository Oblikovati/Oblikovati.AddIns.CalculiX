// SPDX-License-Identifier: GPL-2.0-only

package ccx

import (
	"math"
	"strings"
	"testing"
)

func TestParseEigenFrequencies(t *testing.T) {
	// The verbatim *FREQUENCY .dat table layout from ccx 2.22.
	dat := `     E I G E N V A L U E   O U T P U T

 MODE NO    EIGENVALUE                       FREQUENCY
                                     REAL PART            IMAGINARY PART
                           (RAD/TIME)      (CYCLES/TIME     (RAD/TIME)

      1   0.1670793E+12   0.4087534E+06   0.6505512E+05   0.0000000E+00
      2   0.3067186E+12   0.5538218E+06   0.8814348E+05   0.0000000E+00

     P A R T I C I P A T I O N   F A C T O R S
`
	freqs, err := parseEigenFrequencies(strings.NewReader(dat))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(freqs) != 2 {
		t.Fatalf("got %d frequencies, want 2", len(freqs))
	}
	if math.Abs(freqs[0]-65055.12) > 1 {
		t.Errorf("freq[0] = %v Hz, want ~65055 (cycles/time column)", freqs[0])
	}
}

func TestParseBucklingFactors(t *testing.T) {
	dat := `     B U C K L I N G   F A C T O R   O U T P U T

 MODE NO       BUCKLING
                FACTOR

      1   0.4231000E+01
      2   0.1079400E+02
`
	factors, err := parseBucklingFactors(strings.NewReader(dat))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(factors) != 2 || math.Abs(factors[0]-4.231) > 1e-3 {
		t.Errorf("factors = %v, want [4.231 10.794]", factors)
	}
}

func TestParseEigenFrequenciesMissingTable(t *testing.T) {
	if _, err := parseEigenFrequencies(strings.NewReader("no table here\n")); err == nil {
		t.Error("expected an error when the eigenvalue table is absent")
	}
}
