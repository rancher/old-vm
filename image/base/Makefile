NAME = rancher/vmbase
VERSION = 0.0.1

.PHONY : all clean build

all: build

build:
	docker build -t $(NAME):$(VERSION) .

tag_latest:
	docker tag -f $(NAME):$(VERSION) $(NAME):latest

clean:

test:
	docker run --cap-add NET_ADMIN -it --volumes-from $(mgmt) \
	--device /dev/kvm:/dev/kvm --device /dev/net/tun:/dev/net/tun \
	-v /image $(NAME):$(VERSION) bash
