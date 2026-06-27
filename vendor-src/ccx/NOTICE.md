# Vendored third-party solver stack — provenance, versions & licenses

This directory vendors CalculiX and its numeric dependencies **in source form** so the
add-in builds a fully self-contained `ccx` binary with **no build-time or runtime
external dependencies** (beyond a C/Fortran toolchain and libc/libgfortran/libgomp).
`build.sh` compiles everything here into `build/ccx`.

Each component keeps its **own upstream license** (below). They are GPL-compatible, but
this is **not** "this repo's GPL applies to them" — the licenses here govern these
sources. The SPDX header tool (`scripts/add-spdx-headers.py`) deliberately skips
`vendor-src/`.

## Components (exact upstream releases)

| Component | Version | Upstream | SHA-256 of source archive | License |
|---|---|---|---|---|
| CalculiX `ccx` | 2.22 | http://www.dhondt.de/ccx_2.22.src.tar.bz2 | `3a94dcc775a31f570229734b341d6b06301ebdc759863df901c8b9bf1854c0bc` | GPL-2.0-or-later (© 1998–2024 Guido Dhondt) |
| SPOOLES | 2.2 | https://netlib.org/linalg/spooles/spooles.2.2.tgz | `a84559a0e987a1e423055ef4fdf3035d55b65bbe4bf915efaa1a35bef7f8c5dd` | SPOOLES free-use license (Boeing / C. Ashcraft) |
| Reference LAPACK (incl. BLAS) | 3.8.0 | https://github.com/Reference-LAPACK/lapack (tag v3.8.0) | `deb22cc4a6120bff72621155a9917f485f96ef8319ac074a7afbc68aab88bcf6` | modified BSD-3-Clause (see `lapack-3.8.0/LICENSE`) |
| arpack-ng | 3.9.1 | https://github.com/opencollab/arpack-ng (tag 3.9.1) | `f6641deb07fa69165b7815de9008af3ea47eb39b2bb97521fbf74c97aba6e844` | BSD-3-Clause (see `arpack-ng-3.9.1/LICENSE`) |

LAPACK 3.8.0 is chosen deliberately: it is pure fixed-form Fortran (no `.f90` module
ordering), so it compiles with a flat `gfortran` loop and no CMake — keeping the vendored
build trivially portable. Only the `BLAS/`, `SRC/`, `INSTALL/` subtrees are vendored
(the parts the library needs); `TESTING/`, `DOCS/`, `CMAKE/` etc. are omitted.

## Local modifications (kept minimal, documented for re-vendoring)

- `SPOOLES.2.2/Make.inc` — replaced the ancient Solaris `Make.inc` with a modern
  Linux/macOS one (`CC=cc`, `-O2 -fcommon -DARCH=Linux`, dropped the strict
  `-D_POSIX_C_SOURCE` that hid `struct timezone` in `timings.h`). This is the only edit
  to upstream SPOOLES sources.
- `ccx_2.22/src` — **unmodified** upstream sources. The build (`build.sh`) enables
  `-DSPOOLES -DARPACK -DMATRIXSTORAGE -DNETWORKOUT` (the standard Linux feature set),
  excludes the two include-only fragments `gauss.f` / `xlocal.f` (not standalone
  compilation units, absent from `Makefile.inc`), and compiles with
  `-fcommon -Wno-implicit-int -Wno-implicit-function-declaration` for modern GCC.
- LAPACK / arpack-ng — **unmodified** upstream sources; built with
  `-fallow-argument-mismatch` for modern gfortran.

## Re-vendoring

To update a component: download the upstream archive, verify its SHA-256 against the
table above (or record the new one), extract, copy the listed subtrees here, re-apply the
SPOOLES `Make.inc` if SPOOLES changed, and re-run `build.sh`. Then validate with the
add-in's `requireSolver`-gated end-to-end test (a uniaxial cube whose displacement must
match `u = FL/AE` exactly).
