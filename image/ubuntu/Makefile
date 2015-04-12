NAME = rancher/ubuntu
VERSION = 14.04

.PHONY : all clean build

all: build

build: ubuntu-14.04-amd64.img
	docker build -t $(NAME):$(VERSION) .

ubuntu-14.04-amd64.img:
	wget https://s3-us-west-1.amazonaws.com/sheng/qcow2/ubuntu-14.04-amd64.img

clean:
	rm -f ubuntu-14.04-amd64.img

tag_latest:
	docker tag -f $(NAME):$(VERSION) $(NAME):latest

test:
	docker run --cap-add NET_ADMIN -it --volumes-from $(mgmt) \
	--device /dev/kvm:/dev/kvm --device /dev/net/tun:/dev/net/tun \
	-v /image $(NAME):$(VERSION) -m 512m -smp 1
