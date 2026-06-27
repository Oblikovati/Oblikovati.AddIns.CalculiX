// SPDX-License-Identifier: GPL-2.0-only

package ccx

import (
	"strings"
	"testing"
)

// validModel is a minimally complete model that passes the prerequisite checks.
func validModel() *AnalysisModel {
	return &AnalysisModel{
		Mesh:     unitTet(),
		Material: MaterialProps{Name: "M", YoungMPa: 210000, Poisson: 0.3},
		Fixed:    []FixedConstraint{{Name: "FIX", Nodes: []int{1}, DOFLow: 1, DOFHigh: 3}},
		Forces:   []ForceLoad{{Name: "L", Nodes: []int{2}, Dir: [3]float64{0, 0, -1}, TotalN: 10}},
	}
}

func TestCheckPrerequisitesAcceptsValidModel(t *testing.T) {
	if err := checkPrerequisites(validModel()); err != nil {
		t.Fatalf("valid model rejected: %v", err)
	}
}

func TestCheckPrerequisitesRejectsSetupMistakes(t *testing.T) {
	cases := []struct {
		name string
		want string
		mut  func(*AnalysisModel)
	}{
		{"no elements", "mesh", func(m *AnalysisModel) { m.Mesh = &TetMesh{} }},
		{"no Young's", "Young's modulus", func(m *AnalysisModel) { m.Material.YoungMPa = 0 }},
		{"bad Poisson", "Poisson", func(m *AnalysisModel) { m.Material.Poisson = 0.6 }},
		{"no support", "support", func(m *AnalysisModel) { m.Fixed = nil }},
		{"no load", "no load", func(m *AnalysisModel) { m.Forces = nil }},
		{"gravity no density", "density", func(m *AnalysisModel) {
			m.Forces = nil
			m.Gravity = &GravityLoad{Accel: 9810, Dir: [3]float64{0, 0, -1}}
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m := validModel()
			c.mut(m)
			err := checkPrerequisites(m)
			if err == nil {
				t.Fatalf("%s: expected an error", c.name)
			}
			if !strings.Contains(err.Error(), c.want) {
				t.Errorf("%s: error %q does not mention %q", c.name, err, c.want)
			}
		})
	}
}

func TestOpenEdgesDetectsHoles(t *testing.T) {
	coords, idx := boxSurface(10, 10, 10)
	box, err := weldSurface(coords, idx)
	if err != nil {
		t.Fatalf("weld: %v", err)
	}
	if open := box.openEdges(); open != 0 {
		t.Errorf("watertight box has %d open edges, want 0", open)
	}
	// Drop a triangle to punch a hole; its three edges become open.
	box.Tris = box.Tris[1:]
	if open := box.openEdges(); open == 0 {
		t.Error("box with a missing triangle should report open edges")
	}
}
