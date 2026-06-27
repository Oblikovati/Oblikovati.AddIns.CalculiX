// SPDX-License-Identifier: GPL-2.0-only

package ccx

import (
	"encoding/base64"
	"encoding/json"
	"math"
	"sync"
	"testing"

	"oblikovati.org/api/wire"
)

// boxHost is a fake host serving one box body and a two-face selection (a fixed face and
// a loaded face), enough to drive RunStudyOnHost end-to-end through the real vendored
// gmsh + ccx. Coordinates are in host model units (1 unit = 10 mm); the box is 20×1×1
// model units, i.e. a 200×10×10 mm beam after the engine's cm→mm scaling.
type boxHost struct {
	mu    sync.Mutex
	calls map[string]int
	box   [8][3]float64
}

func newBoxHost() *boxHost {
	const l, h = 20.0, 1.0
	return &boxHost{
		calls: map[string]int{},
		box: [8][3]float64{
			{0, 0, 0}, {l, 0, 0}, {l, h, 0}, {0, h, 0},
			{0, 0, h}, {l, 0, h}, {l, h, h}, {0, h, h},
		},
	}
}

const (
	fixedFaceKey  = "fixed"
	loadedFaceKey = "loaded"
)

func (b *boxHost) Call(method string, req []byte) ([]byte, error) {
	b.mu.Lock()
	b.calls[method]++
	b.mu.Unlock()
	switch method {
	case wire.MethodModelSelection:
		// The host encodes a selected face as "face/<url-base64 of the raw key>".
		refs := []string{encodeFaceRef(fixedFaceKey), encodeFaceRef(loadedFaceKey)}
		return json.Marshal(wire.SelectionResult{Count: 2, Refs: refs})
	case wire.MethodBodyCalculateFacets:
		return json.Marshal(b.bodyFacets())
	case wire.MethodFaceCalculateFacets:
		return b.faceFacets(req)
	default:
		return []byte("{}"), nil // graphics register/set return no body the engine reads
	}
}

func (b *boxHost) saw(method string) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.calls[method] > 0
}

// bodyFacets returns the whole box surface as a raw triangle soup.
func (b *boxHost) bodyFacets() wire.FacetSetResult {
	quads := [6][4]int{{0, 3, 2, 1}, {4, 5, 6, 7}, {0, 1, 5, 4}, {1, 2, 6, 5}, {2, 3, 7, 6}, {3, 0, 4, 7}}
	var coords []float64
	var idx []int
	for _, q := range quads {
		coords, idx = appendQuad(coords, idx, b.box, q)
	}
	return wire.FacetSetResult{VertexCoordinates: coords, VertexIndices: idx}
}

// faceFacets returns the two triangles of the requested face (the x=0 face for the fixed
// key, the x=L face for the loaded key).
func (b *boxHost) faceFacets(req []byte) ([]byte, error) {
	var args wire.FaceFacetsArgs
	if err := json.Unmarshal(req, &args); err != nil {
		return nil, err
	}
	quad := [4]int{1, 2, 6, 5} // x=L (loaded)
	if args.FaceKey == fixedFaceKey {
		quad = [4]int{0, 3, 7, 4} // x=0 (fixed)
	}
	var coords []float64
	var idx []int
	coords, idx = appendQuad(coords, idx, b.box, quad)
	return json.Marshal(wire.FacetSetResult{VertexCoordinates: coords, VertexIndices: idx})
}

// encodeFaceRef mirrors the host's selection encoding: "face/" + url-base64(raw key).
func encodeFaceRef(rawKey string) string {
	return faceRefPrefix + base64.RawURLEncoding.EncodeToString([]byte(rawKey))
}

// appendQuad appends a quad's two triangles to the coordinate/index soup.
func appendQuad(coords []float64, idx []int, v [8][3]float64, q [4]int) ([]float64, []int) {
	base := len(coords) / 3
	for _, c := range q {
		coords = append(coords, v[c][0], v[c][1], v[c][2])
	}
	return coords, append(idx, base, base+1, base+2, base, base+2, base+3)
}

// TestRunStudyOnHostDrivesFullPipeline runs the whole host-facing flow against the real
// vendored solvers: selection -> surface pull -> gmsh volume mesh -> face-group binding
// -> deck -> ccx solve -> .frd parse -> client-graphics render. It asserts a physical
// result and that the geometry/graphics surfaces were all exercised.
func TestRunStudyOnHostDrivesFullPipeline(t *testing.T) {
	bins := requireSolver(t)
	t.Setenv("OBK_CCX_BIN", bins.ccx)
	t.Setenv("OBK_GMSH_BIN", bins.gmsh)

	h := newBoxHost()
	e := NewEngine(h)
	e.applyPanelEdit("mesh_size", "4") // a reasonable mesh on the 200×10×10 beam

	res, err := e.RunStudyOnHost()
	if err != nil {
		t.Fatalf("RunStudyOnHost: %v", err)
	}
	if res.ElementCount == 0 {
		t.Error("no elements meshed")
	}
	if !(res.FieldPeak > 0) {
		t.Errorf("field peak = %v, want a positive value", res.FieldPeak)
	}
	if !(res.MaxDisplacement > 0) {
		t.Errorf("max displacement = %v, want a positive deflection", res.MaxDisplacement)
	}
	for _, m := range []string{
		wire.MethodModelSelection,
		wire.MethodBodyCalculateFacets,
		wire.MethodFaceCalculateFacets,
		wire.MethodClientGraphicsRegisterMapper,
		wire.MethodClientGraphicsSet,
	} {
		if !h.saw(m) {
			t.Errorf("study never called %q", m)
		}
	}
}

