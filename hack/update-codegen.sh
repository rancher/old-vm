#!/bin/bash

# Update this to ensure the generated client code reflects the correct repo path
REPO=${REPO:-"github.com/rancher/vm"}

CODEGEN_IMAGE=${CODEGEN_IMAGE:-rancher/k8s-codegen:1.8}

DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"

docker run --rm -it \
  -v ${DIR}/..:/root/go/src/${REPO} \
  -w /root/go/src/k8s.io/code-generator \
  ${CODEGEN_IMAGE} \
    ./generate-groups.sh \
      "deepcopy,client,informer,lister" "${REPO}/pkg/client" "${REPO}/pkg/apis" \
      ranchervm:v1alpha1 \
      --go-header-file "/root/go/src/${REPO}/hack/boilerplate.txt"
