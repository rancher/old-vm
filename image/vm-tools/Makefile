NAME = rancher/vm-tools
VERSION := $(shell cat ../../VERSION)
BASE_IMAGE=RancherVM-debootstrap-ubuntu-1804.tgz

.PHONY : all build clean push

all: build

build:
	[ -f $(BASE_IMAGE) ] || wget https://s3-us-west-1.amazonaws.com/ranchervm/$(BASE_IMAGE)
	docker build -t $(NAME):$(VERSION) .

clean:
	rm -f $(BASE_IMAGE)

push:
	docker push $(NAME):$(VERSION)

