#!/bin/bash -e

REPO=${REPO:-rancher}
NAME=k8s-codegen
VERSION=1.11

DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
TAG=${REPO}/${NAME}:${VERSION}

docker build -t ${TAG} ${DIR}

