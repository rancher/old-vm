NAME = rancher/vm-windows7
VERSION = 0.0.3
BASE_IMAGE = NEED_WINDOWS_IMAGE_FILE

all: build tag_latest

build:
	cp ../base/* .
	cat Dockerfile.extra >> Dockerfile
	docker build --build-arg BASE_IMAGE=$(BASE_IMAGE) -t $(NAME):$(VERSION) .

tag_latest:
	docker tag -f $(NAME):$(VERSION) $(NAME):latest

test:
	docker run -e "RANCHER_VM=true" --cap-add NET_ADMIN -it -v /var/lib/rancher/vm:/vm --device /dev/kvm:/dev/kvm --device /dev/net/tun:/dev/net/tun \
	$(NAME):$(VERSION)
