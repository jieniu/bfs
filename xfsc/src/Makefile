include env.mk

MAKEFLAGS    += -s

# project root path
PROOT = $(shell pwd)

# path setting
THIRD_PATH    = $(PROOT)/3rd
DOWNLOAD_PATH = $(PROOT)/dl
DEPEND_PATH   = $(PROOT)/depends
RELEASE_PATH  = $(PROOT)/_tmp


# compiler setting
CC        = gcc
CXX       = g++
AR        = ar
STRIP     = strip

CFLAGS   += -g -Wall -fPIC -std=gnu99 -I$(PROOT) -I$(DEPEND_PATH)/include
CXXFLAGS += -g -Wall -fPIC -std=gnu++98 -I$(PROOT) -I$(DEPEND_PATH)/include
CXXFLAGS += -I/usr/include/jsoncpp

LDFLAGS  += -L$(DEPEND_PATH)/lib -L$(PROOT)
LIBS     += -lxfsc -lcurl -ljsoncpp -pthread


LIB_STATIC  = $(DEPEND_PATH)/lib/libxfsc.a
LIB_SHARED  = $(DEPEND_PATH)/lib/libxfsc.so
OBJS        = xfsc.o queue.o http_parser.o sha1.o ylog/ylog.o
OBJS_TEST   = xfsc_test.o
EXE         = xfsc_test


all: prepare $(EXE)

$(EXE): $(LIB_STATIC) $(LIB_SHARED) $(OBJS_TEST)
	@echo -e "  CXXLD  $(EXE)  <<== [ $(OBJS_TEST) $(LIB_STATIC) ]" \
	        && $(CXX) -o $(EXE) $(OBJS_TEST) $(LDFLAGS) $(LIBS)
	@echo -e "\033[34m  Link Success.\033[00m"

$(LIB_STATIC): $(OBJS)
	@echo -e "  AR     $(LIB_STATIC)  <<== [ $(OBJS) ]"\
	        &&  $(AR) rcs $(LIB_STATIC) $(OBJS)

$(LIB_SHARED): $(OBJS)
	@echo -e "  CXX    $(LIB_SHARED)  <<== [ $(OBJS) ]"\
	        &&  $(CXX) -shared -fPIC -o $(LIB_SHARED) $(OBJS) -lcurl -ljsoncpp

prepare:
	@$(MKDIR) $(DEPEND_PATH)/lib

%.o: %.cpp
	@echo -e "  CXX $<" \
                &&  $(CXX) -c $(CXXFLAGS) $< -o $@


third:
	$(MAKE) -C $(THIRD_PATH)

release:
	@$(RM) _tmp
	@$(MKDIR) _tmp
	@$(CP) $(DEPEND_PATH)/lib $(RELEASE_PATH)/
	@$(CP) xfsc.h $(RELEASE_PATH)/
	@echo -e "\033[34m  Release Success.\033[00m"

clean:
#	$(MAKE) clean -C $(THIRD_PATH)
	@$(RM) $(RELEASE_PATH)
	@$(RM) $(EXE) $(LIB_STATIC) $(LIB_SHARED) $(OBJS) $(OBJS_TEST)
	@echo -e "\033[34m  Clean Success.\033[00m"

distclean: clean
	@$(RM) $(DEPEND_PATH)
	@echo -e "\033[34m  DistClean Success.\033[00m"
