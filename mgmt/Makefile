NAME = rancher/vmmgmt
VERSION = 0.0.1
NOVNC_VERSION = 0.0.2

.PHONY : all clean build

all: build

build: v$(NOVNC_VERSION).tar.gz
	rm -rf noVNC-$(NOVNC_VERSION)
	tar xvf v$(NOVNC_VERSION).tar.gz
	mkdir noVNC-$(NOVNC_VERSION)/cgi-bin
	ln -s /var/lib/rancher/ranchervm noVNC-$(NOVNC_VERSION)/cgi-bin
	cp index.html noVNC-$(NOVNC_VERSION)/
	docker build -t $(NAME):$(VERSION) .

v$(NOVNC_VERSION).tar.gz:
	wget https://github.com/rancherio/noVNC/archive/v$(NOVNC_VERSION).tar.gz

clean:
	rm -rf v$(NOVNC_VERSION).tar.gz noVNC-$(NOVNC_VERSION)

tag_latest:
	docker tag -f $(NAME):$(VERSION) $(NAME):latest

run:
	docker run --cap-add NET_ADMIN -it -v /vmmgmt \
	-v /var/run/docker.sock:/var/run/docker.sock \
	-p 8080:80 \
	$(NAME):$(VERSION)
