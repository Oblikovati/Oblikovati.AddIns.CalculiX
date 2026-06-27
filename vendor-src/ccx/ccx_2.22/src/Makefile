
CFLAGS = -Wall -O2 -fcommon -Wno-implicit-int -Wno-implicit-function-declaration  -I ../../../SPOOLES.2.2 -DARCH="Linux" -DSPOOLES -DMATRIXSTORAGE -DNETWORKOUT
FFLAGS = -Wall -O2

CC=cc
FC=gfortran

.c.o :
	$(CC) $(CFLAGS) -c $<
.f.o :
	$(FC) $(FFLAGS) -c $<

include Makefile.inc

SCCXMAIN = ccx_2.22.c

OCCXF = $(SCCXF:.f=.o)
OCCXC = $(SCCXC:.c=.o)
OCCXMAIN = $(SCCXMAIN:.c=.o)

DIR=../../../SPOOLES.2.2

LIBS = \
       $(DIR)/spooles.a \
 \
       -lpthread -lm -lc

ccx_2.22: $(OCCXMAIN) ccx_2.22.a  $(LIBS)
	./date.pl; $(CC) $(CFLAGS) -c ccx_2.22.c; $(FC)  -Wall -O2 -o $@ $(OCCXMAIN) ccx_2.22.a $(LIBS) -fopenmp

ccx_2.22.a: $(OCCXF) $(OCCXC)
	ar vr $@ $?
