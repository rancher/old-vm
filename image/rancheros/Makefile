NAME = rancher/rancheros
VERSION = 0.2.1

.PHONY : all clean build

all: build

build: rancheros-0.2.1-gz.img
	docker build -t $(NAME):$(VERSION) .

rancheros-0.2.1-gz.img:
	wget https://s3-us-west-1.amazonaws.com/sheng/qcow2/rancheros-0.2.1-gz.img

clean:
	rm -f rancheros-0.2.1-gz.img

tag_latest:
	docker tag -f $(NAME):$(VERSION) $(NAME):latest

test:
	docker run --cap-add NET_ADMIN -it --volumes-from $(mgmt) \
	--device /dev/kvm:/dev/kvm --device /dev/net/tun:/dev/net/tun \
	-v /image $(NAME):$(VERSION) -m 512m -smp 1
