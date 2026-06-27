// SPDX-License-Identifier: GPL-2.0-only

package ccx

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// FRD record geometry (CalculiX writes fixed-width fields). A data record is
//
//	" -1" + node-id(I10) + values(E12.5 each)
//
// so values must be sliced by column, not split on whitespace — adjacent negative
// values run together with no separator.
const (
	frdKeyWidth   = 3  // leading " -1" / " -2" / " -3" / " -4" / " -5"
	frdNodeWidth  = 10 // node id field after the key
	frdValueWidth = 12 // each E12.5 value field
	frdDataStart  = frdKeyWidth + frdNodeWidth
)

// ResultField holds the nodal results read back from a .frd: displacement vectors and
// the symmetric stress tensor (xx, yy, zz, xy, yz, zx), keyed by node id.
type ResultField struct {
	Disp   map[int][3]float64
	Stress map[int][6]float64
}

// frdMode tracks which result block the parser is inside.
type frdMode int

const (
	frdNone frdMode = iota
	frdDisp
	frdStress
)

// parseFRD reads the displacement and stress blocks of a CalculiX .frd result file.
// Geometry (node/element) blocks and any other result blocks are skipped. The last
// occurrence of each block wins (the final converged increment).
func parseFRD(r io.Reader) (*ResultField, error) {
	res := &ResultField{Disp: map[int][3]float64{}, Stress: map[int][6]float64{}}
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 1024*1024), 16*1024*1024)
	mode := frdNone
	for sc.Scan() {
		line := sc.Text()
		mode = stepFRD(res, line, mode)
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("read frd: %w", err)
	}
	if len(res.Disp) == 0 {
		return nil, fmt.Errorf("frd has no displacement block")
	}
	return res, nil
}

// stepFRD advances the parser by one line, returning the (possibly changed) mode.
func stepFRD(res *ResultField, line string, mode frdMode) frdMode {
	key := strings.TrimSpace(frdSlice(line, 0, frdKeyWidth))
	switch key {
	case "-4":
		return frdBlockMode(line)
	case "-3":
		return frdNone
	case "-1":
		readFRDData(res, line, mode)
		return mode
	default:
		return mode
	}
}

// frdBlockMode maps a " -4 <NAME>" block header to the mode for its data rows.
func frdBlockMode(line string) frdMode {
	switch {
	case strings.Contains(line, "DISP"):
		return frdDisp
	case strings.Contains(line, "STRESS"):
		return frdStress
	default:
		return frdNone // a result block we don't consume (e.g. error estimator)
	}
}

// readFRDData stores one " -1" data row into the field for the active block.
func readFRDData(res *ResultField, line string, mode frdMode) {
	switch mode {
	case frdDisp:
		if id, v, ok := frdRow(line, 3); ok {
			res.Disp[id] = [3]float64{v[0], v[1], v[2]}
		}
	case frdStress:
		if id, v, ok := frdRow(line, 6); ok {
			res.Stress[id] = [6]float64{v[0], v[1], v[2], v[3], v[4], v[5]}
		}
	}
}

// frdRow parses a data record: the node id and n fixed-width value fields.
func frdRow(line string, n int) (int, []float64, bool) {
	id, err := strconv.Atoi(strings.TrimSpace(frdSlice(line, frdKeyWidth, frdDataStart)))
	if err != nil {
		return 0, nil, false
	}
	vals := make([]float64, n)
	for i := 0; i < n; i++ {
		start := frdDataStart + i*frdValueWidth
		f, err := strconv.ParseFloat(strings.TrimSpace(frdSlice(line, start, start+frdValueWidth)), 64)
		if err != nil {
			return 0, nil, false
		}
		vals[i] = f
	}
	return id, vals, true
}

// frdSlice returns line[lo:hi], clamped to the line length (FRD rows may be space-padded
// or trimmed by the writer).
func frdSlice(line string, lo, hi int) string {
	if lo > len(line) {
		return ""
	}
	if hi > len(line) {
		hi = len(line)
	}
	return line[lo:hi]
}
