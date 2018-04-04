SUBDIRS := 16.04.4-desktop-amd64 16.04.4-server-amd64

.PHONY : all $(SUBDIRS)

all : $(SUBDIRS)

$(SUBDIRS) :
	$(MAKE) -C $@ all
