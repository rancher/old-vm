NAME = rancher/vm-fedora
VERSION = Atomic-27-20180326.1.x86_64
BASE_IMAGE=Fedora-$(VERSION).qcow2

.PHONY : all

all: build push

build:
	[ -f $(BASE_IMAGE) ] || wget https://s3-us-west-1.amazonaws.com/ranchervm/$(BASE_IMAGE)
	docker build --build-arg BASE_IMAGE=$(BASE_IMAGE) -t $(NAME):$(VERSION) .

clean:
	rm -f $(BASE_IMAGE)

push:
	docker push $(NAME):$(VERSION)