// TestRunStudyOnHostResultFieldSelector drives the full real-solver pipeline twice on the
// same cantilever, switching only the result-field selector, and asserts the reported field
// label/unit follow the selection — the live cross-check of the selector code path. The
// max-principal-stress peak must be positive (the beam's tensile fibre) and the displacement
// field must report in mm, distinct from the stress field's MPa.
func TestRunStudyOnHostResultFieldSelector(t *testing.T) {
	bins := requireSolver(t)

	run := func(field ResultFieldKind) *StudyResult {
		t.Setenv("OBK_CCX_BIN", bins.ccx)
		t.Setenv("OBK_GMSH_BIN", bins.gmsh)
		e := NewEngine(newBoxHost())
		e.applyPanelEdit("mesh_size", "4")
		e.applyPanelEdit("result_field", string(field))
		res, err := e.RunStudyOnHost()
		if err != nil {
			t.Fatalf("RunStudyOnHost(%s): %v", field, err)
		}
		return res
	}

	principal := run(ResultMaxPrincipal)
	if principal.FieldLabel != string(ResultMaxPrincipal) || principal.FieldUnit != "MPa" {
		t.Errorf("principal: label=%q unit=%q, want %q/MPa", principal.FieldLabel, principal.FieldUnit, ResultMaxPrincipal)
	}
	if !(principal.FieldPeak > 0) {
		t.Errorf("max principal peak = %v, want a positive tensile stress", principal.FieldPeak)
	}

	disp := run(ResultDisplacement)
	if disp.FieldLabel != string(ResultDisplacement) || disp.FieldUnit != "mm" {
		t.Errorf("displacement: label=%q unit=%q, want %q/mm", disp.FieldLabel, disp.FieldUnit, ResultDisplacement)
	}
	if !(disp.FieldPeak > 0) {
		t.Errorf("displacement peak = %v, want a positive deflection", disp.FieldPeak)
	}
}

// TestRunStudyOnHostHeatTransfer drives the heat-transfer path end-to-end against the real
// vendored solver: the first face is held at 0 K, the second carries a heat flux, and the
// study returns a temperature field with a gradient.
func TestRunStudyOnHostHeatTransfer(t *testing.T) {
	bins := requireSolver(t)
	t.Setenv("OBK_CCX_BIN", bins.ccx)
	t.Setenv("OBK_GMSH_BIN", bins.gmsh)

	e := NewEngine(newBoxHost())
	e.applyPanelEdit("analysis", string(AnalysisHeatTransfer))
	e.applyPanelEdit("mesh_size", "4")

	res, err := e.RunStudyOnHost()
	if err != nil {
		t.Fatalf("RunStudyOnHost (heat): %v", err)
	}
	if res.Scalar == nil {
		t.Fatal("heat-transfer study returned no temperature result")
	}
	if res.Scalar.Label != "temperature" || res.Scalar.Unit != "K" {
		t.Errorf("scalar field = %q (%s), want temperature/K", res.Scalar.Label, res.Scalar.Unit)
	}
	if !(res.Scalar.Max > res.Scalar.Min) {
		t.Errorf("temperature range = %.3g..%.3g K, want a gradient", res.Scalar.Min, res.Scalar.Max)
	}
}

// TestRunStudyOnHostElectrostatic drives the electric-conduction path end-to-end against the
// real vendored solver: the first face holds the applied voltage, the second is grounded, and
// the study returns an electric-potential field with a gradient spanning [0, V].
func TestRunStudyOnHostElectrostatic(t *testing.T) {
	bins := requireSolver(t)
	t.Setenv("OBK_CCX_BIN", bins.ccx)
	t.Setenv("OBK_GMSH_BIN", bins.gmsh)

	e := NewEngine(newBoxHost())
	e.applyPanelEdit("analysis", string(AnalysisElectromagnetic))
	e.applyPanelEdit("mesh_size", "4")
	e.applyPanelEdit("voltage", "12")

	res, err := e.RunStudyOnHost()
	if err != nil {
		t.Fatalf("RunStudyOnHost (electrostatic): %v", err)
	}
	if res.Scalar == nil {
		t.Fatal("electrostatic study returned no potential result")
	}
	if res.Scalar.Label != "electric potential" || res.Scalar.Unit != "V" {
		t.Errorf("scalar field = %q (%s), want electric potential/V", res.Scalar.Label, res.Scalar.Unit)
	}
	if math.Abs(res.Scalar.Min) > 0.1 || math.Abs(res.Scalar.Max-12) > 0.6 {
		t.Errorf("potential range = %.3g..%.3g V, want ~0..12 V", res.Scalar.Min, res.Scalar.Max)
	}
}
