#!/bin/bash -e

IMAGE=${IMAGE:-no}
REPO=${REPO:-rancher}
NAME=vm

version() {
  if [ -n "$(git status --porcelain --untracked-files=no)" ]; then
    DIRTY="-dirty"
  fi

  COMMIT=$(git rev-parse --short HEAD)
  GIT_TAG=$(git tag -l --contains HEAD | head -n 1)

  if [[ -z "$DIRTY" && -n "$GIT_TAG" ]]; then
      VER=$GIT_TAG
  else
      VER="${COMMIT}${DIRTY}"
  fi

  echo ${VER}
}
TAG=`version`
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

