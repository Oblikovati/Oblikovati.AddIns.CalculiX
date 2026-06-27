// SPDX-License-Identifier: GPL-2.0-only

package ccx

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
)

// MeshOptions controls the volume mesh gmsh generates.
type MeshOptions struct {
	SizeMM float64      // characteristic element length (model units); 0 = auto from bbox
	Order  ElementOrder // tet element order (linear C3D4 / quadratic C3D10)
}

// VolumeMesher turns a watertight surface into a solid tetrahedral mesh.
type VolumeMesher interface {
	Mesh(surface *SurfaceMesh, opts MeshOptions, workdir string) (*TetMesh, error)
}

// GmshMesher drives the vendored gmsh CLI: it writes the surface as an STL, generates a
// .geo that wraps it in a volume, runs gmsh, and parses the resulting MSH into a TetMesh
// (already re-numbered to CalculiX C3D10 ordering).
type GmshMesher struct {
	bin string // path to the gmsh binary
}

// NewGmshMesher binds a mesher to the gmsh binary path.
func NewGmshMesher(gmshBin string) GmshMesher { return GmshMesher{bin: gmshBin} }

// autoSizeDivisor sets the default element length to bbox-diagonal / 15 when the caller
// does not specify a mesh size — coarse enough to be fast, fine enough that a quadratic
// tet beam resolves bending.
const autoSizeDivisor = 15.0

// Mesh writes part.stl + part.geo into workdir, runs gmsh, and returns the parsed tet
// mesh. The surface must be watertight; gmsh reports an error otherwise (surfaced here).
func (g GmshMesher) Mesh(surface *SurfaceMesh, opts MeshOptions, workdir string) (*TetMesh, error) {
	if len(surface.Tris) == 0 {
		return nil, fmt.Errorf("volume mesh: empty surface")
	}
	stlPath := filepath.Join(workdir, "part.stl")
	if err := writeFile(stlPath, func(f *os.File) error { return surface.writeSTL(f) }); err != nil {
		return nil, err
	}
	geoPath := filepath.Join(workdir, "part.geo")
	size := meshSize(opts.SizeMM, surface)
	if err := writeFile(geoPath, func(f *os.File) error { return writeGeo(f, "part.stl", size, opts.Order) }); err != nil {
		return nil, err
	}
	mshPath := filepath.Join(workdir, "part.msh")
	if err := runGmsh(g.bin, geoPath, mshPath); err != nil {
		return nil, err
	}
	return readMSHFile(mshPath)
}

// writeGeo emits the gmsh script that loads the STL surface, reclassifies it into smooth
// B-rep-like faces (split at 40° feature edges) so the surface can be REMESHED to the
// requested size — a raw Merge keeps the coarse host tessellation and refuses to refine
// — wraps all resulting faces in one volume, and sets the meshing controls. The
// reclassification also separates the surface into per-face groups (one elementary
// surface tag each), which mshparse records for FaceKey binding.
func writeGeo(f *os.File, stlName string, size float64, order ElementOrder) error {
	_, err := fmt.Fprintf(f, `Merge "%s";
ClassifySurfaces{40*Pi/180, 1, 1, Pi};
CreateGeometry;
Surface Loop(1) = Surface{:};
Volume(1) = {1};
Mesh.MeshSizeMax = %g;
Mesh.MeshSizeMin = 0;
Mesh.ElementOrder = %d;
Mesh.Algorithm3D = 1;
Mesh.Optimize = 1;
`, stlName, size, int(order))
	return err
}

// meshSize returns the requested element length, or an auto size derived from the
// surface bounding-box diagonal when none was given.
func meshSize(requested float64, surface *SurfaceMesh) float64 {
	if requested > 0 {
		return requested
	}
	return surfaceDiagonal(surface) / autoSizeDivisor
}

// surfaceDiagonal returns the length of the surface's bounding-box diagonal.
func surfaceDiagonal(surface *SurfaceMesh) float64 {
	if len(surface.Verts) == 0 {
		return 1
	}
	lo, hi := surface.Verts[0], surface.Verts[0]
	for _, v := range surface.Verts {
		for k := 0; k < 3; k++ {
			lo[k] = math.Min(lo[k], v[k])
			hi[k] = math.Max(hi[k], v[k])
		}
	}
	return math.Sqrt((hi[0]-lo[0])*(hi[0]-lo[0]) + (hi[1]-lo[1])*(hi[1]-lo[1]) + (hi[2]-lo[2])*(hi[2]-lo[2]))
}

// readMSHFile opens and parses a gmsh MSH file into a TetMesh.
func readMSHFile(path string) (*TetMesh, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open msh %s: %w", path, err)
	}
	defer f.Close()
	return parseMSH(f)
}
