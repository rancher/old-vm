#!/bin/bash -xe

TOOLS_VERSION=v1.0

docker build -t leodotcloud/vm-tools:${TOOLS_VERSION} .

echo "docker push leodotcloud/vm-tools:${TOOLS_VERSION}"
