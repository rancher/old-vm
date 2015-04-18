SUBDIRS := image mgmt

.PHONY : all $(SUBDIRS)

all : $(SUBDIRS)

$(SUBDIRS) :
	$(MAKE) -C $@ all
