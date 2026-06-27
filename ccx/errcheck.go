// SPDX-License-Identifier: GPL-2.0-only

package ccx

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// ccxErrorMarker is how CalculiX prefixes a fatal diagnostic on stdout. ccx can print
// these and still exit cleanly, or exit non-zero without a clear Go-level error, so the
// output is always scraped rather than relying on the exit code alone.
const ccxErrorMarker = "*ERROR"

// jacobianElemRE extracts the element number from a nonpositive-jacobian diagnostic.
var jacobianElemRE = regexp.MustCompile(`element\s+(\d+)`)

// scrapeCcxErrors returns a human-readable diagnostic if the solver output contains any
// fatal error, or "" if it ran cleanly. Known failure modes get an actionable hint; a
// nonpositive-jacobian failure additionally lists the offending element numbers.
func scrapeCcxErrors(output string) string {
	errs := errorLines(output)
	if len(errs) == 0 {
		return ""
	}
	msg := strings.Join(errs, "; ")
	if hint := errorHint(output, msg); hint != "" {
		return msg + " — " + hint
	}
	return msg
}

// errorLines returns the de-duplicated, trimmed *ERROR lines from the solver output.
func errorLines(output string) []string {
	seen := map[string]bool{}
	var out []string
	for _, line := range strings.Split(output, "\n") {
		if !strings.Contains(line, ccxErrorMarker) {
			continue
		}
		t := strings.TrimSpace(line)
		if !seen[t] {
			seen[t] = true
			out = append(out, t)
		}
	}
	return out
}

// errorHint maps a known failure signature to an actionable suggestion.
func errorHint(output, msg string) string {
	lower := strings.ToLower(msg)
	switch {
	case strings.Contains(lower, "material"):
		return "assign a material with a valid Young's modulus and Poisson's ratio"
	case strings.Contains(lower, "nonpositive jacobian"):
		if els := nonpositiveJacobianElements(output); len(els) > 0 {
			return fmt.Sprintf("distorted elements %v — refine or improve the mesh", els)
		}
		return "the mesh has distorted elements — refine or improve it"
	case strings.Contains(lower, "singular"):
		return "the model is under-constrained (add or check the fixed support)"
	default:
		return ""
	}
}

// nonpositiveJacobianElements collects the distinct element numbers cited in
// nonpositive-jacobian diagnostics, sorted ascending.
func nonpositiveJacobianElements(output string) []int {
	seen := map[int]bool{}
	for _, line := range strings.Split(output, "\n") {
		if !strings.Contains(line, "nonpositive jacobian") {
			continue
		}
		if m := jacobianElemRE.FindStringSubmatch(line); m != nil {
			if n, err := strconv.Atoi(m[1]); err == nil {
				seen[n] = true
			}
		}
	}
	els := make([]int, 0, len(seen))
	for n := range seen {
		els = append(els, n)
	}
	sort.Ints(els)
	return els
}

// lastLines returns the final n non-empty lines of the solver output, for surfacing
// context when no recognised *ERROR was found.
func lastLines(output string, n int) string {
	var lines []string
	for _, l := range strings.Split(output, "\n") {
		if strings.TrimSpace(l) != "" {
			lines = append(lines, l)
		}
	}
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}
