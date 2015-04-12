NAME = rancher/centos
VERSION = 7.1

.PHONY : all clean build

all: build

build: centos.7-1.x86-64.20150401.qcow2
	docker build -t $(NAME):$(VERSION) .

centos.7-1.x86-64.20150401.qcow2:
	wget https://s3-us-west-1.amazonaws.com/sheng/qcow2/centos.7-1.x86-64.20150401.qcow2

clean:
	rm -f centos.7-1.x86-64.20150401.qcow2

tag_latest:
	docker tag -f $(NAME):$(VERSION) $(NAME):latest

test:
	docker run --cap-add NET_ADMIN -it --volumes-from $(mgmt) \
	--device /dev/kvm:/dev/kvm --device /dev/net/tun:/dev/net/tun \
	-v /image $(NAME):$(VERSION) -m 512m -smp 1
