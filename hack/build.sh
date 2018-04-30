#!/bin/bash -e

IMAGE=${IMAGE:-no}
REPO=${REPO:-rancher}
NAME=vm
TAG=dev

IMAGE_NAME=${REPO}/${NAME}:${TAG}

if [ "$IMAGE" == "no" ]; then
  go build -o bin/ranchervm cmd/main.go
else
  GOOS=linux GOARCH=amd64 go build -o bin/image/ranchervm cmd/main.go
  cp -f hack/Dockerfile bin/image/

  docker build -t ${IMAGE_NAME} bin/image
  echo
  read -p "Push ${IMAGE_NAME} (y/n)? " choice
  case "$choice" in 
    y|Y ) docker push ${IMAGE_NAME} ;;
    * ) ;;
  esac
  rm -r bin/image
fi

