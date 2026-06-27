// SPDX-License-Identifier: GPL-2.0-only

package ccx

import (
	"encoding/base64"
	"testing"
)

func TestDecodeSelectedFacesKeepsAndDecodesFaces(t *testing.T) {
	rawKey := "\x03Extrusion1:side#3" // a real reference key starts with a control byte
	encoded := faceRefPrefix + base64.RawURLEncoding.EncodeToString([]byte(rawKey))
	refs := []string{
		encoded,
		"edge/AAAA",    // dropped: not a face
		"workplane/xy", // dropped: not a face
	}
	faces := decodeSelectedFaces(refs)
	if len(faces) != 1 {
		t.Fatalf("decoded %d faces, want 1 (refs: %v)", len(faces), faces)
	}
	if faces[0] != rawKey {
		t.Errorf("decoded face key = %q, want %q", faces[0], rawKey)
	}
}

func TestDecodeFaceRefRejectsMalformed(t *testing.T) {
	if _, ok := decodeFaceRef("face/not valid base64!!"); ok {
		t.Error("malformed base64 should not decode")
	}
	if _, ok := decodeFaceRef("vertex/AAAA"); ok {
		t.Error("non-face reference should not decode")
	}
}
