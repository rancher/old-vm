NAME = llparse/novnc
VERSION = 0.0.1

.PHONY : all clean build

all: build tag_latest

build:
	docker build -t $(NAME):$(VERSION) .

clean:
	rm -f $(BASE_IMAGE)

tag_latest:
	docker tag $(NAME):$(VERSION) $(NAME):latest

push:
	docker push $(NAME):$(VERSION)
	docker push $(NAME):latest
