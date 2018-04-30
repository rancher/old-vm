SUBDIRS := 27-1.6.x86_64 Atomic-27-20180326.1.x86_64

.PHONY : all $(SUBDIRS)

all : $(SUBDIRS)

$(SUBDIRS) :
	$(MAKE) -C $@ all
