// SPDX-License-Identifier: GPL-2.0-only

package ccx

import (
	"testing"

	"encoding/json"

	"oblikovati.org/api/wire"
)

// refKeysFor builds a ReferenceKeysResult whose body i owns the given face keys, matching the
// index alignment between body.list and model.referenceKeys the mapping relies on.
func refKeysFor(perBody ...[]string) wire.ReferenceKeysResult {
	var out wire.ReferenceKeysResult
	for _, keys := range perBody {
		var faces []wire.TopologyRef
		for _, k := range keys {
			faces = append(faces, wire.TopologyRef{Key: k})
		}
		out.Bodies = append(out.Bodies, wire.BodyTopology{Faces: faces})
	}
	return out
}

func TestBodiesOwningFacesPicksOnlyMatchingBodies(t *testing.T) {
	solids := []wire.BodyInfo{
		{Index: 0, Name: "A", Solid: true, Key: "bA"},
		{Index: 1, Name: "B", Solid: true, Key: "bB"},
		{Index: 2, Name: "C", Solid: true, Key: "bC"},
	}
	refs := refKeysFor(
		[]string{"a0", "a1"}, // body A faces
		[]string{"b0", "b1"}, // body B faces
		[]string{"c0"},       // body C faces
	)
	// Selected faces live on A and C; B has none.
	got := bodiesOwningFaces(solids, refs, []string{"a1", "c0"})
	if len(got) != 2 || got[0].Name != "A" || got[1].Name != "C" {
		t.Fatalf("expected bodies A and C, got %v", names(got))
	}
}

func TestBodiesOwningFacesEmptyWhenNoMatch(t *testing.T) {
	solids := []wire.BodyInfo{{Index: 0, Key: "bA"}}
	refs := refKeysFor([]string{"a0"})
	if got := bodiesOwningFaces(solids, refs, []string{"unknown"}); len(got) != 0 {
		t.Fatalf("expected no owning body for an unmatched face, got %d", len(got))
	}
}

// TestBodiesOwningFacesRespectsSolidIndex checks that a solid's Index — not its position in the
// filtered solids slice — addresses its topology, so a study that skipped a non-solid body still
// maps faces to the right body.
func TestBodiesOwningFacesRespectsSolidIndex(t *testing.T) {
	// body.list order is [surface(0), solid(1)]; solids is filtered to just the solid at Index 1.
	solids := []wire.BodyInfo{{Index: 1, Name: "Solid", Solid: true, Key: "bS"}}
	refs := refKeysFor(
		[]string{"surf0"}, // index 0: a non-solid body
		[]string{"s0"},    // index 1: the solid
	)
	got := bodiesOwningFaces(solids, refs, []string{"s0"})
	if len(got) != 1 || got[0].Name != "Solid" {
		t.Fatalf("expected the solid at Index 1, got %v", names(got))
	}
}

// scopeFakeHost serves body.list + model.referenceKeys for the scope-decision test: three solid
// bodies, each with one distinct face key.
type scopeFakeHost struct{}

func (scopeFakeHost) Call(method string, _ []byte) ([]byte, error) {
	switch method {
	case wire.MethodBodyList:
		return json.Marshal(wire.BodyListResult{Bodies: []wire.BodyInfo{
			{Index: 0, Name: "A", Solid: true, Key: "bA"},
			{Index: 1, Name: "B", Solid: true, Key: "bB"},
		}})
	case wire.MethodModelReferenceKeys:
		return json.Marshal(refKeysFor([]string{"a0"}, []string{"b0"}))
	default:
		return []byte("{}"), nil
	}
}

func TestScopeBodiesSelectedRestrictsToPickedBody(t *testing.T) {
	e := NewEngine(scopeFakeHost{})
	solids := []wire.BodyInfo{
		{Index: 0, Name: "A", Solid: true, Key: "bA"},
		{Index: 1, Name: "B", Solid: true, Key: "bB"},
	}
	// Scope=selected with a face on body B only → just B.
	got, err := e.scopeBodies(StudySettings{BodyScope: BodyScopeSelected}, solids, []string{"b0"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Name != "B" {
		t.Fatalf("BodyScopeSelected should restrict to body B, got %v", names(got))
	}
	// Default scope → all solids, no reference-keys call needed.
	all, err := e.scopeBodies(StudySettings{BodyScope: BodyScopeAll}, solids, []string{"b0"})
	if err != nil || len(all) != 2 {
		t.Fatalf("BodyScopeAll should keep all bodies, got %v (err %v)", names(all), err)
	}
}

// TestScopeBodiesSelectedFallsBackWhenNoFaceMatches checks the safety fallback: an opt-in scope
// whose selected faces match no body keeps the whole part rather than analysing nothing.
func TestScopeBodiesSelectedFallsBackWhenNoFaceMatches(t *testing.T) {
	e := NewEngine(scopeFakeHost{})
	solids := []wire.BodyInfo{{Index: 0, Name: "A", Solid: true, Key: "bA"}, {Index: 1, Name: "B", Solid: true, Key: "bB"}}
	got, err := e.scopeBodies(StudySettings{BodyScope: BodyScopeSelected}, solids, []string{"nope"})
	if err != nil || len(got) != 2 {
		t.Fatalf("an unmatched selection must fall back to all bodies, got %v (err %v)", names(got), err)
	}
}

func names(bodies []wire.BodyInfo) []string {
	out := make([]string, len(bodies))
	for i, b := range bodies {
		out[i] = b.Name
	}
	return out
}
