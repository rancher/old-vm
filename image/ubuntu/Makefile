NAME = llparse/vm-ubuntu
VERSION = 14.04
BASE_IMAGE=ubuntu-14.04-amd64.img

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
