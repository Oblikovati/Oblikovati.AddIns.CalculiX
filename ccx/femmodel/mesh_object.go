// SPDX-License-Identifier: GPL-2.0-only

package femmodel

// MeshObject is the volume-mesh object: gmsh characteristic length (mm; 0 = auto) and element
// order (Quadratic = C3D10, the default; false = linear C3D4).
type MeshObject struct {
	id        string
	MaxSizeMM float64
	Quadratic bool
}

func newMeshObject(id string, maxSizeMM float64, quadratic bool) MeshObject {
	return MeshObject{id: id, MaxSizeMM: maxSizeMM, Quadratic: quadratic}
}

func (o MeshObject) ObjectID() string  { return o.id }
func (o MeshObject) Category() Category { return CategoryMesh }
func (o MeshObject) Name() string       { return "Mesh" }
