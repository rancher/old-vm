# Ignore images without cloud-init working
SUBDIRS := vm-tools novnc centos fedora ubuntu
#SUBDIRS := base rancheros ubuntu centos android-x86 fedora

.PHONY : all $(SUBDIRS)

all : $(SUBDIRS)

$(SUBDIRS) :
	$(MAKE) -C $@ all
