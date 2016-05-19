NAME = rancher/vm-android
VERSION = 4.4
BASE_IMAGE=android-4.4-r2.gz.qcow2

all: build tag_latest

build: $(BASE_IMAGE)
	cp ../base/* .
	cat Dockerfile.extra >> Dockerfile
	docker build --build-arg BASE_IMAGE=$(BASE_IMAGE) -t $(NAME):$(VERSION) .

$(BASE_IMAGE):
	wget https://s3-us-west-1.amazonaws.com/sheng/qcow2/$(BASE_IMAGE)

clean:
	rm -f $(BASE_IMAGE)

tag_latest:
	docker tag -f $(NAME):$(VERSION) $(NAME):latest

test:
	docker run -e "RANCHER_VM=true" --cap-add NET_ADMIN -it -v /var/lib/rancher/vm:/vm --device /dev/kvm:/dev/kvm --device /dev/net/tun:/dev/net/tun \
	$(NAME):$(VERSION)
