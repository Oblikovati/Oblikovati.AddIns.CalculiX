// SPDX-License-Identifier: GPL-2.0-only

package ccx

import (
	"bufio"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
)

// CalculiX writes eigenvalue and buckling-factor tables to the .dat file with spaced-out
// banner headings. The despaced heading is matched so the exact spacing is irrelevant.
const (
	eigenHeading    = "EIGENVALUEOUTPUT"
	bucklingHeading = "BUCKLINGFACTOROUTPUT"
)

// parseEigenFrequencies reads the natural frequencies (Hz) from a *FREQUENCY .dat file, in
// mode order. Each data row is "mode eigenvalue rad/time cycles/time imag"; the Hz value is
// the cycles/time column.
func parseEigenFrequencies(r io.Reader) ([]float64, error) {
	rows, err := parseDatTable(r, eigenHeading, 4)
	if err != nil {
		return nil, err
	}
	freqs := make([]float64, len(rows))
	for i, row := range rows {
		freqs[i] = row[3] // cycles/time (Hz)
	}
	return freqs, nil
}

// parseBucklingFactors reads the buckling load factors from a *BUCKLE .dat file, in mode
// order. Each data row is "mode factor".
func parseBucklingFactors(r io.Reader) ([]float64, error) {
	rows, err := parseDatTable(r, bucklingHeading, 2)
	if err != nil {
		return nil, err
	}
	factors := make([]float64, len(rows))
	for i, row := range rows {
		factors[i] = row[1] // the buckling factor
	}
	return factors, nil
}

// parseDatTable returns the numeric data rows of the .dat table introduced by heading
// (matched despaced). A row qualifies when it has at least minCols numeric fields whose
// first field is an integer mode number; collection stops at the first non-data row after
// the table begins.
func parseDatTable(r io.Reader, heading string, minCols int) ([][]float64, error) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 1024*1024), 8*1024*1024)
	inTable, started := false, false
	var rows [][]float64
	for sc.Scan() {
		line := sc.Text()
		if !inTable {
			if strings.Contains(despace(line), heading) {
				inTable = true
			}
			continue
		}
		if row, ok := datRow(line, minCols); ok {
			started, rows = true, append(rows, row)
			continue
		}
		if started {
			break // table ended
		}
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("read dat: %w", err)
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("no %s table in the .dat file", heading)
	}
	return rows, nil
}

// datRow parses a numeric table row, requiring an integer first field (the mode number)
// and at least minCols fields in total.
func datRow(line string, minCols int) ([]float64, bool) {
	fields := strings.Fields(line)
	if len(fields) < minCols {
		return nil, false
	}
	if _, err := strconv.Atoi(fields[0]); err != nil {
		return nil, false
	}
	vals := make([]float64, len(fields))
	for i, f := range fields {
		v, err := strconv.ParseFloat(f, 64)
		if err != nil {
			return nil, false
		}
		vals[i] = v
	}
	return vals, true
}

// reactionHeading is the despaced banner CalculiX prints before a *NODE PRINT TOTALS=ONLY RF
// block: "total force (fx,fy,fz) for set <NAME> and time ...".
const reactionHeading = "totalforce(fx,fy,fz)"

// parseTotalReaction reads the total support reaction vector (fx, fy, fz) from a static .dat
// file's *NODE PRINT TOTALS=ONLY block. Returns the first such total (one clamped support),
// or an error when the deck requested no reaction print.
func parseTotalReaction(r io.Reader) ([3]float64, error) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 1024*1024), 8*1024*1024)
	seen := false
	for sc.Scan() {
		line := sc.Text()
		if !seen {
			if strings.Contains(despace(line), reactionHeading) {
				seen = true
			}
			continue
		}
		if v, ok := reactionRow(line); ok {
			return v, nil
		}
	}
	if err := sc.Err(); err != nil {
		return [3]float64{}, fmt.Errorf("read dat: %w", err)
	}
	return [3]float64{}, fmt.Errorf("no total-reaction (%q) block in the .dat file", reactionHeading)
}

// reactionRow parses the three force components of a total-reaction line (all floats — unlike a
// mode-table row, there is no leading integer index).
func reactionRow(line string) ([3]float64, bool) {
	fields := strings.Fields(line)
	if len(fields) < 3 {
		return [3]float64{}, false
	}
	var v [3]float64
	for i := 0; i < 3; i++ {
		x, err := strconv.ParseFloat(fields[i], 64)
		if err != nil {
			return [3]float64{}, false
		}
		v[i] = x
	}
	return v, true
}

// reactionMagnitude returns the Euclidean magnitude of a reaction force vector.
func reactionMagnitude(v [3]float64) float64 {
	return math.Sqrt(v[0]*v[0] + v[1]*v[1] + v[2]*v[2])
}

// despace removes all spaces, so a spaced-out CalculiX banner heading matches a plain key.
func despace(s string) string { return strings.ReplaceAll(s, " ", "") }
