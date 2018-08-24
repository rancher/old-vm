NAME = rancher/vm-tools
VERSION = v0.1.0
BASE_IMAGE=RancherVM-debootstrap-ubuntu-1604.tgz

.PHONY : all clean build

all: build tag_latest

build:
	[ -f $(BASE_IMAGE) ] || wget https://s3-us-west-1.amazonaws.com/ranchervm/$(BASE_IMAGE)
	docker build -t $(NAME):$(VERSION) .

clean:
	rm -f $(BASE_IMAGE)

tag_latest:
	docker tag $(NAME):$(VERSION) $(NAME):latest

push:
	docker push $(NAME):$(VERSION)
	docker push $(NAME):latest
