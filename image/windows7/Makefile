NAME = rancher/windows7
VERSION = 0.0.1

all: build

build:
	docker build -t $(NAME):$(VERSION) .

tag_latest:
	docker tag -f $(NAME):$(VERSION) $(NAME):latest

test:
	docker run --cap-add NET_ADMIN -it --volumes-from $(mgmt) \
	--device /dev/kvm:/dev/kvm --device /dev/net/tun:/dev/net/tun \
	-v /image $(NAME):$(VERSION) -m 512m -smp 1
