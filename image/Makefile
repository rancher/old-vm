SUBDIRS := base rancheros ubuntu centos

.PHONY : all $(SUBDIRS)

all : $(SUBDIRS)

$(SUBDIRS) :
	$(MAKE) -C $@ all
