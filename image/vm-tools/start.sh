#!/bin/bash -ex

if [ ! -d /vm-tools ]; then
  echo "error: /vm-tools not mounted, exiting"
  exit 1
fi

if [ ! -f /vm-tools/.success ]; then
  echo "Extracting filesystem to volume mounted at /vm-tools"
  tar xzf /opt/rancher/vm-tools/RancherVM-debootstrap-ubuntu-1804.tgz -C /vm-tools
  cp /opt/rancher/vm-tools/startvm /vm-tools/usr/bin/startvm

  echo "Extraction successful"
  touch /vm-tools/.success
fi
