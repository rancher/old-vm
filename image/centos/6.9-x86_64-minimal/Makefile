NAME = rancher/vm-centos
VERSION = 6.9-x86_64-minimal
BASE_IMAGE=CentOS-$(VERSION).qcow2

.PHONY : all

all: build push

build:
	[ -f $(BASE_IMAGE) ] || wget https://s3-us-west-1.amazonaws.com/ranchervm/$(BASE_IMAGE)
	docker build --build-arg BASE_IMAGE=$(BASE_IMAGE) -t $(NAME):$(VERSION) .

clean:
	rm -f $(BASE_IMAGE)

push:
	docker push $(NAME):$(VERSION)
