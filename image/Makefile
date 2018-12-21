# SUBDIRS := $(wildcard */.)
SUBDIRS := vm-tools novnc

TARGETS := all build push

$(TARGETS): $(SUBDIRS)

$(SUBDIRS):
	$(MAKE) -C $@ $(MAKECMDGOALS)

.PHONY: $(TARGETS) $(SUBDIRS)
