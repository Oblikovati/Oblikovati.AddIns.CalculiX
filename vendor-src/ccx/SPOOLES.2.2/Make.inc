CC = cc
OPTLEVEL = -O2 -fcommon
CFLAGS = $(OPTLEVEL) -DARCH="Linux" -Wno-implicit-int -Wno-implicit-function-declaration
LDFLAGS =
THREAD_LIBS = -lpthread
AR = ar
ARFLAGS = rv
RANLIB = ranlib
