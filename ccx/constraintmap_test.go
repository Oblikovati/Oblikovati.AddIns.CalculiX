// SPDX-License-Identifier: GPL-2.0-only

package ccx

import (
	"reflect"
	"testing"

	"oblikovati.org/calculix/ccx/femmodel"
)

// femmodelNewDefaultForTest is a one-line helper so test bodies stay readable.
func femmodelNewDefaultForTest() *femmodel.Analysis { return femmodel.NewDefaultAnalysis() }

// The migration must be behavior-identical: for every builder kind, mapping settings → object →
// spec must reproduce EXACTLY what newConstraintSpec produced. This is the guard for slice 2.7.
func TestConstraintRoundTripMatchesNewConstraintSpec(t *testing.T) {
	s := defaultSettings()
	faces := []string{"face/a", "face/b"}
	for _, k := range builderKinds() {
		a := femmodelNewDefaultForTest()
		obj := a.AddConstraint("C0", objectForKind(k, faces, s))
		got := constraintSpecFor(obj)
		want := newConstraintSpec(k, "C0", faces, s)
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("kind %q: constraintSpecFor(objectForKind) = %#v, want newConstraintSpec = %#v", k, got, want)
		}
	}
}

func TestMapConstraintsPreservesOrderAndKind(t *testing.T) {
	s := defaultSettings()
	a := femmodelNewDefaultForTest()
	a.AddConstraint("C0", objectForKind(KindForce, []string{"face/a"}, s))
	a.AddConstraint("C1", objectForKind(KindFixed, []string{"face/b"}, s))
	specs := mapConstraints(a.Constraints())
	if len(specs) != 2 || specs[0].Kind() != KindForce || specs[1].Kind() != KindFixed {
		t.Fatalf("mapConstraints = %+v, want [force fixed]", specs)
	}
}
