SUBDIRS := 6.9-x86_64-minimal 7-x86_64-minimal-1708

.PHONY : all $(SUBDIRS)

all : $(SUBDIRS)

$(SUBDIRS) :
	$(MAKE) -C $@ all
