// SPDX-License-Identifier: GPL-2.0-only

package ccx

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
)

// solverBinaries locates the vendored ccx solver and gmsh mesher the engine runs at
// arm's length (subprocess + file exchange; the engine never links them). Both are
// built from source under vendor-src/ (see vendor-src/*/NOTICE.md).
type solverBinaries struct {
	ccx  string
	gmsh string
}

// findSolverBinaries resolves the solver paths, erroring if either is missing so the
// failure names what to build rather than surfacing a cryptic exec error. Resolution:
// OBK_CCX_BIN / OBK_GMSH_BIN (a directory or a direct path), else vendor-src/*/build.
func findSolverBinaries() (solverBinaries, error) {
	b := solverBinaries{
		ccx:  resolveBinary("OBK_CCX_BIN", "vendor-src/ccx/build", "ccx"),
		gmsh: resolveBinary("OBK_GMSH_BIN", "vendor-src/gmsh/build", "gmsh"),
	}
	for tool, p := range map[string]string{"ccx": b.ccx, "gmsh": b.gmsh} {
		if _, err := os.Stat(p); err != nil {
			return b, fmt.Errorf("%s binary missing: %s (run `make build-solvers` or set OBK_%s_BIN): %w",
				tool, p, envSuffix(tool), err)
		}
	}
	return b, nil
}

// resolveBinary returns the binary path from an env override (a file or a directory
// holding the named binary) or the in-repo build directory.
func resolveBinary(env, defaultDir, name string) string {
	dir := os.Getenv(env)
	if dir == "" {
		dir = defaultDir
	}
	if fi, err := os.Stat(dir); err == nil && !fi.IsDir() {
		return dir // env pointed straight at the binary
	}
	return filepath.Join(dir, name)
}

// envSuffix maps a tool name to its env-var infix (ccx -> CCX, gmsh -> GMSH).
func envSuffix(tool string) string {
	if tool == "ccx" {
		return "CCX"
	}
	return "GMSH"
}

// runGmsh runs the mesher on geoPath, writing the MSH 2.2 mesh to outPath. -3 generates
// the 3D (volume) mesh; -format msh2 is the parser's input.
func runGmsh(gmsh, geoPath, outPath string) error {
	cmd := exec.Command(gmsh, geoPath, "-3", "-format", "msh2", "-o", outPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("gmsh mesh %s: %w: %s", geoPath, err, out)
	}
	return nil
}

// runCcx runs the solver on the deck stem (the .inp basename without extension), writing
// <stem>.frd / <stem>.dat. OMP_NUM_THREADS is set so ccx uses the available cores; ccx is
// sensitive to the working directory, so it runs in the deck's directory.
func runCcx(ccx, stem string) error {
	cmd := exec.Command(ccx, "-i", filepath.Base(stem))
	cmd.Dir = filepath.Dir(stem)
	cmd.Env = append(os.Environ(), "OMP_NUM_THREADS="+strconv.Itoa(runtime.NumCPU()))
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("ccx solve %s: %w: %s", stem, err, out)
	}
	return nil
}

// writeFile creates path and hands the open file to write, ensuring it is closed.
func writeFile(path string, write func(*os.File) error) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create %s: %w", path, err)
	}
	defer f.Close()
	if err := write(f); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}
