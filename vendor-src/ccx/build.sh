#!/usr/bin/env bash
# Build the vendored CalculiX solver (ccx) and its numeric dependencies entirely from
# the in-repo sources — no network, no system BLAS/LAPACK/ARPACK. Produces a single
# self-contained `ccx` executable in ./build that the add-in runs as a subprocess
# (OBK_CCX_BIN / vendor-src/ccx/build).
#
# Stack (all vendored under this directory, see NOTICE.md for provenance + licenses):
#   SPOOLES 2.2      sparse direct linear solver   -> spooles.a   (its own recursive make)
#   LAPACK 3.8.0     reference BLAS + LAPACK        -> liblapack.a (fixed-form Fortran)
#   arpack-ng 3.9.1  ARPACK eigensolver            -> libarpack.a
#   CalculiX 2.22    the FE solver                  -> ccx
#
# Requires a C compiler + gfortran (build-time only; the shipped binary links none of
# them dynamically beyond libc/libgfortran/libgomp). Tested with gcc/gfortran 13.
set -euo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
JOBS="${JOBS:-$( (nproc 2>/dev/null || sysctl -n hw.ncpu 2>/dev/null) || echo 4)}"
CC="${CC:-cc}"
FC="${FC:-gfortran}"
OUT="$HERE/build"
mkdir -p "$OUT"

echo "### [1/4] SPOOLES 2.2 -> spooles.a"
make -C "$HERE/SPOOLES.2.2" lib >/dev/null

echo "### [2/4] LAPACK 3.8.0 (reference BLAS+LAPACK) -> liblapack.a"
( cd "$HERE/lapack-3.8.0" && rm -f ./*.o
  $FC -O2 -fcommon -fallow-argument-mismatch -c \
     BLAS/SRC/*.f SRC/*.f \
     INSTALL/dlamch.f INSTALL/slamch.f \
     INSTALL/second_INT_ETIME.f INSTALL/dsecnd_INT_ETIME.f
  ar rcs "$OUT/liblapack.a" ./*.o && rm -f ./*.o )

echo "### [3/4] arpack-ng 3.9.1 -> libarpack.a"
( cd "$HERE/arpack-ng-3.9.1" && rm -f ./*.o
  $FC -O2 -fcommon -fallow-argument-mismatch -c SRC/*.f UTIL/*.f
  ar rcs "$OUT/libarpack.a" ./*.o && rm -f ./*.o )

echo "### [4/4] CalculiX 2.22 -> ccx"
SRC="$HERE/ccx_2.22/src"
CDEF="-DARCH=Linux -DSPOOLES -DARPACK -DMATRIXSTORAGE -DNETWORKOUT"
CFLAGS="-O2 -fcommon -Wno-implicit-int -Wno-implicit-function-declaration -I $HERE/SPOOLES.2.2 $CDEF"
( cd "$SRC" && rm -f ./*.o ./*.a
  # gauss.f / xlocal.f are include fragments, not standalone sources (not in Makefile.inc).
  ls ./*.f | grep -vxF -e ./gauss.f -e ./xlocal.f | xargs -P"$JOBS" -I{} "$FC" -O2 -fcommon -c {}
  # ccx_2.22.c is the entry point, linked separately from the object archive.
  ls ./*.c | grep -vxF ./ccx_2.22.c | xargs -P"$JOBS" -I{} "$CC" $CFLAGS -c {}
  ar rcs ccx.a ./*.o
  "$CC" $CFLAGS -c ccx_2.22.c -o ccx_main.o
  "$FC" -O2 -o "$OUT/ccx" ccx_main.o ccx.a \
     "$HERE/SPOOLES.2.2/spooles.a" "$OUT/libarpack.a" "$OUT/liblapack.a" \
     -lpthread -lm -fopenmp
  rm -f ./*.o ./*.a )

strip -s "$OUT/ccx" 2>/dev/null || true
echo "### built $OUT/ccx"
