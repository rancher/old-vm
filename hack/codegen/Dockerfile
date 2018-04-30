FROM ubuntu:16.04

ENV HOME=/root
ENV GOPATH=/root/go
ENV PATH=/root/go/bin:/usr/local/go/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin
ENV DEBIAN_FRONTEND=noninteractive

RUN \
  apt-get update && \
  apt-get -y install \
    apt-transport-https \
    build-essential \
    ca-certificates \
    curl \
    git \
    jq \
    lsof \
    psmisc \
    strace \
    wget \
    mercurial \
    software-properties-common \
  && \
  apt-get clean

ENV GO_VERSION 1.8.3
ENV K8S_VERSION release-1.8

RUN \
  mkdir -p ${GOPATH}/bin ${GOPATH}/src ${GOPATH}/pkg && \
  wget -qO- https://storage.googleapis.com/golang/go${GO_VERSION}.linux-amd64.tar.gz | tar -C /usr/local -xzf -

RUN set -x && \
  go get github.com/tools/godep && \
  mkdir -p ${GOPATH}/src/k8s.io/ && \
  git clone https://github.com/kubernetes/code-generator ${GOPATH}/src/k8s.io/code-generator && \
  cd ${GOPATH}/src/k8s.io/code-generator && \
  git checkout ${K8S_VERSION} && \
  godep restore && \
  go install k8s.io/code-generator/cmd/client-gen && \
  go install k8s.io/code-generator/cmd/conversion-gen && \
  go install k8s.io/code-generator/cmd/deepcopy-gen && \
  go install k8s.io/code-generator/cmd/defaulter-gen && \
  go install k8s.io/code-generator/cmd/go-to-protobuf && \
  go install k8s.io/code-generator/cmd/import-boss && \
  go install k8s.io/code-generator/cmd/informer-gen && \
  go install k8s.io/code-generator/cmd/lister-gen && \
  go install k8s.io/code-generator/cmd/openapi-gen && \
  go install k8s.io/code-generator/cmd/set-gen && \
  go get golang.org/x/tools/cmd/goimports && \
  go get k8s.io/apimachinery/pkg/runtime
