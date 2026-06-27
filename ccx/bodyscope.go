// SPDX-License-Identifier: GPL-2.0-only

package ccx

import (
	"fmt"

	"oblikovati.org/api/wire"
)

// scopeBodies restricts the solid bodies a study analyses to those the user picked, when the
// study is set to BodyScopeSelected. A body is "picked" when it owns one of the selected faces;
// the owning body is found through model.referenceKeys, which lists each body's persistent face
// reference keys in the SAME order as body.list (both enumerate SurfaceBodies().All()), so a
// solid's Index addresses its topology. BodyInfo.Key (new in api v0.92.1) is what makes a body a
// first-class, recompute-stable reference alongside its faces; before it, a body could not be
// addressed in the same key space as the faces a user selects.
//
// The default scope (BodyScopeAll) and any case that resolves to no owning body fall back to the
// full solid set, so this never analyses an empty model or changes the existing whole-part flow.
func (e *Engine) scopeBodies(settings StudySettings, solids []wire.BodyInfo, faceKeys []string) ([]wire.BodyInfo, error) {
	if settings.BodyScope != BodyScopeSelected || len(faceKeys) == 0 {
		return solids, nil
	}
	refs, err := e.api.Model().ReferenceKeys()
	if err != nil {
		return nil, fmt.Errorf("read reference keys for body scope: %w", err)
	}
	scoped := bodiesOwningFaces(solids, refs, faceKeys)
	if len(scoped) == 0 {
		return solids, nil // selected faces matched no body's topology — keep the whole part
	}
	return scoped, nil
}

// bodiesOwningFaces returns the solid bodies that own at least one of faceKeys, preserving the
// input order. A body owns a face when that face's reference key appears in the body's topology
// (refs.Bodies[body.Index].Faces), exploiting the index alignment between body.list and
// model.referenceKeys.
func bodiesOwningFaces(solids []wire.BodyInfo, refs wire.ReferenceKeysResult, faceKeys []string) []wire.BodyInfo {
	want := make(map[string]bool, len(faceKeys))
	for _, k := range faceKeys {
		want[k] = true
	}
	var out []wire.BodyInfo
	for _, b := range solids {
		if b.Index < 0 || b.Index >= len(refs.Bodies) {
			continue
		}
		if topologyOwnsAnyFace(refs.Bodies[b.Index], want) {
			out = append(out, b)
		}
	}
	return out
}

// topologyOwnsAnyFace reports whether any of a body's face reference keys is in want.
func topologyOwnsAnyFace(t wire.BodyTopology, want map[string]bool) bool {
	for _, f := range t.Faces {
		if want[f.Key] {
			return true
		}
	}
	return false
}
