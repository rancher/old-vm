NAME = rancher/vm-base
VERSION = 0.0.3

.PHONY : all clean build

all: build tag_latest

build:
	docker build -t $(NAME):$(VERSION) .

tag_latest:
	docker tag -f $(NAME):$(VERSION) $(NAME):latest

clean:

test:
	docker run -e "RANCHER_VM=true" --cap-add NET_ADMIN -it -v /var/lib/rancher/vm:/vm --device /dev/kvm:/dev/kvm --device /dev/net/tun:/dev/net/tun \
	$(NAME):$(VERSION)
