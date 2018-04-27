NAME = rancher/vm-windows7
VERSION = 0.0.3
BASE_IMAGE=windows7-starter.qcow2

.PHONY : all clean build

all: build tag_latest

build: $(BASE_IMAGE)
	docker build -t $(NAME):$(VERSION) .

$(BASE_IMAGE):
	wget https://s3-us-west-1.amazonaws.com/sheng/qcow2/$(BASE_IMAGE)

clean:
	rm -f $(BASE_IMAGE)

tag_latest:
	docker tag $(NAME):$(VERSION) $(NAME):latest

push:
	docker push $(NAME):$(VERSION)
	docker push $(NAME):latest
