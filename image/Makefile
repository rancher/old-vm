SUBDIRS := base rancheros ubuntu centos android-x86

.PHONY : all $(SUBDIRS)

all : $(SUBDIRS)

$(SUBDIRS) :
	$(MAKE) -C $@ all
