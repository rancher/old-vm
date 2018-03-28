NAME = llparse/vm-centos
VERSION = 7.1
BASE_IMAGE = centos.7-1.x86-64.20150401.qcow2

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
