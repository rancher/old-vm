NAME = rancher/vm-novnc
VERSION := $(shell cat ../../VERSION)

.PHONY : all build push

all: build

build:
	docker build -t $(NAME):$(VERSION) .

push:
	docker push $(NAME):$(VERSION)

