NAME = leodotcloud/vm-ubuntu
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

test:
	docker run -e "RANCHER_VM=true" --cap-add NET_ADMIN -it -v /var/lib/rancher/vm:/vm --device /dev/kvm:/dev/kvm --device /dev/net/tun:/dev/net/tun \
	$(NAME):$(VERSION)
